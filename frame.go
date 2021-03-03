package gfsmux

import (
	"encoding/binary"
	"fmt"
)

const (
	// Protocol version 1: old commands

	// CmdSyn ... stream open
	CmdSyn byte = iota
	// CmdFin ... stream close, EOF
	CmdFin
	// CmdPsh ... push data
	CmdPsh
	// CmdNop ... noop
	CmdNop

	// Protocol version 2: new commands

	// CmdUpd ... notify bytes consumed by remote peer-end
	CmdUpd
)

const (
	// data size of CmdUpd, format:
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
	// HeaderSize ...
	HeaderSize = sizeOfVer + sizeOfCmd + sizeOfSid + sizeOfLength
)

// Frame defines a packet from or to be multiplexed into a single Connection
type Frame struct {
	Ver  byte
	Cmd  byte
	Sid  uint32
	Data []byte
}

// NewFrame ...
func NewFrame(
	Version,
	Cmd byte,
	Sid uint32,
) Frame {
	return Frame{Ver: Version, Cmd: Cmd, Sid: Sid}
}

type rawHeader [HeaderSize]byte

func (
	h rawHeader,
) Version() byte {
	return h[0]
}

func (
	h rawHeader,
) Cmd() byte {
	return h[1]
}

func (
	h rawHeader,
) Length() uint16 {
	return binary.LittleEndian.Uint16(
		h[2:],
	)
}

func (
	h rawHeader,
) StreamID() uint32 {
	return binary.LittleEndian.Uint32(
		h[4:],
	)
}

func (
	h rawHeader,
) String() string {
	return fmt.Sprintf(
		"Version:%d Cmd:%d StreamID:%d Length:%d",
		h.Version(),
		h.Cmd(),
		h.StreamID(),
		h.Length(),
	)
}

type updHeader [szCmdUPD]byte

func (
	h updHeader,
) Consumed() uint32 {
	return binary.LittleEndian.Uint32(
		h[:],
	)
}

func (
	h updHeader,
) Window() uint32 {
	return binary.LittleEndian.Uint32(
		h[4:],
	)
}
