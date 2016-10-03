package net

import (
	"encoding/binary"
	"math"
	"sync"

	"github.com/juju/errors"
)

const (
	defaultIPv4TTL = 32
)

// IPv4Protocol represents the protocol field of an IPv4 packet.
type IPv4Protocol uint8

const (
	IPv4ProtocolTCP IPv4Protocol = 6
)

type IPv4Host struct {
	table     routingTable
	devices   []namedDevice
	callbacks [256]func(b []byte, src, dst IPv4)

	mu sync.RWMutex
}

func (host *IPv4Host) callback(dev *namedDevice, b []byte) {
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

	host.mu.RLock()
	defer host.mu.RUnlock()
	var us bool
	for _, dev := range host.devices {
		ok, addr, _ := dev.dev.(IPv4Device).IPv4()
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
	} else {
		// forward
		if hdr.TTL < 2 {
			// TTL is or would become 0 after decrement
			// See "TTL" section, https://tools.ietf.org/html/rfc791#page-14
			return
		}
		hdr.TTL--
		setTTL(b, hdr.TTL)
		nexthop, dev := host.table.Lookup(hdr.dst)
		if nexthop == nil {
			// TODO(joshlf): ICMP reply
			return
		}
		dev.dev.(IPv4Device).WriteToIPv4(b, nexthop.(IPv4))
		// TODO(joshlf): Log error
	}
}

func (host *IPv4Host) WriteTo(b []byte, addr IPv4, proto IPv4Protocol) error {
	host.mu.RLock()
	defer host.mu.RUnlock()
	nexthop, dev := host.table.Lookup(addr)
	if nexthop == nil {
		return noRoute{addr.String()}
	}
	ok, devaddr, _ := dev.dev.(IPv4Device).IPv4()
	if !ok {
		return errors.New("device has no IPv4 address")
	}

	if len(b) > math.MaxUint16-20 {
		return mtuErr("IPv4 payload exceeds maximum size")
	}
	var hdr ipv4Header
	hdr.version = 4
	hdr.IHL = 5
	hdr.len = 20 + uint16(len(b))
	hdr.TTL = defaultIPv4TTL
	hdr.proto = uint8(proto)
	hdr.src = devaddr
	hdr.dst = addr

	buf := make([]byte, int(hdr.len))
	writeIPv4Header(&hdr, buf)
	copy(buf[20:], b)

	_, err := dev.dev.(IPv4Device).WriteToIPv4(buf, nexthop.(IPv4))
	return errors.Annotate(err, "write IPv4 packet")
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
	proto    uint8
	checksum uint16
	src, dst IPv4
}

// assumes b is long enough
func writeIPv4Header(hdr *ipv4Header, buf []byte) {
	getBytes(&buf, 1)[0] = (hdr.version << 4) | hdr.IHL
	getBytes(&buf, 1)[0] = (hdr.DSCP << 2) | hdr.ECN
	binary.BigEndian.PutUint16(getBytes(&buf, 2), hdr.len)
	binary.BigEndian.PutUint16(getBytes(&buf, 2), hdr.id)
	getBytes(&buf, 1)[0] = (hdr.flags << 5) | uint8(hdr.fragOff>>8)
	getBytes(&buf, 1)[0] = byte(hdr.fragOff)
	getBytes(&buf, 1)[0] = hdr.TTL
	getBytes(&buf, 1)[0] = hdr.proto
	binary.BigEndian.PutUint16(getBytes(&buf, 2), hdr.checksum)
	copy(getBytes(&buf, 4), hdr.src[:])
	copy(getBytes(&buf, 4), hdr.dst[:])
}

// assumes b is long enough
func readIPv4Header(hdr *ipv4Header, buf []byte) {
	b := getByte(&buf)
	hdr.version = b >> 4
	hdr.IHL = b & 0xF
	b = getByte(&buf)
	hdr.DSCP = b >> 2
	hdr.ECN = b & 3
	hdr.len = binary.BigEndian.Uint16(getBytes(&buf, 2))
	hdr.id = binary.BigEndian.Uint16(getBytes(&buf, 2))
	b = getByte(&buf)
	hdr.flags = b >> 5
	hdr.fragOff = (uint16(b&0x1F) << 8) | uint16(getByte(&buf)) // 0x1F is 5 1s bits
	hdr.TTL = getByte(&buf)
	hdr.proto = getByte(&buf)
	hdr.checksum = binary.BigEndian.Uint16(getBytes(&buf, 2))
	copy(hdr.src[:], getBytes(&buf, 4))
	copy(hdr.dst[:], getBytes(&buf, 4))
}

// setTTL sets the TTL in the IP header encoded in b
// without having to expensively rewrite the entire
// header using writeIPv4Header
func setTTL(b []byte, ttl uint8) {
	b[8] = ttl
}
