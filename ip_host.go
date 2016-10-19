package net

import (
	"github.com/joshlf/net/internal/errors"
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

	// SetTTL sets the TTL for all outoing packets. If ttl is 0, a default TTL
	// will be used.
	SetTTL(ttl uint8)

	// GetConfigCopyIPv4 returns an IPv4Host which is simply a wrapper around
	// the original host, but which allows setting configuration values
	// without setting those values on the original host. In particular, all
	// methods except for SetTTL operate directly on the original host.
	GetConfigCopyIPv4() IPv4Host
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

	// SetTTL sets the TTL for all outoing packets. If ttl is 0, a default TTL
	// will be used.
	SetTTL(ttl uint8)

	// GetConfigCopyIPv6 returns an IPv6Host which is simply a wrapper around
	// the original host, but which allows setting configuration values
	// without setting those values on the original host. In particular, all
	// methods except for SetTTL operate directly on the original host.
	GetConfigCopyIPv6() IPv6Host
}

type IPHost struct {
	IPv4Host
	IPv6Host
}

func (host *IPHost) AddDevice(dev Device) {
	if dev4, ok := dev.(IPv4Device); ok {
		host.IPv4Host.AddIPv4Device(dev4)
	}
	if dev6, ok := dev.(IPv6Device); ok {
		host.IPv6Host.AddIPv6Device(dev6)
	}
}

func (host *IPHost) RemoveDevice(dev Device) {
	if dev4, ok := dev.(IPv4Device); ok {
		host.IPv4Host.RemoveIPv4Device(dev4)
	}
	if dev6, ok := dev.(IPv6Device); ok {
		host.IPv6Host.RemoveIPv6Device(dev6)
	}
}

func (host *IPHost) RegisterCallback(f func(b []byte, src, dst IP), proto IPProtocol) {
	host.IPv4Host.RegisterIPv4Callback(func(b []byte, src, dst IPv4) { f(b, src, dst) }, proto)
	host.IPv6Host.RegisterIPv6Callback(func(b []byte, src, dst IPv6) { f(b, src, dst) }, proto)
}

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

func (host *IPHost) SetForwarding(on bool) {
	host.IPv4Host.SetForwarding(on)
	host.IPv6Host.SetForwarding(on)
}

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

func (host *IPHost) SetTTL(ttl uint8) {
	host.IPv4Host.SetTTL(ttl)
	host.IPv6Host.SetTTL(ttl)
}

func (host *IPHost) GetConfigCopy() *IPHost {
	return &IPHost{
		IPv4Host: host.IPv4Host.GetConfigCopyIPv4(),
		IPv6Host: host.IPv6Host.GetConfigCopyIPv6(),
	}
}
