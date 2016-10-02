package net

import (
	"fmt"
	"sync"

	"github.com/juju/errors"
)

// An EthernetInterface is a low-level interface capable of reading
// and writing ethernet frames.
//
// An EthernetInterface is safe for concurrent access.
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
	WriteFrame(b []byte, dst MAC, et EtherType) (n int, err error)
	// WriteFrameSrc is like WriteFrame, but allows the
	// source MAC address to be set explicitly.
	WriteFrameSrc(b []byte, src, dst MAC, et EtherType) (n int, err error)
	Deadliner
}

// EtherType is a value of 1536 or greater which indicates
// the protocol type of a packet encapsulated in an ethernet frame.
type EtherType uint16

const (
	EtherTypeIPv4 EtherType = 0x0800
	EtherTypeARP  EtherType = 0x0806
	EtherTypeIPv6 EtherType = 0x86DD
)

// MAC is an ethernet media access control address.
type MAC [6]byte

// BroadcastMAC is the broadcast MAC address.
var BroadcastMAC = MAC{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}

// An EthernetDevice is a device which uses an EthernetInterface
// as its underlying frame transport mechanism. It implements
// the Device interface.
type EthernetDevice struct {
	iface EthernetInterface
	arp   *arp // nil if the device is down
	// frames chan ethernetFrame

	// When bringing the device up or down,
	// first acquire mu, then close done,
	// then wg.Wait(). When bringing the
	// device up, first acquire mu, then
	// re-initialize done and wg, then
	// launch the arp daemon and the frame
	// reader daemon.
	//
	// Once mu is acquired, make sure to
	// check whether done is closed to see
	// if the device is currently down.
	// done chan struct{}
	// wg   sync.WaitGroup

	// Acquire a read lock for all operations.
	// Acquire a write lock to bring the device
	// up or down.
	mu sync.RWMutex
}

var _ Device = &EthernetDevice{}

// type ethernetFrame struct {
// 	src, dst MAC
// 	et       EtherType
// 	payload  []byte
// }

// NewEthernetDevice creates a new EthernetDevice using iface for frame
// transport and addr as the interface's MAC address, and brings the
// device up.
func NewEthernetDevice(iface EthernetInterface, addr MAC) (*EthernetDevice, error) {
	err := iface.SetMAC(addr)
	if err != nil {
		return nil, errors.Annotate(err, "create new ethernet device")
	}
	arp, err := newARP()
	if err != nil {
		return nil, errors.Annotate(err, "create new ethernet device")
	}
	return &EthernetDevice{
		iface: iface,
		arp:   arp,
	}, nil
}

// BringUp brings dev up. If it is already up, BringUp is a no-op.
func (dev *EthernetDevice) BringUp() error {
	dev.mu.Lock()
	defer dev.mu.Unlock()
	if dev.arp != nil {
		// already up
		return nil
	}

	arp, err := newARP()
	if err != nil {
		return errors.Annotate(err, "bring device up")
	}
	dev.arp = arp
	return nil
}

// BringDown brings dev down. If it is already up, BringDown is a no-op.
func (dev *EthernetDevice) BringDown() error {
	dev.mu.Lock()
	defer dev.mu.Unlock()
	if dev.arp == nil {
		// already down
		return nil
	}

	dev.arp.Stop()
	return nil
}

// IsUp returns true if dev is up.
func (dev *EthernetDevice) IsUp() bool {
	dev.mu.RLock()
	up := dev.arp != nil
	dev.mu.RUnlock()
	return up
}

// MTU returns dev's maximum transmission unit, or 0 if no MTU is set.
func (dev *EthernetDevice) MTU() uint64 {
	dev.mu.RLock()
	mtu := dev.iface.MTU()
	dev.mu.RUnlock()
	return mtu
}

// SetMTU sets dev's maximum transmission unit, returning any error encountered.
func (dev *EthernetDevice) SetMTU(mtu uint64) error {
	dev.mu.RLock()
	err := errors.Annotate(dev.iface.SetMTU(mtu), "set mtu on device")
	dev.mu.RUnlock()
	return err
}

func (dev *EthernetDevice) ReadFrom(b []byte) (n int, hdr IPHeader, err error) {
	panic("not implemented")
}

// ReadFromIPv4 is like ReadFrom, but for IPv4 only.
func (dev *EthernetDevice) ReadFromIPv4(b []byte) (n int, hdr IPv4Header, err error) {
	panic("not implemented")
}

// ReadFromIPv6 is like ReadFrom, but for IPv6 only.
func (dev *EthernetDevice) ReadFromIPv6(b []byte) (n int, hdr IPv6Header, err error) {
	panic("not implemented")
}

func (dev *EthernetDevice) WriteTo(b []byte, hdr IPHeader, dst IP) (n int, err error) {
	hdr4, hdrOK := hdr.(*IPv4Header)
	dst4, dstOK := dst.(IPv4)
	switch {
	case hdrOK && dstOK:
		return dev.WriteToIPv4(b, hdr4, dst4)
	case !hdrOK && !dstOK:
		return dev.WriteToIPv6(b, hdr.(*IPv6Header), dst.(IPv6))
	case hdrOK && !dstOK:
		return 0, fmt.Errorf("write to device: IPv4 header with IPv6 address")
	default:
		return 0, fmt.Errorf("write to device: IPv6 header with IPv4 address")
	}
}

// WriteToIPv4 is like WriteTo, but for IPv4 only.
func (dev *EthernetDevice) WriteToIPv4(b []byte, hdr *IPv4Header, dst IPv4) (n int, err error) {
	dev.mu.RLock()
	defer dev.mu.RUnlock()

	buf := encodeHeaderAndBody(b, hdr)
	mac, err := dev.arp.LookupIPv4(dst)
	if err != nil {
		return 0, errors.Annotate(err, "write to device")
	}

	n, err = dev.iface.WriteFrame(buf, mac, EtherTypeIPv4)
	hdrlen := hdr.EncodedLen()
	if n < hdrlen {
		n = 0
	} else {
		n -= hdrlen
	}
	if err != nil {
		return n, errors.Annotate(err, "write to device")
	}
	return n, nil
}

// WriteToIPv6 is like WriteTo, but for IPv6 only.
func (dev *EthernetDevice) WriteToIPv6(b []byte, hdr *IPv6Header, dst IPv6) (n int, err error) {
	dev.mu.RLock()
	defer dev.mu.RUnlock()

	buf := encodeHeaderAndBody(b, hdr)
	mac, err := dev.arp.LookupIPv6(dst)
	if err != nil {
		return 0, errors.Annotate(err, "write to device")
	}

	n, err = dev.iface.WriteFrame(buf, mac, EtherTypeIPv6)
	hdrlen := hdr.EncodedLen()
	if n < hdrlen {
		n = 0
	} else {
		n -= hdrlen
	}
	if err != nil {
		return n, errors.Annotate(err, "write to device")
	}
	return n, nil
}

func encodeHeaderAndBody(b []byte, hdr IPHeader) []byte {
	hdrlen := hdr.EncodedLen()
	buf := getByteSlice(len(b) + hdrlen)
	hdr.Marshal(buf)
	copy(buf[hdrlen:], b)
	return b
}
