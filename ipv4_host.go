package net

import (
	"math"
	"sync"

	"github.com/joshlf/net/internal/errors"
	"github.com/joshlf/net/internal/parse"
)

const (
	// default IPv4 TTL or IPv6 hops
	defaultTTL = 32
)

// IPProtocol represents the protocol field of an IPv4 packet and the
// next header field of an IPv6 packet.
type IPProtocol uint8

const (
	IPProtocolTCP IPProtocol = 6
)

type ipv4Host struct {
	table     ipv4RoutingTable
	devices   map[IPv4Device]bool // make sure to check if nil before modifying
	callbacks [256]func(b []byte, src, dst IPv4)
	forward   bool

	mu sync.RWMutex
}

func NewIPv4Host() IPv4Host {
	return &ipv4Host{devices: make(map[IPv4Device]bool)}
}

// AddDevice adds dev, allowing host to send and receive IP packets
// over the device. Afer calling AddDevice, the caller must not interact
// with the device, except for through host, or until a subsequent
// call to RemoveDevice.  If dev has already been registered, AddDevice
// is a no-op.
func (host *ipv4Host) AddIPv4Device(dev IPv4Device) {
	host.mu.Lock()
	defer host.mu.Unlock()
	dev.RegisterIPv4Callback(func(b []byte) { host.callback(dev, b) })
	host.devices[dev] = true
}

// RemoveDevice removes dev from the host. After calling RemoveDevice,
// the caller may safely interact with the device directly. If no
// such device is currently registered, RemoveDevice is a no-op.
func (host *ipv4Host) RemoveIPv4Device(dev IPv4Device) {
	host.mu.Lock()
	defer host.mu.Unlock()
	if !host.devices[dev] {
		return
	}
	dev.RegisterIPv4Callback(nil)
	delete(host.devices, dev)
}

func (host *ipv4Host) AddIPv4Route(subnet IPv4Subnet, nexthop IPv4) {
	host.mu.Lock()
	host.table.AddRoute(subnet, nexthop)
	host.mu.Unlock()
}

func (host *ipv4Host) AddIPv4DeviceRoute(subnet IPv4Subnet, dev IPv4Device) {
	host.mu.Lock()
	host.table.AddDeviceRoute(subnet, dev)
	host.mu.Unlock()
}

func (host *ipv4Host) IPv4Routes() []IPv4Route {
	host.mu.RLock()
	routes := host.table.Routes()
	host.mu.RUnlock()
	return routes
}

func (host *ipv4Host) IPv4DeviceRoutes() []IPv4DeviceRoute {
	host.mu.RLock()
	routes := host.table.DeviceRoutes()
	host.mu.RUnlock()
	return routes
}

// SetForwarding turns forwarding on or off for host. If forwarding is on,
// received IP packets which are not destined for this host will be forwarded
// to the appropriate next hop if possible.
func (host *ipv4Host) SetForwarding(on bool) {
	host.mu.Lock()
	host.forward = on
	host.mu.Unlock()
}

// Forwarding returns whether or not forwarding is turned on for host.
func (host *ipv4Host) Forwarding() bool {
	host.mu.RLock()
	on := host.forward
	host.mu.RUnlock()
	return on
}

// RegisterCallback registers f to be called whenever an IP packet of the given
// protocol is received. It overwrites any previously-registered callbacks.
// If f is nil, any previously-registered callbacks are cleared.
func (host *ipv4Host) RegisterIPv4Callback(f func(b []byte, src, dst IPv4), proto IPProtocol) {
	host.mu.Lock()
	host.callbacks[int(proto)] = f
	host.mu.Unlock()
}

func (host *ipv4Host) WriteToIPv4(b []byte, addr IPv4, proto IPProtocol) (n int, err error) {
	return host.WriteToTTLIPv4(b, addr, proto, defaultTTL)
}

func (host *ipv4Host) WriteToTTLIPv4(b []byte, addr IPv4, proto IPProtocol, ttl uint8) (n int, err error) {
	host.mu.RLock()
	defer host.mu.RUnlock()
	nexthop, dev, ok := host.table.Lookup(addr)
	if !ok {
		return 0, errors.Annotate(errors.NewNoRoute(addr.String()), "write IPv4 packet")
	}
	devaddr, _, ok := dev.(IPv4Device).IPv4()
	if !ok {
		return 0, errors.New("device has no IPv4 address")
	}

	if len(b) > math.MaxUint16-20 {
		// MTU errors are only for link-layer payloads
		return 0, errors.New("IPv4 payload exceeds maximum IPv4 packet size")
	}
	var hdr ipv4Header
	hdr.version = 4
	hdr.IHL = 5
	hdr.len = 20 + uint16(len(b))
	hdr.TTL = ttl
	hdr.proto = proto
	hdr.src = devaddr
	hdr.dst = addr

	buf := make([]byte, int(hdr.len))
	writeIPv4Header(&hdr, buf)
	copy(buf[20:], b)

	n, err = dev.WriteToIPv4(buf, nexthop)
	if n < 20 {
		n = 0
	} else {
		n -= 20
	}
	return n, errors.Annotate(err, "write IPv4 packet")
}

func (host *ipv4Host) callback(dev IPv4Device, b []byte) {
	// We accept the device as an argument
	// because we may use it in the future,
	// for example for a NAT server to tell
	// which of multiple private-addressed
	// networks a packet came from.
	if len(b) < 20 {
		return
	}
	var hdr ipv4Header
	readIPv4Header(&hdr, b)
	if int(hdr.len) != len(b) {
		// TODO(joshlf): Log it
		return
	}

	host.mu.RLock()
	defer host.mu.RUnlock()
	var us bool
	for dev := range host.devices {
		addr, _, ok := dev.IPv4()
		if ok && addr == hdr.dst {
			us = true
			break
		}
	}

	if us {
		// deliver
		c := host.callbacks[int(hdr.proto)]
		if c == nil {
			return
		}
		c(b[20:], hdr.src, hdr.dst)
	} else if host.forward {
		// forward
		if hdr.TTL < 2 {
			// TTL is or would become 0 after decrement
			// See "TTL" section, https://tools.ietf.org/html/rfc791#page-14
			return
		}
		hdr.TTL--
		setTTL(b, hdr.TTL)
		nexthop, dev, ok := host.table.Lookup(hdr.dst)
		if !ok {
			// TODO(joshlf): ICMP reply
			return
		}
		dev.WriteToIPv4(b, nexthop)
		// TODO(joshlf): Log error
	}
}

// TODO(joshlf):
//   - support options
//   - compute and validate checksums

type ipv4Header struct {
	version  uint8
	IHL      uint8
	DSCP     uint8
	ECN      uint8
	len      uint16
	id       uint16
	flags    uint8
	fragOff  uint16
	TTL      uint8
	proto    IPProtocol
	checksum uint16
	src, dst IPv4
}

// assumes b is long enough
func writeIPv4Header(hdr *ipv4Header, buf []byte) {
	parse.GetBytes(&buf, 1)[0] = (hdr.version << 4) | hdr.IHL
	parse.GetBytes(&buf, 1)[0] = (hdr.DSCP << 2) | hdr.ECN
	parse.PutUint16(&buf, hdr.len)
	parse.PutUint16(&buf, hdr.id)
	parse.GetBytes(&buf, 1)[0] = (hdr.flags << 5) | uint8(hdr.fragOff>>8)
	parse.GetBytes(&buf, 1)[0] = byte(hdr.fragOff)
	parse.GetBytes(&buf, 1)[0] = hdr.TTL
	parse.GetBytes(&buf, 1)[0] = byte(hdr.proto)
	parse.PutUint16(&buf, hdr.checksum)
	copy(parse.GetBytes(&buf, 4), hdr.src[:])
	copy(parse.GetBytes(&buf, 4), hdr.dst[:])
}

// assumes b is long enough
func readIPv4Header(hdr *ipv4Header, buf []byte) {
	b := parse.GetByte(&buf)
	hdr.version = b >> 4
	hdr.IHL = b & 0xF
	b = parse.GetByte(&buf)
	hdr.DSCP = b >> 2
	hdr.ECN = b & 3
	hdr.len = parse.GetUint16(&buf)
	hdr.id = parse.GetUint16(&buf)
	b = parse.GetByte(&buf)
	hdr.flags = b >> 5
	hdr.fragOff = (uint16(b&0x1F) << 8) | uint16(parse.GetByte(&buf)) // 0x1F is 5 1s bits
	hdr.TTL = parse.GetByte(&buf)
	hdr.proto = IPProtocol(parse.GetByte(&buf))
	hdr.checksum = parse.GetUint16(&buf)
	copy(hdr.src[:], parse.GetBytes(&buf, 4))
	copy(hdr.dst[:], parse.GetBytes(&buf, 4))
}

// setTTL sets the TTL in the IP header encoded in b
// without having to expensively rewrite the entire
// header using writeIPv4Header
func setTTL(b []byte, ttl uint8) {
	b[8] = ttl
}
