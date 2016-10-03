package net

import (
	"io"
	"net"
	"sync"
	"time"

	"github.com/juju/errors"
)

// UDPIPv4Device represents a device created by sending link-layer packets over
// UDP. A UDPIPv4Device is only capable of sending and receiving IPv4 packets.
// UDPIPv4Device are point-to-point - there is always exactly one other
// link-local device.
//
// The zero UDPIPv4Device is not a valid UDPIPv4Device. UDPIPv4Device are safe
// for concurrent access.
type UDPIPv4Device struct {
	laddr, raddr  *net.UDPAddr
	conn          *net.UDPConn // nil if down
	addr, netmask IPv4         // unset if zero value

	mu sync.RWMutex
}

var _ Device = &UDPIPv4Device{}

// NewUDPIPv4Device creates a new UDPDevice, which is down by default.
func NewUDPIPv4Device(laddr, raddr *net.UDPAddr) (dev *UDPIPv4Device, err error) {
	return &UDPIPv4Device{laddr: laddr, raddr: raddr}, nil
}

// IPv4 returns dev's IPv4 address and network mask if they have been set.
func (dev *UDPIPv4Device) IPv4() (ok bool, addr, netmask IPv4) {
	dev.mu.RLock()
	defer dev.mu.RUnlock()
	if dev.addr == (IPv4{}) {
		return false, addr, netmask
	}
	return true, dev.addr, dev.netmask
}

// SetIPv4 sets dev's IPv4 address and network mask, returning any error
// encountered. SetIPv4 can only be  called when dev is down.
//
// Calling SetIPv4 with the zero value for addr unsets the IPv4 address.
func (dev *UDPIPv4Device) SetIPv4(addr, netmask IPv4) error {
	dev.mu.Lock()
	defer dev.mu.Unlock()
	if dev.isUp() {
		return errors.New("set device IP address on up device")
	}
	dev.addr, dev.netmask = addr, netmask
	return nil
}

// BringUp brings dev up. If it is already up, BringUp is a no-op.
func (dev *UDPIPv4Device) BringUp() error {
	dev.mu.Lock()
	defer dev.mu.Unlock()
	if dev.isUp() {
		return nil
	}

	conn, err := net.DialUDP("udp", dev.laddr, dev.raddr)
	if err != nil {
		return errors.Annotate(err, "bring device up")
	}
	dev.conn = conn
	return nil
}

// BringDown brings dev down. If it is already down, BringDown is a no-op.
func (dev *UDPIPv4Device) BringDown() error {
	dev.mu.Lock()
	defer dev.mu.Unlock()
	if !dev.isUp() {
		return nil
	}

	err := dev.conn.Close()
	dev.conn = nil
	return errors.Annotate(err, "bring device down")
}

// IsUp returns true if dev is up.
func (dev *UDPIPv4Device) IsUp() bool {
	dev.mu.RLock()
	up := dev.isUp()
	dev.mu.RUnlock()
	return up
}

func (dev *UDPIPv4Device) isUp() bool {
	return dev.conn != nil
}

// MTU returns 0; UDPIPv4Devices do not support MTUs.
func (dev *UDPIPv4Device) MTU() uint64 { return 0 }

// SetMTU returns an error; UDPIPv4Devices do not support MTUs.
func (dev *UDPIPv4Device) SetMTU(mtu uint64) error {
	return errors.New("UDPIPv4Device does not support setting MTU")
}

func (dev *UDPIPv4Device) ReadFrom(b []byte) (n int, hdr IPHeader, err error) {
	n, ihdr, err := dev.ReadFromIPv4(b)
	if ihdr != nil {
		hdr = ihdr
	}
	return n, hdr, err
}

func (dev *UDPIPv4Device) ReadFromIPv4(b []byte) (n int, hdr *IPv4Header, err error) {
	dev.mu.RLock()
	defer dev.mu.RUnlock()
	if !dev.isUp() {
		return 0, nil, errors.New("read from to down device")
	}

	buf := getByteSlice(ipv4HeaderMaxLen + len(b))
	nn, err := dev.conn.Read(buf)
	buf = buf[:nn]
	if err != nil {
		return 0, nil, errors.Annotate(err, "read from device")
	}
	hdr = new(IPv4Header)
	err = hdr.Unmarshal(buf)
	if err != nil {
		return 0, nil, errors.Annotate(err, "read from device")
	}
	buf = buf[hdr.EncodedLen():]
	n = copy(b, buf)
	if len(b) < len(buf) {
		return n, hdr, io.EOF
	}
	return n, hdr, nil
}

func (dev *UDPIPv4Device) WriteTo(b []byte, hdr IPHeader, dst IP) (n int, err error) {
	hdr4, hdrOK := hdr.(*IPv4Header)
	dst4, dstOK := dst.(IPv4)
	switch {
	case hdrOK && dstOK:
		return dev.WriteToIPv4(b, hdr4, dst4)
	case !hdrOK && !dstOK:
		return 0, errors.New("write IPv6 packet to IPv4-only device")
	case hdrOK && !dstOK:
		return 0, errors.New("write to device: IPv4 header with IPv6 address")
	default:
		return 0, errors.New("write to device: IPv6 header with IPv4 address")
	}
}

// WriteToIPv4 is like Device's WriteTo, but for IPv4 only.
func (dev *UDPIPv4Device) WriteToIPv4(b []byte, hdr *IPv4Header, dst IPv4) (n int, err error) {
	dev.mu.RLock()
	defer dev.mu.RUnlock()
	if !dev.isUp() {
		return 0, errors.New("write to down device")
	}

	buf := encodeHeaderAndBody(b, hdr, 0)
	n, err = dev.conn.Write(buf)
	hdrlen := hdr.EncodedLen()
	if n < hdrlen {
		n = 0
	} else {
		n -= hdrlen
	}
	return n, errors.Annotate(err, "write to device")
}

func (dev *UDPIPv4Device) SetReadDeadline(t time.Time) error {
	dev.mu.RLock()
	defer dev.mu.RUnlock()
	if !dev.isUp() {
		return errors.New("set read deadline on down device")
	}
	return dev.conn.SetReadDeadline(t)
}

func (dev *UDPIPv4Device) SetWriteDeadline(t time.Time) error {
	dev.mu.RLock()
	defer dev.mu.RUnlock()
	if !dev.isUp() {
		return errors.New("set write deadline on down device")
	}
	return dev.conn.SetWriteDeadline(t)
}

func (dev *UDPIPv4Device) SetDeadline(t time.Time) error {
	dev.mu.RLock()
	defer dev.mu.RUnlock()
	if !dev.isUp() {
		return errors.New("set deadline on down device")
	}
	return dev.conn.SetDeadline(t)
}
