package net

import (
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

func (dev *UDPIPv4Device) ReadIPv4(b []byte) (n int, err error) {
	dev.mu.RLock()
	defer dev.mu.RUnlock()
	if !dev.isUp() {
		return 0, errors.New("read from to down device")
	}

	n, err = dev.conn.Read(b)
	return n, errors.Annotate(err, "read from device")
}

// WriteToIPv4 is like Device's WriteTo, but for IPv4 only.
func (dev *UDPIPv4Device) WriteToIPv4(b []byte, dst IPv4) (n int, err error) {
	dev.mu.RLock()
	defer dev.mu.RUnlock()
	if !dev.isUp() {
		return 0, errors.New("write to down device")
	}

	n, err = dev.conn.Write(b)
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
