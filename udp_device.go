package net

import (
	"net"
	"time"

	"github.com/juju/errors"
)

// UDPIPv4Device represents a device created by sending link-layer packets over
// UDP. A UDPIPv4Device is only capable of sending and receiving IPv4 packets.
// UDPIPv4Device are point-to-point - there is always exactly one other
// link-local device.
//
// The zero UDPIPv4Device is not a valid UDPIPv4Device. UDPIPv4Devices are safe
// for concurrent access.
type UDPIPv4Device struct {
	laddr, raddr  *net.UDPAddr
	conn          *net.UDPConn // only a listening connection; down if nil
	mtu           int
	addr, netmask IPv4           // unset if zero value
	callback      func(b []byte) // unset if nil

	sync syncer
}

var _ Device = &UDPIPv4Device{}

// NewUDPIPv4Device creates a new UDPDevice, which is down by default.
// It is the caller's responsibility to ensure that both sides of the connection
// are configured with the same MTU, which must be non-zero. Keep in mind that
// a single MTU-sized buffer will be allocated in order to read incoming packets,
// so an overly-large MTU will result in significant memory waste.
func NewUDPIPv4Device(laddr, raddr *net.UDPAddr, mtu int) (dev *UDPIPv4Device, err error) {
	if mtu == 0 {
		return nil, errors.New("new UDPIPv4Device: zero MTU")
	}
	return &UDPIPv4Device{laddr: laddr, raddr: raddr, mtu: mtu}, nil
}

// UDPAddrs returns the local and remote UDP addresses used by dev.
func (dev *UDPIPv4Device) UDPAddrs() (laddr, raddr *net.UDPAddr) {
	dev.sync.RLock()
	laddr, raddr = dev.laddr, dev.raddr
	dev.sync.RUnlock()
	return laddr, raddr
}

// IPv4 returns dev's IPv4 address and network mask if they have been set.
func (dev *UDPIPv4Device) IPv4() (ok bool, addr, netmask IPv4) {
	dev.sync.RLock()
	defer dev.sync.RUnlock()
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
	dev.sync.Lock()
	defer dev.sync.Unlock()
	if dev.isUp() {
		return errors.New("set device IP address on up device")
	}
	dev.addr, dev.netmask = addr, netmask
	return nil
}

// BringUp brings dev up. If it is already up, BringUp is a no-op.
func (dev *UDPIPv4Device) BringUp() error {
	return dev.sync.BringUp(func() error {
		dev.sync.Lock()
		defer dev.sync.Unlock()
		// NOTE(joshlf): Don't need to check whether the device is up already;
		// dev.sync.BringUp guarantees that we'll only be called if the device
		// is down.

		conn, err := net.ListenUDP("udp", dev.laddr)
		if err != nil {
			return errors.Annotate(err, "bring device up")
		}
		dev.conn = conn
		return nil
	}, dev.readDaemon)
}

// BringDown brings dev down. If it is already down, BringDown is a no-op.
func (dev *UDPIPv4Device) BringDown() error {
	return dev.sync.BringDown(func() error {
		dev.sync.Lock()
		defer dev.sync.Unlock()
		// NOTE(joshlf): Don't need to check whether the device is down already;
		// dev.sync.BringDown guarantees that we'll only be called if the device
		// is down.

		err := dev.conn.Close()
		dev.conn = nil
		return errors.Annotate(err, "bring device down")
	})
}

// IsUp returns true if dev is up.
func (dev *UDPIPv4Device) IsUp() bool {
	dev.sync.RLock()
	up := dev.isUp()
	dev.sync.RUnlock()
	return up
}

func (dev *UDPIPv4Device) isUp() bool {
	return dev.conn != nil
}

// MTU returns dev's MTU.
func (dev *UDPIPv4Device) MTU() int { return dev.mtu }

// RegisterIPv4Callback registers f to be called when IPv4 packets are received.
func (dev *UDPIPv4Device) RegisterIPv4Callback(f func(b []byte)) {
	dev.sync.RLock()
	dev.callback = f
	dev.sync.RUnlock()
}

func (dev *UDPIPv4Device) WriteToIPv4(b []byte, dst IPv4) (n int, err error) {
	if len(b) > dev.mtu {
		return 0, mtuErr("write to device: IPv4 payload exceeds MTU")
	}
	dev.sync.RLock()
	defer dev.sync.RUnlock()
	if !dev.isUp() {
		return 0, errors.New("write to down device")
	}

	n, err = dev.conn.WriteToUDP(b, dev.raddr)
	return n, errors.Annotate(err, "write to device")
}

func (dev *UDPIPv4Device) readDaemon() {
	b := make([]byte, dev.mtu)
	for {
		select {
		case <-dev.sync.StopChan():
			return
		default:
		}

		dev.sync.RLock()
		err := dev.conn.SetReadDeadline(time.Now().Add(time.Millisecond * 100))
		if err != nil {
			// TODO(joshlf): Log it
			dev.sync.RUnlock()
			continue
		}
		n, _, err := dev.conn.ReadFrom(b)
		// TODO(joshlf): ReadFrom doesn't seem to return an error if there's
		// a partial read, so we can't tell whether there was more data
		// (that is, whether the other side sent a larger frame than the
		// MTU allows). Maybe we need to define a simple header format
		// to carry explicit frame length information?
		if err != nil {
			if !IsTimeout(err) {
				// TODO(joshlf): Log it
			}
			dev.sync.RUnlock()
			continue
		}
		if dev.callback != nil {
			dev.callback(b[:n])
		}
		dev.sync.RUnlock()
	}
}
