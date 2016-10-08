package net

import "sync"

type ipv6Host struct {
	table     ipv6RoutingTable
	devices   map[IPv6Device]bool
	callbacks [256]func(b []byte, src, dst IPv6)
	forward   bool

	mu sync.RWMutex
}

func NewIPv6Host() IPv6Host {
	return &ipv6Host{devices: make(map[IPv6Device]bool)}
}

// AddDevice adds dev, allowing host to send and receive IP packets
// over the device. Afer calling AddDevice, the caller must not interact
// with the device, except for through host, or until a subsequent
// call to RemoveDevice.  If dev has already been registered, AddDevice
// is a no-op.
func (host *ipv6Host) AddIPv6Device(dev IPv6Device) {
	host.mu.Lock()
	defer host.mu.Unlock()
	dev.RegisterIPv6Callback(func(b []byte) { host.callback(dev, b) })
	host.devices[dev] = true
}

// RemoveDevice removes dev from the host. After calling RemoveDevice,
// the caller may safely interact with the device directly. If no
// such device is currently registered, RemoveDevice is a no-op.
func (host *ipv6Host) RemoveIPv6Device(dev IPv6Device) {
	host.mu.Lock()
	defer host.mu.Unlock()
	if !host.devices[dev] {
		return
	}
	dev.RegisterIPv6Callback(nil)
	delete(host.devices, dev)
}

func (host *ipv6Host) AddIPv6Route(subnet IPv6Subnet, nexthop IPv6) {
	host.mu.Lock()
	host.table.AddRoute(subnet, nexthop)
	host.mu.Unlock()
}

func (host *ipv6Host) AddIPv6DeviceRoute(subnet IPv6Subnet, dev IPv6Device) {
	host.mu.Lock()
	host.table.AddDeviceRoute(subnet, dev)
	host.mu.Unlock()
}

func (host *ipv6Host) IPv6Routes() []IPv6Route {
	host.mu.RLock()
	routes := host.table.Routes()
	host.mu.RUnlock()
	return routes
}

func (host *ipv6Host) IPv6DeviceRoutes() []IPv6DeviceRoute {
	host.mu.RLock()
	routes := host.table.DeviceRoutes()
	host.mu.RUnlock()
	return routes
}

// SetForwarding turns forwarding on or off for host. If forwarding is on,
// received IP packets which are not destined for this host will be forwarded
// to the appropriate next hop if possible.
func (host *ipv6Host) SetForwarding(on bool) {
	host.mu.Lock()
	host.forward = on
	host.mu.Unlock()
}

// Forwarding returns whether or not forwarding is turned on for host.
func (host *ipv6Host) Forwarding() bool {
	host.mu.RLock()
	on := host.forward
	host.mu.RUnlock()
	return on
}

// RegisterCallback registers f to be called whenever an IP packet of the given
// protocol is received. It overwrites any previously-registered callbacks.
// If f is nil, any previously-registered callbacks are cleared.
func (host *ipv6Host) RegisterIPv6Callback(f func(b []byte, src, dst IPv6), proto IPProtocol) {
	host.mu.Lock()
	host.callbacks[int(proto)] = f
	host.mu.Unlock()
}

func (host *ipv6Host) WriteToIPv6(b []byte, addr IPv6, proto IPProtocol) (n int, err error) {
	return host.WriteToTTLIPv6(b, addr, proto, defaultTTL)
}

func (host *ipv6Host) WriteToTTLIPv6(b []byte, addr IPv6, proto IPProtocol, hops uint8) (n int, err error) {
	panic("unimplemented")
}

func (host *ipv6Host) callback(dev IPv6Device, b []byte) {}
