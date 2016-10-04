package net

import (
	"sync"

	"github.com/juju/errors"
)

// An EthernetInterface is a low-level interface capable of reading
// and writing ethernet frames.
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
	// WriteFrame writes an ethernet frame with the payload b.
	// b is expected to contain space preceding the payload itself
	// for the ethernet header, which WriteFrame is responsible
	// for writing. If the interface has an MTU set, and len(b)
	// is larger than that MTU plus the length of an ethernet header,
	// the frame will not be written, and instead WriteFrame will
	// return an MTU error (see IsMTU).
	//
	// If the destination MAC is the broadcast MAC, the frame will
	// be broadcast to all devices on the local ethernet network.
	//
	// If a MAC address has been set, that will be used as the
	// frame's source MAC. Otherwise, WriteFrame will return an error.
	WriteFrame(b []byte, dst MAC, et EtherType) (n int, err error)
	// WriteFrameSrc is like WriteFrame, but allows the source MAC
	// address to be set explicitly.
	WriteFrameSrc(b []byte, src, dst MAC, et EtherType) (n int, err error)
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
// is down, and has no associated IPv4 or IPv6 address.
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

// // run in a separate goroutine to read packets from dev.iface
// func (dev *EthernetDevice) packetReader() {
// 	// keep track of the buffer between loops
// 	// so that we don't reallocate after timeouts
// 	var buf []byte
// 	for {
// 		select {
// 		case <-dev.down:
// 			return
// 		default:
// 		}
//
// 		dev.mu.RLock()
// 		bufsize := int(dev.iface.MTU() + ethernetHeaderLen)
// 		if len(buf) < bufsize {
// 			buf = getByteSlice(bufsize)
// 		}
//
// 		dev.iface.SetReadDeadline(time.Now().Add(time.Millisecond * 100))
// 		n, src, dst, et, err := dev.iface.ReadFrame(buf)
// 		if err != nil {
// 			if IsTimeout(err) {
// 				dev.mu.RUnlock()
// 				continue
// 			}
// 			// TODO(joshlf): Log it
// 		} else {
// 			buf = buf[ethernetHeaderLen:n]
// 			switch et {
// 			case EtherTypeIPv4:
// 				dev.ipv4 <- buf
// 			case EtherTypeIPv6:
// 				dev.ipv6 <- buf
// 			case EtherTypeARP:
// 				err = dev.arp.HandlePacket(src, dst, buf)
// 				if err != nil {
// 					// TODO(joshlf)
// 				}
// 			default:
// 				// drop it
// 				// TODO(joshlf): Log it?
// 			}
// 			buf = nil
// 			dev.mu.RUnlock()
// 		}
// 	}
// }

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

func (dev *EthernetDevice) RegisterIPv4Callback(f func(b []byte)) {
	dev.mu.Lock()
	dev.callback4 = f
	dev.mu.Unlock()
}

func (dev *EthernetDevice) RegisterIPv6Callback(f func(b []byte)) {
	dev.mu.Lock()
	dev.callback6 = f
	dev.mu.Unlock()
}

// IPv4 returns dev's IPv4 address and network mask if they have been set.
func (dev *EthernetDevice) IPv4() (ok bool, addr, netmask IPv4) {
	dev.mu.RLock()
	defer dev.mu.RUnlock()
	if dev.addr4 == (IPv4{}) {
		return false, addr, netmask
	}
	return true, dev.addr4, dev.netmask4
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
func (dev *EthernetDevice) IPv6() (ok bool, addr, netmask IPv6) {
	dev.mu.RLock()
	defer dev.mu.RUnlock()
	if dev.addr6 == (IPv6{}) {
		return false, addr, netmask
	}
	return true, dev.addr6, dev.netmask6
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

// WriteToIPv4 is like WriteTo, but for IPv4 only.
func (dev *EthernetDevice) WriteToIPv4(b []byte, dst IPv4) (n int, err error) {
	dev.mu.RLock()
	defer dev.mu.RUnlock()
	if !dev.isUp() {
		return 0, errors.New("write to down device")
	}

	buf := getByteSlice(ethernetHeaderLen + len(b))
	copy(buf[ethernetHeaderLen:], b)
	var mac MAC // TODO(joshlf): Look it up in ARP
	if err != nil {
		return 0, errors.Annotate(err, "write to device")
	}
	return dev.writeTo(buf, mac)
}

// WriteToIPv6 is like WriteTo, but for IPv6 only.
func (dev *EthernetDevice) WriteToIPv6(b []byte, dst IPv6) (n int, err error) {
	dev.mu.RLock()
	defer dev.mu.RUnlock()
	if !dev.isUp() {
		return 0, errors.New("write to down device")
	}

	buf := getByteSlice(ethernetHeaderLen + len(b))
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

//
// // readFrom performs a generic read
// func (dev *EthernetDevice) readFrom(b []byte, packets chan []byte) (n int, err error) {
// 	select {
// 	case <-dev.getReadDeadlineTimer():
// 		return 0, timeout("read timeout")
// 	case buf := <-packets:
// 		n = copy(b, buf)
// 		if len(b) < len(buf) {
// 			return n, io.EOF
// 		}
// 		return n, nil
// 	}
// }

// // ReadFrom reads an IP packet from dev, copying the payload into b.
// // It returns the number of bytes copied and the IP header on the packet.
// // ReadFrom can be made to time out and return an error after a fixed
// // time limit; see IsTimeout, SetDeadline, and SetReadDeadline.
// //
// // If a packet whose payload is larger than len(b) is received, n will
// // be len(b), and err will be io.EOF.
// func (dev *EthernetDevice) Read(b []byte) (n int, err error) {
// 	dev.mu.RLock()
// 	defer dev.mu.RUnlock()
// 	if !dev.isUp() {
// 		return 0, errors.New("read from down device")
// 	}
//
// 	return dev.readFrom(b, dev.ipv4, dev.ipv6)
// }

// // ReadIPv4 is like Read, but for IPv4 only.
// func (dev *EthernetDevice) ReadIPv4(b []byte) (n int, err error) {
// 	dev.mu.RLock()
// 	defer dev.mu.RUnlock()
// 	if !dev.isUp() {
// 		return 0, errors.New("read from down device")
// 	}
//
// 	return dev.readFrom(b, dev.ipv4)
// }

// ReadIPv6 is like Read, but for IPv6 only.
// func (dev *EthernetDevice) ReadIPv6(b []byte) (n int, err error) {
// 	dev.mu.RLock()
// 	defer dev.mu.RUnlock()
// 	if !dev.isUp() {
// 		return 0, errors.New("read from down device")
// 	}
//
// 	return dev.readFrom(b, dev.ipv6)
// }

// if no deadline is set, return nil. Otherwise, return
// a channel in the manner of time.After
// func (dev *EthernetDevice) getReadDeadlineTimer() <-chan time.Time {
// 	now := time.Now()
// 	deadline := dev.readDeadline.Load().(time.Time)
// 	if deadline == (time.Time{}) {
// 		return nil
// 	}
// 	if now.Before(deadline) {
// 		return time.After(deadline.Sub(now))
// 	}
// 	// the deadline is already here
// 	c := make(chan time.Time, 1)
// 	c <- deadline
// 	return c
// }

// // WriteTo writes an IP packet to the device with the specified header
// // and to the given destination address. The destination address is
// // resolved to a link-local address, and the resulting link-layer frame
// // is sent to that address. The destination address does not have to
// // match the desitnation address in the IP packet header.
// //
// // WriteTo can be made to time out and return an error after a fixed
// // time limit; see IsTimeout, SetDeadline, and SetWriteDeadline.
// //
// // If len(b) + hdr.EncodedLen() is larger than the device's MTU,
// // WriteTo will not write the packet, and will return an MTU error
// // (see IsMTU).
// func (dev *EthernetDevice) WriteTo(b []byte, dst IP) (n int, err error) {
// 	switch dst := dst.(type) {
// 	case IPv4:
// 		return dev.WriteToIPv4(b, dst)
// 	case IPv6:
// 		return dev.WriteToIPv6(b, dst)
// 	default:
// 		panic("unreachable")
// 	}
// }

// // SetReadDeadline sets the deadline for future calls to ReadFrom,
// // ReadFromIPv4, and ReadFromIPv6. If the deadline is reached,
// // these calls will fail with a timeout (see IsTimeout) instead
// // of blocking. A zero value for t means read calls will not time out.
// func (dev *EthernetDevice) SetReadDeadline(t time.Time) error {
// 	dev.mu.RLock()
// 	defer dev.mu.RUnlock()
// 	if !dev.isUp() {
// 		return errors.New("set read deadline on down device")
// 	}
// 	dev.readDeadline.Store(t)
// 	return nil
// }
//
// // SetWriteDeadline sets the deadline for future calls to WriteTo,
// // WriteToIPv4, and WriteToIPv6. If the deadline is reached,
// // these calls will fail with a timeout (see IsTimeout) instead
// // of blocking. A zero value for t means read calls will not time out.
// //
// // Write timeouts on EthernetDevices are very rare.
// func (dev *EthernetDevice) SetWriteDeadline(t time.Time) error {
// 	dev.mu.RLock()
// 	defer dev.mu.RUnlock()
// 	if !dev.isUp() {
// 		return errors.New("set write deadline on down device")
// 	}
// 	return dev.iface.SetReadDeadline(t)
// }
//
// // SetDeadline calls SetReadDeadline and SetWriteDeadline.
// func (dev *EthernetDevice) SetDeadline(t time.Time) error {
// 	dev.mu.RLock()
// 	defer dev.mu.RUnlock()
// 	if !dev.isUp() {
// 		return errors.New("set deadline on down device")
// 	}
// 	// the only time SetReadDeadline can return an error
// 	// is when the device is down
// 	dev.SetReadDeadline(t)
// 	return dev.SetWriteDeadline(t)
// }

// // encodeHeaderAndBody encodes an IP packet with the payload
// // b and the header hdr. The returned byte slice includes
// // any extra space that is specified.
// func encodeHeaderAndBody(b []byte, hdr IPHeader, extra int) []byte {
// 	hdrlen := hdr.EncodedLen()
// 	buf := getByteSlice(extra + hdrlen + len(b))
// 	hdr.Marshal(buf[extra:])
// 	copy(buf[extra+hdrlen:], b)
// 	return b
// }
