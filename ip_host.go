package net

import (
	"github.com/juju/errors"
)

type IPv4Host interface {
	AddIPv4Device(dev IPv4Device)
	RemoveIPv4Device(dev IPv4Device)
	RegisterIPv4Callback(f func(b []byte, src, dst IPv4), proto IPProtocol)
	AddIPv4Route(subnet IPv4Subnet, nexthop IPv4)
	AddIPv4DeviceRoute(subnet IPv4Subnet, dev IPv4Device)
	IPv4Routes() []IPv4Route
	IPv4DeviceRoutes() []IPv4DeviceRoute
	SetForwarding(on bool)
	Forwarding() bool
	WriteToIPv4(b []byte, addr IPv4, proto IPProtocol) (n int, err error)
	WriteToTTLIPv4(b []byte, addr IPv4, proto IPProtocol, ttl uint8) (n int, err error)
}

type IPv6Host interface {
	AddIPv6Device(dev IPv6Device)
	RemoveIPv6Device(dev IPv6Device)
	RegisterIPv6Callback(f func(b []byte, src, dst IPv6), proto IPProtocol)
	AddIPv6Route(subnet IPv6Subnet, nexthop IPv6)
	AddIPv6DeviceRoute(subnet IPv6Subnet, dev IPv6Device)
	IPv6Routes() []IPv6Route
	IPv6DeviceRoutes() []IPv6DeviceRoute
	SetForwarding(on bool)
	Forwarding() bool
	WriteToIPv6(b []byte, addr IPv6, proto IPProtocol) (n int, err error)
	WriteToTTLIPv6(b []byte, addr IPv6, proto IPProtocol, ttl uint8) (n int, err error)
}

type IPHost struct {
	IPv4Host
	IPv6Host
}

// AddDevice adds dev as a device to host.IPv4, host.IPv6, or both depending
// on which of the IPv4Device and IPv6Device interfaces it implements.
func (host *IPHost) AddDevice(dev Device) {
	if dev4, ok := dev.(IPv4Device); ok {
		host.IPv4Host.AddIPv4Device(dev4)
	}
	if dev6, ok := dev.(IPv6Device); ok {
		host.IPv6Host.AddIPv6Device(dev6)
	}
}

// AddDevice removes dev as a device to host.IPv4, host.IPv6, or both depending
// on which of the IPv4Device and IPv6Device interfaces it implements.
func (host *IPHost) RemoveDevice(dev Device) {
	if dev4, ok := dev.(IPv4Device); ok {
		host.IPv4Host.RemoveIPv4Device(dev4)
	}
	if dev6, ok := dev.(IPv6Device); ok {
		host.IPv6Host.RemoveIPv6Device(dev6)
	}
}

// RegisterCallback registers f as the callback for both host.IPv4Host and
// host.IPv6Host.
func (host *IPHost) RegisterCallback(f func(b []byte, src, dst IP), proto IPProtocol) {
	host.IPv4Host.RegisterIPv4Callback(func(b []byte, src, dst IPv4) { f(b, src, dst) }, proto)
	host.IPv6Host.RegisterIPv6Callback(func(b []byte, src, dst IPv6) { f(b, src, dst) }, proto)
}

// AddRoute adds the given route to host.IPv4 or host.IPv6 as appropriate.
func (host *IPHost) AddRoute(subnet IPSubnet, nexthop IP) error {
	if subnet.IPVersion() != nexthop.IPVersion() {
		return errors.New("add route: mixed IP subnet and next hop versions")
	}
	switch subnet.IPVersion() {
	case 4:
		host.IPv4Host.AddIPv4Route(subnet.(IPv4Subnet), nexthop.(IPv4))
	case 6:
		host.IPv6Host.AddIPv6Route(subnet.(IPv6Subnet), nexthop.(IPv6))
	}
	return nil
}

// AddDeviceRoute adds the given device route to host.IPv4 or host.IPv6 as
// appropriate.
func (host *IPHost) AddDeviceRoute(subnet IPSubnet, dev Device) error {
	switch subnet := subnet.(type) {
	case IPv4Subnet:
		dev4, ok := dev.(IPv4Device)
		if !ok {
			errors.New("add device route: IPv4 subnet with non-IPv4-enabled device")
		}
		host.IPv4Host.AddIPv4DeviceRoute(subnet, dev4)
	case IPv6Subnet:
		dev6, ok := dev.(IPv6Device)
		if !ok {
			errors.New("add device route: IPv6 subnet with non-IPv6-enabled device")
		}
		host.IPv6Host.AddIPv6DeviceRoute(subnet, dev6)
	}
	return nil
}

// SetForwarding sets forwarding on or off on host.IPv4Host and host.IPv6Host.
func (host *IPHost) SetForwarding(on bool) {
	host.IPv4Host.SetForwarding(on)
	host.IPv6Host.SetForwarding(on)
}

// WriteTo writes to the appropriate host depending on the IP version of addr.
func (host *IPHost) WriteTo(b []byte, addr IP, proto IPProtocol) (n int, err error) {
	switch addr := addr.(type) {
	case IPv4:
		return host.IPv4Host.WriteToIPv4(b, addr, proto)
	case IPv6:
		return host.IPv6Host.WriteToIPv6(b, addr, proto)
	default:
		panic("unreachable")
	}
}

// WriteToTTL writes to the appropriate host depending on the IP version of addr.
func (host *IPHost) WriteToTTL(b []byte, addr IP, proto IPProtocol, ttl uint8) (n int, err error) {
	switch addr := addr.(type) {
	case IPv4:
		return host.IPv4Host.WriteToTTLIPv4(b, addr, proto, ttl)
	case IPv6:
		return host.IPv6Host.WriteToTTLIPv6(b, addr, proto, ttl)
	default:
		panic("unreachable")
	}
}
