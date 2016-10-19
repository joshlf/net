package net

import (
	"math"
	"sync"

	"github.com/joshlf/net/internal/errors"
	"github.com/joshlf/net/internal/parse"
)

type ipv6Host struct {
	table     ipv6RoutingTable
	devices   map[IPv6Device]bool
	callbacks [256]func(b []byte, src, dst IPv6)
	forward   bool

	mu sync.RWMutex
}

type ipv6ConfigurationHost struct {
	*ipv6Host
	ttl uint8

	mu sync.RWMutex
}

func (host *ipv6ConfigurationHost) rlock()   { host.mu.RLock(); host.ipv6Host.mu.RLock() }
func (host *ipv6ConfigurationHost) runlock() { host.ipv6Host.mu.RUnlock(); host.mu.RUnlock() }
func (host *ipv6ConfigurationHost) lock()    { host.mu.Lock(); host.ipv6Host.mu.Lock() }
func (host *ipv6ConfigurationHost) unlock()  { host.ipv6Host.mu.Unlock(); host.mu.Unlock() }

func NewIPv6Host() IPv6Host {
	return &ipv6ConfigurationHost{
		ipv6Host: &ipv6Host{devices: make(map[IPv6Device]bool)},
		ttl:      defaultTTL,
	}
}

func (host *ipv6ConfigurationHost) SetTTL(ttl uint8) {
	if ttl == 0 {
		ttl = defaultTTL
	}
	host.mu.Lock()
	host.ttl = ttl
	host.mu.Unlock()
}

func (host *ipv6ConfigurationHost) GetConfigCopyIPv6() IPv6Host {
	host.rlock()
	new := *host
	new.mu = sync.RWMutex{} // sync.RMUtexes can't be safely copied
	host.runlock()
	return &new
}

func (host *ipv6ConfigurationHost) AddIPv6Device(dev IPv6Device) {
	host.lock()
	defer host.unlock()
	dev.RegisterIPv6Callback(func(b []byte) { host.callback(dev, b) })
	host.devices[dev] = true
}

func (host *ipv6ConfigurationHost) RemoveIPv6Device(dev IPv6Device) {
	host.lock()
	defer host.unlock()
	if !host.devices[dev] {
		return
	}
	dev.RegisterIPv6Callback(nil)
	delete(host.devices, dev)
}

func (host *ipv6ConfigurationHost) AddIPv6Route(subnet IPv6Subnet, nexthop IPv6) {
	host.lock()
	host.table.AddRoute(subnet, nexthop)
	host.unlock()
}

func (host *ipv6ConfigurationHost) AddIPv6DeviceRoute(subnet IPv6Subnet, dev IPv6Device) {
	host.lock()
	host.table.AddDeviceRoute(subnet, dev)
	host.unlock()
}

func (host *ipv6ConfigurationHost) IPv6Routes() []IPv6Route {
	host.rlock()
	routes := host.table.Routes()
	host.runlock()
	return routes
}

func (host *ipv6ConfigurationHost) IPv6DeviceRoutes() []IPv6DeviceRoute {
	host.rlock()
	routes := host.table.DeviceRoutes()
	host.runlock()
	return routes
}

func (host *ipv6ConfigurationHost) SetForwarding(on bool) {
	host.lock()
	host.forward = on
	host.unlock()
}

func (host *ipv6ConfigurationHost) Forwarding() bool {
	host.rlock()
	on := host.forward
	host.runlock()
	return on
}

func (host *ipv6ConfigurationHost) RegisterIPv6Callback(f func(b []byte, src, dst IPv6), proto IPProtocol) {
	host.lock()
	host.callbacks[int(proto)] = f
	host.unlock()
}

func (host *ipv6ConfigurationHost) WriteToIPv6(b []byte, addr IPv6, proto IPProtocol) (n int, err error) {
	host.rlock()
	n, err = host.write(b, addr, proto, host.ttl)
	host.runlock()
	return n, err
}

func (host *ipv6Host) write(b []byte, addr IPv6, proto IPProtocol, hops uint8) (n int, err error) {
	host.mu.RLock()
	defer host.mu.RUnlock()
	nexthop, dev, ok := host.table.Lookup(addr)
	if !ok {
		return 0, errors.Annotate(errors.NewNoRoute(addr.String()), "write IPv6 packet")
	}
	devaddr, _, ok := dev.(IPv6Device).IPv6()
	if !ok {
		return 0, errors.New("device has no IPv6 address")
	}

	if len(b) > math.MaxUint16-40 {
		// MTU errors are only for link-layer payloads
		return 0, errors.New("IPv6 payload exceeds maximum IPv6 packet size")
	}

	var hdr ipv6Header
	hdr.version = 6
	hdr.len = 40 + uint16(len(b))
	hdr.nextHdr = proto
	hdr.hopLimit = hops
	hdr.src = devaddr
	hdr.dst = addr

	buf := make([]byte, int(hdr.len))
	writeIPv6Header(&hdr, buf)
	copy(buf[40:], b)

	n, err = dev.WriteToIPv6(buf, nexthop)
	if n < 40 {
		n = 0
	} else {
		n -= 40
	}

	return n, errors.Annotate(err, "write IPv6 packet")
}

type ipv6Header struct {
	version      uint8
	trafficClass uint8
	flowLabel    uint32
	len          uint16
	nextHdr      IPProtocol
	hopLimit     uint8
	src, dst     IPv6
}

func writeIPv6Header(hdr *ipv6Header, buf []byte) {
	parse.GetBytes(&buf, 1)[0] = (hdr.version << 4) | (hdr.trafficClass >> 4)
	parse.GetBytes(&buf, 1)[0] = (hdr.trafficClass << 4) | uint8(hdr.flowLabel>>16)
	parse.PutUint16(&buf, uint16(hdr.flowLabel&0xff))
	parse.PutUint16(&buf, hdr.len)
	parse.GetBytes(&buf, 1)[0] = byte(hdr.nextHdr)
	parse.GetBytes(&buf, 1)[0] = hdr.hopLimit
	copy(parse.GetBytes(&buf, 16), hdr.src[:])
	copy(parse.GetBytes(&buf, 16), hdr.dst[:])
}

func readIPv6Header(hdr *ipv6Header, buf []byte) {
	a := parse.GetByte(&buf)
	hdr.version = a >> 4
	b := parse.GetByte(&buf)
	hdr.trafficClass = (a << 4) | (b >> 4)
	flowBottom := parse.GetUint16(&buf)
	hdr.flowLabel = (uint32(b&0xf) << 16) | uint32(flowBottom)
	hdr.len = parse.GetUint16(&buf)
	hdr.nextHdr = IPProtocol(parse.GetByte(&buf))
	hdr.hopLimit = parse.GetByte(&buf)
	copy(hdr.src[:], parse.GetBytes(&buf, 16))
	copy(hdr.dst[:], parse.GetBytes(&buf, 16))
}

func (host *ipv6Host) callback(dev IPv6Device, b []byte) {
	if len(b) < 40 {
		return
	}
	var hdr ipv6Header
	readIPv6Header(&hdr, b)
	if int(hdr.len) != len(b) {
		// TODO(joshlf): Log it
		return
	}

	host.mu.RLock()
	defer host.mu.RUnlock()
	var us bool
	for dev := range host.devices {
		addr, _, ok := dev.IPv6()
		if ok && addr == hdr.dst {
			us = true
			break
		}
	}

	if us {
		// deliver
		c := host.callbacks[int(hdr.nextHdr)]
		if c == nil {
			return
		}
		c(b[40:], hdr.src, hdr.dst)
	} else if host.forward {
		// forward
		if hdr.hopLimit < 2 {
			// TTL is or would become 0 after decrement
			// See "TTL" section, https://tools.ietf.org/html/rfc791#page-14
			return
		}
		hdr.hopLimit--
		setTTL(b, hdr.hopLimit)
		nexthop, dev, ok := host.table.Lookup(hdr.dst)
		if !ok {
			// XXX: ICMPv6 reply
			return
		}
		dev.WriteToIPv6(b, nexthop)
	}
}
