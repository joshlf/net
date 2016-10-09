package net

import (
	"sync"

	"github.com/juju/errors"
)

// An EthernetInterface is a low-level interface capable of reading
// and writing Ethernet frames.
//
// An EthernetInterface is safe for concurrent access.
type EthernetInterface interface {
	// BringUp brings the interface up. If it is already up,
	// BringUp is a no-op.
	BringUp() error
	// BringDown brings the interface down. If it is already down,
	// BringDown is a no-op.
	BringDown() error
	// IsUp returns true if the interface is up.
	IsUp() bool

	// MAC returns the interface's MAC address, if any.
	MAC() (ok bool, mac MAC)
	// SetMAC sets the interface's MAC address. It is an error
	// to call SetMAC with the broadcast MAC, or while the
	// interface is up.
	SetMAC(mac MAC) error

	// MTU returns the interface's MTU. If no MTU is set, MTU will return 0.
	MTU() int
	// SetMTU sets the interface's MTU. It is an error to set
	// an MTU of 0 or to call SetMTU while the interface is up.
	SetMTU(mtu uint64) error

	// RegisterCallback registers f as the function to be called
	// when a new Ethernet frame arrives. It overwrites any
	// previously-registered callbacks. If f is nil, incoming
	// Ethernet frames will be dropped.
	//
	// If the interface has its MAC set, only Ethernet frames
	// whose destination MAC is equal to the interface's MAC or
	// is the broadcast MAC will be returned.
	//
	// RegisterCallback can only be called while the interface
	// is down.
	RegisterCallback(f func(b []byte, src, dst MAC, et EtherType))
	// WriteFrame writes an Ethernet frame with the payload b.
	// b is expected to contain space preceding the payload itself
	// for the Ethernet header, which WriteFrame is responsible
	// for writing. If the interface has an MTU set, and len(b)
	// is larger than that MTU plus the length of an Ethernet header,
	// the frame will not be written, and instead WriteFrame will
	// return an MTU error (see IsMTU).
	//
	// If the destination MAC is the broadcast MAC, the frame will
	// be broadcast to all devices on the local Ethernet network.
	//
	// If a MAC address has been set, that will be used as the
	// frame's source MAC. Otherwise, WriteFrame will return an error.
	WriteFrame(b []byte, dst MAC, et EtherType) (n int, err error)
	// WriteFrameSrc is like WriteFrame, but allows the source MAC
	// address to be set explicitly.
	WriteFrameSrc(b []byte, src, dst MAC, et EtherType) (n int, err error)
}

// EtherType is a value of 1536 or greater which indicates
// the protocol type of a packet encapsulated in an Ethernet frame.
type EtherType uint16

const (
	EtherTypeIPv4 EtherType = 0x0800
	EtherTypeARP  EtherType = 0x0806
	EtherTypeIPv6 EtherType = 0x86DD
)

// MAC is an Ethernet media access control address.
type MAC [6]byte

// BroadcastMAC is the broadcast MAC address.
var BroadcastMAC = MAC{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}

// An EthernetDevice is a device which uses an EthernetInterface
// as its underlying frame transport mechanism. It implements
// the Device interface.
type EthernetDevice struct {
	iface EthernetInterface
	up    bool
	// arp                  *arp         // nil if the device is down
	addr4, netmask4      IPv4         // unset if zero value
	addr6, netmask6      IPv6         // unset if zero value
	callback4, callback6 func([]byte) // unset if nil

	// ipv4, ipv6 chan []byte // nil if the device is down

	// readDeadline atomic.Value // stores a time.Time value

	// Acquire a read lock for all operations.
	// Acquire a write lock to bring the device
	// up or down. When bringing the device down,
	// close the down channel and wg.Wait(). Then
	// arp.Stop() and set arp, ipv4, ipv6 to nil.
	// When bringing the device up, initialize
	// ipv4, ipv6, down, wg, and arp, then spawn
	// worker goroutines.
	// down chan struct{}
	// wg   sync.WaitGroup
	mu sync.RWMutex
}

var _ Device = &EthernetDevice{}     // make sure *EthernetDevice implements Device
var _ IPv4Device = &EthernetDevice{} // make sure *EthernetDevice implements IPv4Device
var _ IPv6Device = &EthernetDevice{} // make sure *EthernetDevice implements IPv6Device

// NewEthernetDevice creates a new EthernetDevice using iface for frame
// transport and addr as the interface's MAC address. iface is assumed
// to be down. After a successful call to NewEthernetDevice, the returned
// EthernetDevice is considered to own iface; modifications to iface by
// the caller may result in undefined behavior. The returned device
// is down, and has no associated IPv4 or IPv6 addresses.
func NewEthernetDevice(iface EthernetInterface, addr MAC) (*EthernetDevice, error) {
	err := iface.SetMAC(addr)
	if err != nil {
		return nil, errors.Annotate(err, "create new ethernet device")
	}
	dev := &EthernetDevice{
		iface: iface,
	}
	iface.RegisterCallback(dev.callback)
	return dev, nil
}

func (dev *EthernetDevice) callback(b []byte, src, dst MAC, et EtherType) {
	dev.mu.RLock()
	defer dev.mu.RUnlock()
	if !dev.isUp() {
		return
	}

	switch et {
	case EtherTypeARP:
		// TODO(joshlf): Implement ARP
	case EtherTypeIPv4:
		if dev.callback4 != nil {
			dev.callback4(b)
		}
	case EtherTypeIPv6:
		if dev.callback6 != nil {
			dev.callback6(b)
		}
	}
}

// RegisterIPv4Callback implements IPv4Device's RegisterIPv4Callback.
func (dev *EthernetDevice) RegisterIPv4Callback(f func(b []byte)) {
	dev.mu.Lock()
	dev.callback4 = f
	dev.mu.Unlock()
}

// RegisterIPv6Callback implements IPv6Device's RegisterIPv6Callback.
func (dev *EthernetDevice) RegisterIPv6Callback(f func(b []byte)) {
	dev.mu.Lock()
	dev.callback6 = f
	dev.mu.Unlock()
}

// IPv4 returns dev's IPv4 address and network mask if they have been set.
func (dev *EthernetDevice) IPv4() (addr, netmask IPv4, ok bool) {
	dev.mu.RLock()
	defer dev.mu.RUnlock()
	if dev.addr4 == (IPv4{}) {
		return addr, netmask, false
	}
	return dev.addr4, dev.netmask4, true
}

// SetIPv4 sets dev's IPv4 address and network mask, returning any error
// encountered. SetIPv4 can only be called when dev is down.
//
// Calling SetIPv4 with the zero value for addr unsets the IPv4 address.
func (dev *EthernetDevice) SetIPv4(addr, netmask IPv4) error {
	dev.mu.Lock()
	defer dev.mu.Unlock()
	if dev.isUp() {
		return errors.New("set device IP address on up device")
	}
	dev.addr4, dev.netmask4 = addr, netmask
	return nil
}

// IPv6 returns dev's IPv6 address and network mask if they have been set.
func (dev *EthernetDevice) IPv6() (addr, netmask IPv6, ok bool) {
	dev.mu.RLock()
	defer dev.mu.RUnlock()
	if dev.addr6 == (IPv6{}) {
		return addr, netmask, false
	}
	return dev.addr6, dev.netmask6, true
}

// SetIPv6 sets dev's IPv6 address and network mask, returning any error
// encountered. SetIPv6 can only be called when dev is down.
//
// Calling SetIPv6 with the zero value for addr unsets the IPv6 address.
func (dev *EthernetDevice) SetIPv6(addr, netmask IPv6) error {
	dev.mu.Lock()
	defer dev.mu.Unlock()
	if dev.isUp() {
		return errors.New("set device IP address on up device")
	}
	dev.addr6, dev.netmask6 = addr, netmask
	return nil
}

// BringUp brings dev up. If it is already up, BringUp is a no-op.
func (dev *EthernetDevice) BringUp() error {
	dev.mu.Lock()
	defer dev.mu.Unlock()
	if dev.isUp() {
		return nil
	}

	err := dev.iface.BringUp()
	if err != nil {
		errors.Annotate(err, "bring device up")
	}
	dev.up = true
	return nil
}

// BringDown brings dev down. If it is already up, BringDown is a no-op.
func (dev *EthernetDevice) BringDown() error {
	dev.mu.Lock()
	defer dev.mu.Unlock()
	if !dev.isUp() {
		return nil
	}

	err := dev.iface.BringDown()
	if err != nil {
		return errors.Annotate(err, "bring device down")
	}
	dev.up = false
	return nil
}

// IsUp returns true if dev is up.
func (dev *EthernetDevice) IsUp() bool {
	dev.mu.RLock()
	up := dev.isUp()
	dev.mu.RUnlock()
	return up
}

func (dev *EthernetDevice) isUp() bool {
	return dev.up
}

// MTU returns dev's maximum transmission unit, or 0 if no MTU is set.
func (dev *EthernetDevice) MTU() int {
	dev.mu.RLock()
	mtu := dev.iface.MTU()
	dev.mu.RUnlock()
	return mtu
}

func (dev *EthernetDevice) WriteToIPv4(b []byte, dst IPv4) (n int, err error) {
	dev.mu.RLock()
	defer dev.mu.RUnlock()
	if !dev.isUp() {
		return 0, errors.New("write to down device")
	}

	buf := make([]byte, ethernetHeaderLen+len(b))
	copy(buf[ethernetHeaderLen:], b)
	var mac MAC // TODO(joshlf): Look it up in ARP
	if err != nil {
		return 0, errors.Annotate(err, "write to device")
	}
	return dev.writeTo(buf, mac)
}

func (dev *EthernetDevice) WriteToIPv6(b []byte, dst IPv6) (n int, err error) {
	dev.mu.RLock()
	defer dev.mu.RUnlock()
	if !dev.isUp() {
		return 0, errors.New("write to down device")
	}

	buf := make([]byte, ethernetHeaderLen+len(b))
	copy(buf[ethernetHeaderLen:], b)
	var mac MAC // TODO(joshlf): Look it up
	if err != nil {
		return 0, errors.Annotate(err, "write to device")
	}
	return dev.writeTo(buf, mac)
}

// writeTo implements logic common to WriteToIPv4 and WriteToIPv6;
// it writes to the given MAC address and returns the correct values
func (dev *EthernetDevice) writeTo(b []byte, mac MAC) (n int, err error) {
	n, err = dev.iface.WriteFrame(b, mac, EtherTypeIPv4)
	if n < ethernetHeaderLen {
		n = 0
	} else {
		n -= ethernetHeaderLen
	}
	return n, errors.Annotate(err, "write to device")
}
