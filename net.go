package net

import (
	"net"
	"time"

	"github.com/juju/errors"
)

// A Device is a handle on a physical or virtual network device.
//
// Devices are safe for concurrent access.
type Device interface {
	// IPv4 returns the Device's IPv4 address, if any.
	IPv4() (ok bool, addr IPv4)
	// IPv6 returns the Device's IPv6 address, if any.
	IPv6() (ok bool, addr IPv6)

	// BringUp brings the Device up. If it is already up,
	// BringUp is a no-op.
	BringUp() error
	// BringDown brings the Device down. If it is already down,
	// BringDown is a no-op.
	BringDown() error
	// IsUp returns true if the Device is up.
	IsUp() bool
	// IsDown returns true if the Device is down.
	IsDown() bool

	// GetMTU returns the device's maximum transmission unit,
	// or 0 if no MTU is set.
	GetMTU() uint64

	// SetMTU sets the maximum transmission unit on the device,
	// returning any error encountered. Note that
	SetMTU(mtu uint64) error

	// ReadFrom reads a packet from the device,
	// copying the payload into b. It returns the
	// number of bytes copied into b and the return
	// address and protocol that were on the packet.
	//
	// If a packet larger than len(b) is received,
	// n will be len(b), and err will be io.EOF
	ReadFrom(b []byte) (n int, addr net.Addr, proto Protocol, err error)
	// WriteTo writes a packet to the device with
	// the specified remote address and protocol.
	//
	// If len(b) is larger than the device's MTU,
	// WriteTo will not write the packet, and will
	// return an error such that IsMTU(err) == true.
	WriteTo(b []byte, addr net.Addr, proto Protocol) error

	// GetConn returns a DeviceConn which only reads packets
	// with the given protocol, unless proto is nil, in which
	// case all protocols are read. If proto is not nil, it is
	// an error to write a packet to dc with a protocol other
	// than proto.
	GetConn(proto Protocol) (dc DeviceConn, err error)
}

// A DeviceConn is a handle to reading packets from and writing packets to
// a Device. It provides functionality that is not global to the Device
// such as deadlines.
//
// DeviceConns are safe for concurrent access.
type DeviceConn interface {
	// ReadFrom reads a packet from the device,
	// copying the payload into b. It returns the
	// number of bytes copied into b and the return
	// address and protocol that were on the packet.
	// ReadFrom can be made to time out and return
	// an error with Timeout() == true after a fixed
	// time limit; see SetDeadline and SetReadDeadline.
	//
	// If a packet larger than len(b) is received,
	// n will be len(b), and err will be io.EOF
	ReadFrom(b []byte) (n int, addr net.Addr, proto Protocol, err error)

	// WriteTo writes a packet to the device with
	// the specified remote address and protocol.
	// WriteTo can be made to time out and return
	// an error with Timeout() == true after a fixed time limit;
	// see SetDeadline and SetWriteDeadline.
	// On device connections, write timeouts are rare.
	//
	// If len(b) is larger than the device's MTU,
	// WriteTo will not write the packet, and will
	// return an error such that IsMTU(err) == true.
	WriteTo(b []byte, addr net.Addr, proto Protocol) error

	// SetDeadline sets the read and write deadlines associated
	// with the connection.
	SetDeadline(t time.Time) error

	// SetReadDeadline sets the deadline for future Read calls.
	// If the deadline is reached, Read will fail with a timeout
	// (see type Error) instead of blocking.
	// A zero value for t means Read will not time out.
	SetReadDeadline(t time.Time) error

	// SetWriteDeadline sets the deadline for future Write calls.
	// If the deadline is reached, Write will fail with a timeout
	// (see type Error) instead of blocking.
	// A zero value for t means Write will not time out.
	// Even if write times out, it may return n > 0, indicating that
	// some of the data was successfully written.
	SetWriteDeadline(t time.Time) error
}

// A Protocol represents a protocol implemented on top of a particular
// network layer.
type Protocol interface {
	String() string
}

type mtuErr string

func (m mtuErr) Error() string { return string(m) }
func (m mtuErr) MTU() bool     { return true }

// IsMTU returns true if err is an MTU-related error,
// as defined by having an MTU() bool method which
// returns true.
func IsMTU(err error) bool {
	type mtu interface {
		MTU() bool
	}
	me, ok := errors.Cause(err).(mtu)
	return ok && me.MTU()
}

// IsTimeout returns true if err is a timeout-related error,
// as defined by having a Timeout() bool method which returns
// true.
func IsTimeout(err error) bool {
	type timeout interface {
		Timeout() bool
	}
	te, ok := errors.Cause(err).(timeout)
	return ok && te.Timeout()
}
