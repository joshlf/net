package net

import (
	"time"

	"github.com/juju/errors"
)

// TODO(joshlf): Maybe rename Device to IPDevice
// These Devices only support IP operations, and
// maybe we want a lower-level device interface
// that allows writing directly to link-layer
// addresses. Alternatively:
//   - That interface should be called LinkDevice
//     (or similar)
//   - Those devices require special per-device
//     types (e.g., an ethernet MAC address), so
//     a common Go interface doesn't make sense.

// A Device is a handle on a physical or virtual network device. A Device
// must implement the IPv4Device or IPv6Device interfaces, although
// it may also implement both.
//
// Devices are safe for concurrent access.
type Device interface {
	// BringUp brings the Device up. If it is already up,
	// BringUp is a no-op.
	BringUp() error
	// BringDown brings the Device down. If it is already down,
	// BringDown is a no-op.
	BringDown() error
	// IsUp returns true if the Device is up.
	IsUp() bool

	// MTU returns the device's maximum transmission unit,
	// or 0 if no MTU is set.
	MTU() uint64
	// SetMTU sets the maximum transmission unit on the device,
	// returning any error encountered. Some devices may not support
	// MTUs, and SetMTU on such devices will return an error.
	SetMTU(mtu uint64) error

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
	ReadFrom(b []byte) (n int, hdr IPHeader, err error)
	// WriteTo writes a packet to the device with
	// the specified destination address.
	// WriteTo can be made to time out and return
	// an error with Timeout() == true after a fixed time limit;
	// see SetDeadline and SetWriteDeadline.
	// On device connections, write timeouts are rare.
	//
	// If len(b) is larger than the device's MTU,
	// WriteTo will not write the packet, and will
	// return an MTU error (see IsMTU).
	WriteTo(b []byte, hdr IPHeader, dst IP) (n int, err error)
	Deadliner
}

// An IPv4Device is a Device with IPv4-specific methods.
type IPv4Device interface {
	Device

	// IPv4 returns the device's IPv4 address and network mask
	// if they have been set.
	IPv4() (ok bool, addr, netmask IPv4)
	// SetIPv4 sets the device's IPv4 address and network mask,
	// returning any error encountered. SetIPv4 can only be
	// called when the device is down.
	//
	// Calling SetIPv4 with the zero value for addr unsets
	// the IPv4 address.
	SetIPv4(addr, netmask IPv4) error

	// ReadFromIPv4 is like Device's ReadFrom,
	// but for IPv4 only.
	ReadFromIPv4(b []byte) (n int, hdr *IPv4Header, err error)
	// WriteToIPv4 is like Device's WriteTo,
	// but for IPv4 only.
	WriteToIPv4(b []byte, hdr *IPv4Header, dst IPv4) (n int, err error)
}

// An IPv6Device is a Device with IPv6-specific methods.
type IPv6Device interface {
	Device

	// IPv6 returns the device's IPv6 address and network mask
	// if they have been set.
	IPv6() (ok bool, addr, netmask IPv6)
	// SetIPv6 sets the device's IPv6 address and network mask,
	// returning any error encountered. SetIPv6 can only be
	// called when the device is down.
	//
	// Calling SetIPv6 with the zero value for addr unsets
	// the IPv6 address.
	SetIPv6(addr, netmask IPv6) error

	// ReadFromIPv6 is like Device's ReadFrom,
	// but for IPv6 only.
	ReadFromIPv6(b []byte) (n int, hdr *IPv6Header, err error)
	// WriteToIPv6 is like Device's WriteTo,
	// but for IPv6 only.
	WriteToIPv6(b []byte, hdr *IPv6Header, dst IPv6) (n int, err error)
}

//
// // A DeviceConn is a handle to reading IP packets from and writing IP packets to
// // a Device.
// //
// // DeviceConns are safe for concurrent access.
// type DeviceConn interface {
// 	// ReadFrom reads a packet from the device,
// 	// copying the payload into b. It returns the
// 	// number of bytes copied into b and the return
// 	// address and protocol that were on the packet.
// 	// ReadFrom can be made to time out and return
// 	// an error with Timeout() == true after a fixed
// 	// time limit; see SetDeadline and SetReadDeadline.
// 	//
// 	// If a packet larger than len(b) is received,
// 	// n will be len(b), and err will be io.EOF
// 	ReadFrom(b []byte) (n int, hdr IPHeader, err error)
//
// 	// WriteTo writes a packet to the device with
// 	// the specified destination address.
// 	// WriteTo can be made to time out and return
// 	// an error with Timeout() == true after a fixed time limit;
// 	// see SetDeadline and SetWriteDeadline.
// 	// On device connections, write timeouts are rare.
// 	//
// 	// If len(b) is larger than the device's MTU,
// 	// WriteTo will not write the packet, and will
// 	// return an MTU error (see IsMTU).
// 	WriteTo(b []byte, hdr IPHeader, dst IP) error
//
// 	Deadliner
// }
//
// // IPv4DeviceConn is like DeviceConn, but for IPv4.
// type IPv4DeviceConn interface {
// 	// ReadFrom is like DeviceConn's ReadFrom,
// 	// but for IPv4 only.
// 	ReadFrom(b []byte) (n int, hdr IPv4Header, err error)
// 	// WriteTo is like DeviceConn's WriteTo,
// 	// but for IPv4 only.
// 	WriteTo(b []byte, hdr IPv4Header, dst IP) error
// 	Deadliner
// }
//
// // IPv6DeviceConn is like DeviceConn, but for IPv6.
// type IPv6DeviceConn interface {
// 	// ReadFrom is like DeviceConn's ReadFrom,
// 	// but for IPv6 only.
// 	ReadFrom(b []byte) (n int, hdr IPv6Header, err error)
// 	// WriteTo is like DeviceConn's WriteTo,
// 	// but for IPv6 only.
// 	WriteTo(b []byte, hdr IPv6Header, dst IP) error
// 	Deadliner
// }

// ReadDeadliner is the interface that wraps the SetReadDeadline method.
type ReadDeadliner interface {
	// SetReadDeadline sets the deadline for future read-related calls
	// (Read, ReadFrom, etc). If the deadline is reached, these calls
	// will fail with a timeout (see IsTimeout) instead of blocking.
	// A zero value for t means read calls will not time out.
	SetReadDeadline(t time.Time) error
}

// WriteDeadliner is the interface that wraps the SetWriteDeadline method.
type WriteDeadliner interface {
	// SetWriteDeadline sets the deadline for future write-related calls
	// (Write, WriteTo, etc). If the deadline is reached, these calls
	// will fail with a timeout (see IsTimeout) instead of blocking.
	// A zero value for t means write calls will not time out.
	SetWriteDeadline(t time.Time) error
}

// Deadliner is the type that wraps all three deadline-related methods.
type Deadliner interface {
	ReadDeadliner
	WriteDeadliner
	SetDeadline(t time.Time) error // Call SetReadDeadline(t) and SetWriteDeadline(t)
}

// TODO(joshlf): Maybe we don't need the Protocol type?

// A Protocol represents a protocol implemented on top of a particular
// network layer.
type Protocol interface {
	String() string
}

type mtuErr string

func (m mtuErr) Error() string { return string(m) }

// IsMTU returns true if err is an MTU-related error
// created by this package.
func IsMTU(err error) bool {
	_, ok := errors.Cause(err).(mtuErr)
	return ok
}

type timeout string

func (t timeout) Error() string { return string(t) }
func (t timeout) Timeout() bool { return true }

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
