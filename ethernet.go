package net

import (
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"time"

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

	// ReadFrame reads an ethernet frame into b. b includes space
	// for both the ethernet header and the frame payload. n is the
	// number of bytes written to b (subtract the length of an ethernet
	// header to obtain the payload length). If the frame (including
	// header) was larger than len(b), n == len(b),
	// and err == io.EOF.
	//
	// If the interface has its MAC set, only ethernet frames whose
	// destination MAC is equal to the interface's MAC or is the
	// broadcast MAC will be returned.
	ReadFrame(b []byte) (n int, src, dst MAC, et EtherType, err error)
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

	ipv4, ipv6 chan []byte // nil if the device is down

	readDeadline atomic.Value // stores a time.Time value

	// Acquire a read lock for all operations.
	// Acquire a write lock to bring the device
	// up or down. When bringing the device down,
	// close the down channel and wg.Wait().
	// Then set ipv4 and ipv6 to nil. When
	// bringing the device up, initialize ipv4,
	// ipv6, down, and wg, then spawn worker
	// goroutines.
	down chan struct{}
	wg   sync.WaitGroup
	mu   sync.RWMutex
}

var _ Device = &EthernetDevice{}

// NewEthernetDevice creates a new EthernetDevice using iface for frame
// transport and addr as the interface's MAC address, and brings the
// device up.
func NewEthernetDevice(iface EthernetInterface, addr MAC) (*EthernetDevice, error) {
	err := iface.SetMAC(addr)
	if err != nil {
		return nil, errors.Annotate(err, "create new ethernet device")
	}
	dev := &EthernetDevice{
		iface: iface,
	}
	dev.readDeadline.Store(time.Time{})
	err = dev.BringUp()
	if err != nil {
		return nil, errors.Annotate(err, "create new ethernet device")
	}
	return dev, nil
}

// run in a separate goroutine to read packets from dev.iface
func (dev *EthernetDevice) packetReader() {
	// keep track of the buffer between loops
	// so that we don't reallocate after timeouts
	var buf []byte
	for {
		select {
		case <-dev.down:
			return
		default:
		}

		dev.mu.RLock()
		bufsize := int(dev.iface.MTU() + ethernetHeaderLen)
		if len(buf) < bufsize {
			buf = getByteSlice(bufsize)
		}

		dev.iface.SetReadDeadline(time.Now().Add(time.Millisecond * 100))
		n, src, dst, et, err := dev.iface.ReadFrame(buf)
		if err != nil {
			if IsTimeout(err) {
				dev.mu.RUnlock()
				continue
			}
			// TODO(joshlf): Log it
		} else {
			buf = buf[ethernetHeaderLen:n]
			switch et {
			case EtherTypeIPv4:
				dev.ipv4 <- buf
			case EtherTypeIPv6:
				dev.ipv6 <- buf
			case EtherTypeARP:
				err = dev.arp.HandlePacket(src, dst, buf)
				if err != nil {
					// TODO(joshlf)
				}
			default:
				// drop it
				// TODO(joshlf): Log it?
			}
			buf = nil
			dev.mu.RUnlock()
		}
	}
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
	dev.down = make(chan struct{})
	dev.ipv4 = make(chan []byte, 8)
	dev.ipv6 = make(chan []byte, 8)
	dev.wg = sync.WaitGroup{}
	dev.wg.Add(1)
	go func() { dev.packetReader(); dev.wg.Done() }()
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

	close(dev.down)
	dev.wg.Wait()
	// make sure all workers have returned before stopping ARP
	dev.arp.Stop()
	dev.arp = nil
	dev.ipv4 = nil
	dev.ipv6 = nil
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
	return dev.arp != nil
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

// readFrom performs a generic read. To read only IPv4, set ipv6 to nil.
// To read only IPv6, set ipv4 to nil. Assumes dev.mu.RLock().
func (dev *EthernetDevice) readFrom(b []byte, ipv4, ipv6 chan []byte) (n int, hdr IPHeader, err error) {
	var buf []byte
	for {
		select {
		case <-dev.getReadDeadlineTimer():
			return 0, nil, timeout("read timeout")
		case buf = <-ipv4:
			hdr = new(IPv4Header)
		case buf = <-ipv6:
			hdr = new(IPv6Header)
		}
		err = hdr.Unmarshal(buf)
		if err != nil {
			// TODO(joshlf): Log it
			continue
		}
		buf = buf[hdr.EncodedLen():]
		n = copy(b, buf)
		if len(b) < len(buf) {
			return n, hdr, io.EOF
		}
		return n, hdr, nil
	}
}

// ReadFrom reads an IP packet from dev, copying the payload into b.
// It returns the number of bytes copied and the IP header on the packet.
// ReadFrom can be made to time out and return an error after a fixed
// time limit; see IsTimeout, SetDeadline, and SetReadDeadline.
//
// If a packet whose payload is larger than len(b) is received, n will
// be len(b), and err will be io.EOF.
func (dev *EthernetDevice) ReadFrom(b []byte) (n int, hdr IPHeader, err error) {
	dev.mu.RLock()
	defer dev.mu.RUnlock()
	if !dev.isUp() {
		return 0, nil, errors.New("read from down device")
	}

	return dev.readFrom(b, dev.ipv4, dev.ipv6)
}

// ReadFromIPv4 is like ReadFrom, but for IPv4 only.
func (dev *EthernetDevice) ReadFromIPv4(b []byte) (n int, hdr *IPv4Header, err error) {
	dev.mu.RLock()
	defer dev.mu.RUnlock()
	if !dev.isUp() {
		return 0, nil, errors.New("read from down device")
	}

	n, ihdr, err := dev.readFrom(b, dev.ipv4, nil)
	if ihdr != nil {
		hdr = ihdr.(*IPv4Header)
	}
	return n, hdr, err
}

// ReadFromIPv6 is like ReadFrom, but for IPv6 only.
func (dev *EthernetDevice) ReadFromIPv6(b []byte) (n int, hdr *IPv6Header, err error) {
	dev.mu.RLock()
	defer dev.mu.RUnlock()
	if !dev.isUp() {
		return 0, nil, errors.New("read from down device")
	}

	n, ihdr, err := dev.readFrom(b, nil, dev.ipv6)
	if ihdr != nil {
		hdr = ihdr.(*IPv6Header)
	}
	return n, hdr, err
}

// if no deadline is set, return nil. Otherwise, return
// a channel in the manner of time.After
func (dev *EthernetDevice) getReadDeadlineTimer() <-chan time.Time {
	now := time.Now()
	deadline := dev.readDeadline.Load().(time.Time)
	if deadline == (time.Time{}) {
		return nil
	}
	if now.Before(deadline) {
		return time.After(deadline.Sub(now))
	}
	// the deadline is already here
	c := make(chan time.Time, 1)
	c <- deadline
	return c
}

// WriteTo writes an IP packet to the device with the specified header
// and to the given destination address. The destination address is
// resolved to a link-local address, and the resulting link-layer frame
// is sent to that address. The destination address does not have to
// match the desitnation address in the IP packet header.
//
// WriteTo can be made to time out and return an error after a fixed
// time limit; see IsTimeout, SetDeadline, and SetWriteDeadline.
//
// If len(b) + hdr.EncodedLen() is larger than the device's MTU,
// WriteTo will not write the packet, and will return an MTU error
// (see IsMTU).
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
	if !dev.isUp() {
		return 0, errors.New("write to down device")
	}

	buf := encodeHeaderAndBody(b, hdr)
	mac, err := dev.arp.LookupIPv4(dst)
	if err != nil {
		return 0, errors.Annotate(err, "write to device")
	}

	n, err = dev.iface.WriteFrame(buf, mac, EtherTypeIPv4)
	hdrlen := ethernetHeaderLen + hdr.EncodedLen()
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
	if !dev.isUp() {
		return 0, errors.New("write to down device")
	}

	buf := encodeHeaderAndBody(b, hdr)
	mac, err := dev.arp.LookupIPv6(dst)
	if err != nil {
		return 0, errors.Annotate(err, "write to device")
	}

	n, err = dev.iface.WriteFrame(buf, mac, EtherTypeIPv6)
	hdrlen := ethernetHeaderLen + hdr.EncodedLen()
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

// SetReadDeadline sets the deadline for future calls to ReadFrom,
// ReadFromIPv4, and ReadFromIPv6. If the deadline is reached,
// these calls will fail with a timeout (see IsTimeout) instead
// of blocking. A zero value for t means read calls will not time out.
func (dev *EthernetDevice) SetReadDeadline(t time.Time) error {
	dev.mu.RLock()
	defer dev.mu.RUnlock()
	if !dev.isUp() {
		return errors.New("set read deadline on down device")
	}
	dev.readDeadline.Store(t)
	return nil
}

// SetWriteDeadline sets the deadline for future calls to WriteTo,
// WriteToIPv4, and WriteToIPv6. If the deadline is reached,
// these calls will fail with a timeout (see IsTimeout) instead
// of blocking. A zero value for t means read calls will not time out.
//
// Write timeouts on EthernetDevices are very rare.
func (dev *EthernetDevice) SetWriteDeadline(t time.Time) error {
	dev.mu.RLock()
	defer dev.mu.RUnlock()
	if !dev.isUp() {
		return errors.New("set write deadline on down device")
	}
	return dev.iface.SetReadDeadline(t)
}

// SetDeadline calls SetReadDeadline and SetWriteDeadline.
func (dev *EthernetDevice) SetDeadline(t time.Time) error {
	dev.mu.RLock()
	defer dev.mu.RUnlock()
	if !dev.isUp() {
		return errors.New("set deadline on down device")
	}
	// the only time SetReadDeadline can return an error
	// is when the device is down
	dev.SetReadDeadline(t)
	return dev.SetWriteDeadline(t)
}

// encodeHeaderAndBody encodes an IP packet with the payload
// b and the header hdr. The returned byte slice includes
// space for an ethernet frame header.
func encodeHeaderAndBody(b []byte, hdr IPHeader) []byte {
	hdrlen := hdr.EncodedLen()
	buf := getByteSlice(ethernetHeaderLen + hdrlen + len(b))
	hdr.Marshal(buf[ethernetHeaderLen:])
	copy(buf[ethernetHeaderLen+hdrlen:], b)
	return b
}
