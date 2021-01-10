package gfsmux // import "go.gridfinity.dev/gfsmux"

import (
	"encoding/binary"
	"fmt"
)

const ( // cmds
	// protocol version 1:
	CmdSyn byte = iota // stream open
	CmdFin             // stream close, a.k.a EOF mark
	CmdPsh             // data push
	CmdNop             // no operation

	// protocol version 2 extra commands
	// notify bytes consumed by remote peer-end
	cmdUPD
)

const (
	// data size of cmdUPD, format:
	// |4B data consumed(ACK)| 4B window size(WINDOW) |
	szCmdUPD = 8
)

const (
	// initial peer window guess, a slow-start
	initialPeerWindow = 262144
)

const (
	sizeOfVer    = 1
	sizeOfCmd    = 1
	sizeOfLength = 2
	sizeOfSid    = 4
	HeaderSize   = sizeOfVer + sizeOfCmd + sizeOfSid + sizeOfLength
)

// Frame defines a packet from or to be multiplexed into a single Connection
type Frame struct {
	Ver  byte
	Cmd  byte
	Sid  uint32
	Data []byte
}

func NewFrame(Version, Cmd byte, Sid uint32) Frame {
	return Frame{Ver: Version, Cmd: Cmd, Sid: Sid}
}

type rawHeader [HeaderSize]byte

func (h rawHeader) Version() byte {
	return h[0]
}

func (h rawHeader) Cmd() byte {
	return h[1]
}

func (h rawHeader) Length() uint16 {
	return binary.LittleEndian.Uint16(h[2:])
}

func (h rawHeader) StreamID() uint32 {
	return binary.LittleEndian.Uint32(h[4:])
}

func (h rawHeader) String() string {
	return fmt.Sprintf("Version:%d Cmd:%d StreamID:%d Length:%d",
		h.Version(), h.Cmd(), h.StreamID(), h.Length())
}

type updHeader [szCmdUPD]byte

func (h updHeader) Consumed() uint32 {
	return binary.LittleEndian.Uint32(h[:])
}

func (h updHeader) Window() uint32 {
	return binary.LittleEndian.Uint32(h[4:])
}
