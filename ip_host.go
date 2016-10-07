package net

import (
	"github.com/juju/errors"
)

type IPHost struct {
	IPv4 *IPv4Host
	IPv6 *IPv6Host
}

// TODO(joshlf): Add RegisterCallback?

// AddDevice adds dev as a device to host.IPv4, host.IPv6, or both depending
// on which of the IPv4Device and IPv6Device interfaces it implements.
func (host *IPHost) AddDevice(dev Device) {
	if dev4, ok := dev.(IPv4Device); ok {
		host.IPv4.AddDevice(dev4)
	}
	if dev6, ok := dev.(IPv6Device); ok {
		host.IPv6.AddDevice(dev6)
	}
}

// AddDevice removes dev as a device to host.IPv4, host.IPv6, or both depending
// on which of the IPv4Device and IPv6Device interfaces it implements.
func (host *IPHost) RemoveDevice(dev Device) {
	if dev4, ok := dev.(IPv4Device); ok {
		host.IPv4.RemoveDevice(dev4)
	}
	if dev6, ok := dev.(IPv6Device); ok {
		host.IPv6.RemoveDevice(dev6)
	}
}

// AddRoute adds the given route to host.IPv4 or host.IPv6 as appropriate.
func (host *IPHost) AddRoute(subnet IPSubnet, nexthop IP) error {
	if subnet.IPVersion() != nexthop.IPVersion() {
		return errors.New("add route: mixed IP subnet and next hop versions")
	}
	switch subnet.IPVersion() {
	case 4:
		host.IPv4.AddRoute(subnet.(IPv4Subnet), nexthop.(IPv4))
	case 6:
		host.IPv6.AddRoute(subnet.(IPv6Subnet), nexthop.(IPv6))
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
		host.IPv4.AddDeviceRoute(subnet, dev4)
	case IPv6Subnet:
		dev6, ok := dev.(IPv6Device)
		if !ok {
			errors.New("add device route: IPv6 subnet with non-IPv6-enabled device")
		}
		host.IPv6.AddDeviceRoute(subnet, dev6)
	}
	return nil
}

// WriteTo writes to the appropriate host depending on the IP version of addr.
func (host *IPHost) WriteTo(b []byte, addr IP, proto IPProtocol) (n int, err error) {
	switch addr := addr.(type) {
	case IPv4:
		return host.IPv4.WriteTo(b, addr, proto)
	case IPv6:
		return host.IPv6.WriteTo(b, addr, proto)
	default:
		panic("unreachable")
	}
}

// WriteToTTL writes to the appropriate host depending on the IP version of addr.
func (host *IPHost) WriteToTTL(b []byte, addr IP, proto IPProtocol, ttl uint8) (n int, err error) {
	switch addr := addr.(type) {
	case IPv4:
		return host.IPv4.WriteToTTL(b, addr, proto, ttl)
	case IPv6:
		return host.IPv6.WriteToTTL(b, addr, proto, ttl)
	default:
		panic("unreachable")
	}
}
