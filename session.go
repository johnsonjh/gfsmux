package gfsmux

import (
	"container/heap"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

const (
	defaultAcceptBacklog = 1024
)

var (
	ErrInvalidProtocol = errors.New("invalid protocol")
	ErrConsumed        = errors.New("peer consumed more than sent")
	ErrGoAway          = errors.New("stream id overflows, should start a new Connection")
	ErrTimeout         = &timeoutError{}
	ErrWouldBlock      = errors.New("operation would block on IO")
)

var _ net.Error = &timeoutError{}

type timeoutError struct{}

func (e *timeoutError) Error() string   { return "timeout" }
func (e *timeoutError) Timeout() bool   { return true }
func (e *timeoutError) Temporary() bool { return true }

type WriteRequest struct {
	Prio   uint64
	frame  Frame
	result chan writeResult
}

type writeResult struct {
	n   int
	err error
}

type buffersWriter interface {
	WriteBuffers(v [][]byte) (n int, err error)
}

// Session defines a multiplexed Connection for streams
type Session struct {
	Conn io.ReadWriteCloser

	Config           *Config
	nextStreamID     uint32 // next stream identifier
	nextStreamIDLock sync.Mutex

	bucket       int32         // token bucket
	bucketNotify chan struct{} // used for waiting for tokens

	streams    map[uint32]*Stream // all streams in this session
	streamLock sync.Mutex         // locks streams

	die     chan struct{} // flag session has died
	dieOnce sync.Once

	// socket error handling
	socketReadError      atomic.Value
	socketWriteError     atomic.Value
	chSocketReadError    chan struct{}
	chSocketWriteError   chan struct{}
	socketReadErrorOnce  sync.Once
	socketWriteErrorOnce sync.Once

	// smux protocol errors
	protoError     atomic.Value
	chProtoError   chan struct{}
	protoErrorOnce sync.Once

	chAccepts chan *Stream

	dataReady int32 // flag data has arrived

	goAway int32 // flag id exhausted

	deadline atomic.Value

	shaper chan WriteRequest // a shaper for writing
	writes chan WriteRequest
}

func newSession(Config *Config, Conn io.ReadWriteCloser, client bool) *Session {
	s := new(Session)
	s.die = make(chan struct{})
	s.Conn = Conn
	s.Config = Config
	s.streams = make(map[uint32]*Stream)
	s.chAccepts = make(chan *Stream, defaultAcceptBacklog)
	s.bucket = int32(Config.MaxReceiveBuffer)
	s.bucketNotify = make(chan struct{}, 1)
	s.shaper = make(chan WriteRequest)
	s.writes = make(chan WriteRequest)
	s.chSocketReadError = make(chan struct{})
	s.chSocketWriteError = make(chan struct{})
	s.chProtoError = make(chan struct{})

	if client {
		s.nextStreamID = 1
	} else {
		s.nextStreamID = 0
	}

	go s.shaperLoop()
	go s.recvLoop()
	go s.sendLoop()
	if !Config.KeepAliveDisabled {
		go s.keepalive()
	}
	return s
}

// OpenStream is used to create a new stream
func (s *Session) OpenStream() (*Stream, error) {
	if s.IsClosed() {
		return nil, io.ErrClosedPipe
	}

	// generate stream id
	s.nextStreamIDLock.Lock()
	if s.goAway > 0 {
		s.nextStreamIDLock.Unlock()
		return nil, ErrGoAway
	}

	s.nextStreamID += 2
	Sid := s.nextStreamID
	if Sid == Sid%2 { // stream-id overflows
		s.goAway = 1
		s.nextStreamIDLock.Unlock()
		return nil, ErrGoAway
	}
	s.nextStreamIDLock.Unlock()

	stream := newStream(Sid, s.Config.MaxFrameSize, s)

	if _, err := s.WriteFrame(NewFrame(byte(s.Config.Version), CmdSyn, Sid)); err != nil {
		return nil, err
	}

	s.streamLock.Lock()
	defer s.streamLock.Unlock()
	select {
	case <-s.chSocketReadError:
		return nil, s.socketReadError.Load().(error)
	case <-s.chSocketWriteError:
		return nil, s.socketWriteError.Load().(error)
	case <-s.die:
		return nil, io.ErrClosedPipe
	default:
		s.streams[Sid] = stream
		return stream, nil
	}
}

// Open returns a generic ReadWriteCloser
func (s *Session) Open() (io.ReadWriteCloser, error) {
	return s.OpenStream()
}

// AcceptStream is used to block until the next available stream
// is ready to be accepted.
func (s *Session) AcceptStream() (*Stream, error) {
	var deadline <-chan time.Time
	if d, ok := s.deadline.Load().(time.Time); ok && !d.IsZero() {
		timer := time.NewTimer(time.Until(d))
		defer timer.Stop()
		deadline = timer.C
	}

	select {
	case stream := <-s.chAccepts:
		return stream, nil
	case <-deadline:
		return nil, ErrTimeout
	case <-s.chSocketReadError:
		return nil, s.socketReadError.Load().(error)
	case <-s.chProtoError:
		return nil, s.protoError.Load().(error)
	case <-s.die:
		return nil, io.ErrClosedPipe
	}
}

// Accept Returns a generic ReadWriteCloser instead of smux.Stream
func (s *Session) Accept() (io.ReadWriteCloser, error) {
	return s.AcceptStream()
}

// Close is used to close the session and all streams.
func (s *Session) Close() error {
	var once bool
	s.dieOnce.Do(func() {
		close(s.die)
		once = true
	})

	if once {
		s.streamLock.Lock()
		for k := range s.streams {
			s.streams[k].sessionClose()
		}
		s.streamLock.Unlock()
		return s.Conn.Close()
	}
	return io.ErrClosedPipe
}

// notifyBucket notifies recvLoop that bucket is available
func (s *Session) notifyBucket() {
	select {
	case s.bucketNotify <- struct{}{}:
	default:
	}
}

func (s *Session) notifyReadError(err error) {
	s.socketReadErrorOnce.Do(func() {
		s.socketReadError.Store(err)
		close(s.chSocketReadError)
	})
}

func (s *Session) notifyWriteError(err error) {
	s.socketWriteErrorOnce.Do(func() {
		s.socketWriteError.Store(err)
		close(s.chSocketWriteError)
	})
}

func (s *Session) notifyProtoError(err error) {
	s.protoErrorOnce.Do(func() {
		s.protoError.Store(err)
		close(s.chProtoError)
	})
}

// IsClosed does a safe check to see if we have shutdown
func (s *Session) IsClosed() bool {
	select {
	case <-s.die:
		return true
	default:
		return false
	}
}

// NumStreams returns the number of currently open streams
func (s *Session) NumStreams() int {
	if s.IsClosed() {
		return 0
	}
	s.streamLock.Lock()
	defer s.streamLock.Unlock()
	return len(s.streams)
}

// SetDeadline sets a deadline used by Accept* calls.
// A zero time value disables the deadline.
func (s *Session) SetDeadline(t time.Time) error {
	s.deadline.Store(t)
	return nil
}

// LocalAddr satisfies net.Conn interface
func (s *Session) LocalAddr() net.Addr {
	if ts, ok := s.Conn.(interface {
		LocalAddr() net.Addr
	}); ok {
		return ts.LocalAddr()
	}
	return nil
}

// RemoteAddr satisfies net.Conn interface
func (s *Session) RemoteAddr() net.Addr {
	if ts, ok := s.Conn.(interface {
		RemoteAddr() net.Addr
	}); ok {
		return ts.RemoteAddr()
	}
	return nil
}

// notify the session that a stream has closed
func (s *Session) streamClosed(Sid uint32) {
	s.streamLock.Lock()
	if n := s.streams[Sid].recycleTokens(); n > 0 { // return remaining tokens to the bucket
		if atomic.AddInt32(&s.bucket, int32(n)) > 0 {
			s.notifyBucket()
		}
	}
	delete(s.streams, Sid)
	s.streamLock.Unlock()
}

// returnTokens is called by stream to return token after read
func (s *Session) returnTokens(n int) {
	if atomic.AddInt32(&s.bucket, int32(n)) > 0 {
		s.notifyBucket()
	}
}

// recvLoop keeps on reading from underlying Connection if tokens are available
func (s *Session) recvLoop() {
	var hdr rawHeader
	var updHdr updHeader

	for {
		for atomic.LoadInt32(&s.bucket) <= 0 && !s.IsClosed() {
			select {
			case <-s.bucketNotify:
			case <-s.die:
				return
			}
		}

		// read header first
		if _, err := io.ReadFull(s.Conn, hdr[:]); err == nil {
			atomic.StoreInt32(&s.dataReady, 1)
			if hdr.Version() != byte(s.Config.Version) {
				s.notifyProtoError(ErrInvalidProtocol)
				return
			}
			Sid := hdr.StreamID()
			switch hdr.Cmd() {
			case CmdNop:
			case CmdSyn:
				s.streamLock.Lock()
				if _, ok := s.streams[Sid]; !ok {
					stream := newStream(Sid, s.Config.MaxFrameSize, s)
					s.streams[Sid] = stream
					select {
					case s.chAccepts <- stream:
					case <-s.die:
					}
				}
				s.streamLock.Unlock()
			case CmdFin:
				s.streamLock.Lock()
				if stream, ok := s.streams[Sid]; ok {
					stream.fin()
					stream.notifyReadEvent()
				}
				s.streamLock.Unlock()
			case CmdPsh:
				if hdr.Length() > 0 {
					newbuf := defaultAllocator.Get(int(hdr.Length()))
					if written, err := io.ReadFull(s.Conn, newbuf); err == nil {
						s.streamLock.Lock()
						if stream, ok := s.streams[Sid]; ok {
							stream.pushBytes(newbuf)
							atomic.AddInt32(&s.bucket, -int32(written))
							stream.notifyReadEvent()
						}
						s.streamLock.Unlock()
					} else {
						s.notifyReadError(err)
						return
					}
				}
			case CmdUpd:
				if _, err := io.ReadFull(s.Conn, updHdr[:]); err == nil {
					s.streamLock.Lock()
					if stream, ok := s.streams[Sid]; ok {
						stream.update(updHdr.Consumed(), updHdr.Window())
					}
					s.streamLock.Unlock()
				} else {
					s.notifyReadError(err)
					return
				}
			default:
				s.notifyProtoError(ErrInvalidProtocol)
				return
			}
		} else {
			s.notifyReadError(err)
			return
		}
	}
}

func (s *Session) keepalive() {
	tickerPing := time.NewTicker(s.Config.KeepAliveInterval)
	tickerTimeout := time.NewTicker(s.Config.KeepAliveTimeout)
	defer tickerPing.Stop()
	defer tickerTimeout.Stop()
	for {
		select {
		case <-tickerPing.C:
			s.WriteFrameInternal(NewFrame(byte(s.Config.Version), CmdNop, 0), tickerPing.C, 0)
			s.notifyBucket() // force a signal to the recvLoop
		case <-tickerTimeout.C:
			if !atomic.CompareAndSwapInt32(&s.dataReady, 1, 0) {
				// recvLoop may block while bucket is 0, in this case,
				// session should not be closed.
				if atomic.LoadInt32(&s.bucket) > 0 {
					s.Close()
					return
				}
			}
		case <-s.die:
			return
		}
	}
}

// shaper shapes the sending sequence among streams
func (s *Session) shaperLoop() {
	var reqs ShaperHeap
	var next WriteRequest
	var chWrite chan WriteRequest

	for {
		if len(reqs) > 0 {
			chWrite = s.writes
			next = heap.Pop(&reqs).(WriteRequest)
		} else {
			chWrite = nil
		}

		select {
		case <-s.die:
			return
		case r := <-s.shaper:
			if chWrite != nil { // next is valid, reshape
				heap.Push(&reqs, next)
			}
			heap.Push(&reqs, r)
		case chWrite <- next:
		}
	}
}

func (s *Session) sendLoop() {
	var buf []byte
	var n int
	var err error
	var vec [][]byte // vector for writeBuffers

	bw, ok := s.Conn.(buffersWriter)
	if ok {
		buf = make([]byte, HeaderSize)
		vec = make([][]byte, 2)
	} else {
		buf = make([]byte, (1<<16)+HeaderSize)
	}

	for {
		select {
		case <-s.die:
			return
		case request := <-s.writes:
			buf[0] = request.frame.Ver
			buf[1] = request.frame.Cmd
			binary.LittleEndian.PutUint16(buf[2:], uint16(len(request.frame.Data)))
			binary.LittleEndian.PutUint32(buf[4:], request.frame.Sid)

			if len(vec) > 0 {
				vec[0] = buf[:HeaderSize]
				vec[1] = request.frame.Data
				n, err = bw.WriteBuffers(vec)
			} else {
				copy(buf[HeaderSize:], request.frame.Data)
				n, err = s.Conn.Write(buf[:HeaderSize+len(request.frame.Data)])
			}

			n -= HeaderSize
			if n < 0 {
				n = 0
			}

			result := writeResult{
				n:   n,
				err: err,
			}

			request.result <- result
			close(request.result)

			// store Conn error
			if err != nil {
				s.notifyWriteError(err)
				return
			}
		}
	}
}

// WriteFrame writes the frame to the underlying Connection
// and returns the number of bytes written if successful
func (s *Session) WriteFrame(f Frame) (n int, err error) {
	return s.WriteFrameInternal(f, nil, 0)
}

// WriteFrameInternal is to support deadline used in keepalive
func (s *Session) WriteFrameInternal(f Frame, deadline <-chan time.Time, Prio uint64) (int, error) {
	req := WriteRequest{
		Prio:   Prio,
		frame:  f,
		result: make(chan writeResult, 1),
	}
	select {
	case s.shaper <- req:
	case <-s.die:
		return 0, io.ErrClosedPipe
	case <-s.chSocketWriteError:
		return 0, s.socketWriteError.Load().(error)
	case <-deadline:
		return 0, ErrTimeout
	}

	select {
	case result := <-req.result:
		return result.n, result.err
	case <-s.die:
		return 0, io.ErrClosedPipe
	case <-s.chSocketWriteError:
		return 0, s.socketWriteError.Load().(error)
	case <-deadline:
		return 0, ErrTimeout
	}
}
