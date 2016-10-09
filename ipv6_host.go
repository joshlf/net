package net

import (
	"math"
	"sync"

	"github.com/joshlf/net/internal/parse"
	"github.com/juju/errors"
)

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
	host.mu.RLock()
	defer host.mu.RUnlock()
	nexthop, dev, ok := host.table.Lookup(addr)
	if !ok {
		return 0, errors.Annotate(noRoute{addr.String()}, "write IPv6 packet")
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
