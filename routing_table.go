package net

import "sync"

// TODO(joshlf): Eventually specialize separate IPv4 and IPv6 versions
// for performance (get rid of interface and type assertion overhead)

// TODO(joshlf): What about subnet refinement? We assume each IP is only in
// a single subnet, but we also allow multiple subnets that are "unequal"
// because one is a subset of the other.

type IPv4Route struct {
	Subnet  IPv4Subnet
	Nexthop IPv4
}

type IPv4DeviceRoute struct {
	Subnet IPv4Subnet
	Device IPv4Device
}

type IPv6Route struct {
	Subnet  IPv6Subnet
	Nexthop IPv6
}

type IPv6DeviceRoute struct {
	Subnet IPv6Subnet
	Device IPv6Device
}

type ipv4RoutingTable struct {
	rt routingTable
}

func (rt *ipv4RoutingTable) AddRoute(subnet IPv4Subnet, nexthop IPv4) {
	rt.rt.AddRoute(subnet, nexthop)
}

func (rt *ipv4RoutingTable) DeleteRoute(subnet IPv4Subnet) {
	rt.rt.DeleteRoute(subnet)
}

func (rt *ipv4RoutingTable) AddDeviceRoute(subnet IPv4Subnet, dev IPv4Device) {
	rt.rt.AddDeviceRoute(subnet, dev)
}

func (rt *ipv4RoutingTable) DeleteDeviceRoute(subnet IPv4Subnet) {
	rt.rt.DeleteDeviceRoute(subnet)
}

func (rt *ipv4RoutingTable) Lookup(addr IPv4) (nexthop IPv4, dev IPv4Device, ok bool) {
	n, d := rt.rt.Lookup(addr)
	if n == nil {
		return IPv4{}, nil, false
	}
	return n.(IPv4), d.(IPv4Device), true
}

func (rt *ipv4RoutingTable) Routes() []IPv4Route {
	var routes []IPv4Route
	for _, route := range rt.rt.Routes() {
		routes = append(routes, IPv4Route{
			Subnet:  route.subnet.(IPv4Subnet),
			Nexthop: route.nexthop.(IPv4),
		})
	}
	return routes
}

func (rt *ipv4RoutingTable) DeviceRoutes() []IPv4DeviceRoute {
	var routes []IPv4DeviceRoute
	for _, route := range rt.rt.DeviceRoutes() {
		routes = append(routes, IPv4DeviceRoute{
			Subnet: route.subnet.(IPv4Subnet),
			Device: route.device.(IPv4Device),
		})
	}
	return routes
}

type ipv6RoutingTable struct {
	rt routingTable
}

func (rt *ipv6RoutingTable) AddRoute(subnet IPv6Subnet, nexthop IPv6) {
	rt.rt.AddRoute(subnet, nexthop)
}

func (rt *ipv6RoutingTable) DeleteRoute(subnet IPv6Subnet) {
	rt.rt.DeleteRoute(subnet)
}

func (rt *ipv6RoutingTable) AddDeviceRoute(subnet IPv6Subnet, dev IPv6Device) {
	rt.rt.AddDeviceRoute(subnet, dev)
}

func (rt *ipv6RoutingTable) DeleteDeviceRoute(subnet IPv6Subnet) {
	rt.rt.DeleteDeviceRoute(subnet)
}

func (rt *ipv6RoutingTable) Lookup(addr IPv4) (nexthop IPv6, dev IPv6Device, ok bool) {
	n, d := rt.rt.Lookup(addr)
	if n == nil {
		return IPv6{}, nil, false
	}
	return n.(IPv6), d.(IPv6Device), true
}

func (rt *ipv6RoutingTable) Routes() []IPv6Route {
	var routes []IPv6Route
	for _, route := range rt.rt.Routes() {
		routes = append(routes, IPv6Route{
			Subnet:  route.subnet.(IPv6Subnet),
			Nexthop: route.nexthop.(IPv6),
		})
	}
	return routes
}

func (rt *ipv6RoutingTable) DeviceRoutes() []IPv6DeviceRoute {
	var routes []IPv6DeviceRoute
	for _, route := range rt.rt.DeviceRoutes() {
		routes = append(routes, IPv6DeviceRoute{
			Subnet: route.subnet.(IPv6Subnet),
			Device: route.device.(IPv6Device),
		})
	}
	return routes
}

// routingTable is generic so that it can work with either IPv4 or IPv6,
// but any given instance should only be used with one of the two versions,
// and the implementation assumes this is happening.
type routingTable struct {
	routes       []routingTableIPRoute
	deviceRoutes []routingTableDeviceRoute
	mu           sync.RWMutex
}

type routingTableIPRoute struct {
	subnet  IPSubnet
	nexthop IP
}

type routingTableDeviceRoute struct {
	subnet IPSubnet
	device Device
}

func (r *routingTable) AddRoute(subnet IPSubnet, nexthop IP) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i, rr := range r.routes {
		if SubnetEqual(rr.subnet, subnet) {
			r.routes[i].nexthop = nexthop
			return
		}
	}
	r.routes = append(r.routes, routingTableIPRoute{subnet: subnet, nexthop: nexthop})
}

func (r *routingTable) DeleteRoute(subnet IPSubnet) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i, rr := range r.routes {
		if SubnetEqual(rr.subnet, subnet) {
			copy(r.routes[i:], r.routes[i+1:])
			r.routes = r.routes[:len(r.routes)-1]
			return
		}
	}
}

func (r *routingTable) AddDeviceRoute(subnet IPSubnet, dev Device) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i, rr := range r.deviceRoutes {
		if SubnetEqual(rr.subnet, subnet) {
			r.deviceRoutes[i].device = dev
			return
		}
	}
	r.deviceRoutes = append(r.deviceRoutes, routingTableDeviceRoute{
		subnet: subnet,
		device: dev,
	})
}

func (r *routingTable) DeleteDeviceRoute(subnet IPSubnet) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i, rr := range r.deviceRoutes {
		if SubnetEqual(rr.subnet, subnet) {
			copy(r.deviceRoutes[i:], r.deviceRoutes[i+1:])
			r.deviceRoutes = r.deviceRoutes[:len(r.deviceRoutes)-1]
			return
		}
	}
}

func (r *routingTable) Lookup(addr IP) (nexthop IP, dev Device) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	dev = r.lookupDeviceRoute(addr)
	if dev != nil {
		return addr, dev
	}
	for _, rr := range r.routes {
		if SubnetHas(rr.subnet, addr) {
			dev = r.lookupDeviceRoute(rr.nexthop)
			if dev == nil {
				return nil, nil
			}
			return rr.nexthop, dev
		}
	}
	return nil, nil
}

func (r *routingTable) lookupDeviceRoute(addr IP) Device {
	for _, r := range r.deviceRoutes {
		if SubnetHas(r.subnet, addr) {
			return r.device
		}
	}
	return nil
}

func (r *routingTable) Routes() []routingTableIPRoute {
	r.mu.RLock()
	routes := append([]routingTableIPRoute(nil), r.routes...)
	r.mu.RUnlock()
	return routes
}

func (r *routingTable) DeviceRoutes() []routingTableDeviceRoute {
	r.mu.RLock()
	routes := append([]routingTableDeviceRoute(nil), r.deviceRoutes...)
	r.mu.RUnlock()
	return routes
}
