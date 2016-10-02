package net

import "fmt"

// An EthernetInterface is a low-level interface capable of reading
// and writing ethernet frames.
type EthernetInterface interface {
	// MAC returns the interface's MAC address, if any.
	MAC() (ok bool, mac MAC)
	// SetMAC sets the interface's MAC address. If mac == BroadcastMac,
	// SetMAC will panic.
	SetMAC(mac MAC) error

	// MTU returns the interface's MTU. If no MTU is set, MTU will return 0.
	MTU() uint64
	// SetMTU sets the interface's MTU. If mtu == 0, SetMTU will panic.
	SetMTU(mtu uint64) error

	// ReadFrame reads an ethernet frame into b.
	// n is the number of bytes written to b. If
	// the frame was larger than len(b), n == len(b),
	// and err == io.EOF.
	//
	// If the interface has its MAC set, only ethernet
	// frames whose destination MAC is equal to the
	// interface's MAC or is the broadcast MAC will
	// be returned.
	ReadFrame(b []byte) (n int, src, dst MAC, et EtherType, err error)
	// WriteFrame writes an ethernet frame with the
	// payload b. If the interface has an MTU set,
	// and len(b) is larger than that MTU, the frame
	// will not be written, and instead WriteFrame will
	// return an error such that IsMTU(err) == true.
	//
	// If the destination MAC is the broadcast MAC,
	// the frame will be broadcast to all devices
	// on the local ethernet network.
	//
	// If a MAC address has been set, that will be used
	// as the frame's source MAC. Otherwise, WriteFrame
	// will return an error.
	WriteFrame(b []byte, dst MAC, et EtherType) error
	// WriteFrameSrc is like WriteFrame, but allows the
	// source MAC address to be set explicitly.
	WriteFrameSrc(b []byte, src, dst MAC, et EtherType) error
}

// EtherType is a value of 1536 or greater which indicates
// the protocol type of a packet encapsulated in an ethernet frame.
//
// EtherType implements the Protocol interface.
type EtherType uint16

func (e EtherType) String() string {
	return fmt.Sprintf("0x%x", uint16(e))
}

const (
	EtherTypeIPv4 EtherType = 0x0800
	EtherTypeARP  EtherType = 0x0806
	EtherTypeIPv6 EtherType = 0x86DD
)

// MAC is an ethernet media access control address.
type MAC [6]byte

// BroadcastMAC is the broadcast MAC address.
var BroadcastMAC = MAC{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}
