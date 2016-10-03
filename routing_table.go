package net

import "sync"

// TODO(joshlf): Eventually specialize separate IPv4 and IPv6 versions
// for performance (get rid of interface and type assertion overhead)

// TODO(joshlf): What about subnet refinement? We assume each IP is only in
// a single subnet, but we also allow multiple subnets that are "unequal"
// because one is a subset of the other.

type namedDevice struct {
	name string
	dev  Device
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
	device *namedDevice
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

func (r *routingTable) AddDeviceRoute(subnet IPSubnet, dev *namedDevice) {
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

func (r *routingTable) Lookup(addr IP) (nexthop IP, dev *namedDevice) {
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

func (r *routingTable) lookupDeviceRoute(addr IP) *namedDevice {
	for _, r := range r.deviceRoutes {
		if SubnetHas(r.subnet, addr) {
			return r.device
		}
	}
	return nil
}
