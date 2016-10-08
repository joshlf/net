package net

import (
	"testing"
)

func TestIPv4Packet(t *testing.T) {
	src, _ := ParseIPv4("1.2.3.4")
	dst, _ := ParseIPv4("1.2.3.5")
	v4pkt := ipv4Header{
		version:  4,
		IHL:      5,
		DSCP:     0,
		ECN:      0,
		len:      400,
		id:       20,
		flags:    1,
		fragOff:  60,
		TTL:      15,
		proto:    132,
		checksum: 0,
		src:      src,
		dst:      dst,
	}
	var v4read ipv4Header

	buf := make([]byte, 20)
	writeIPv4Header(&v4pkt, buf)
	readIPv4Header(&v4read, buf)

	if v4pkt != v4read {
		t.Error("Parsed IPv4 packet isn't equivalent to input")
	}
}

func TestIPv6Packet(t *testing.T) {
	src, _ := ParseIPv6("fd00::1")
	dst, _ := ParseIPv6("fe80::b449:e9ff:fe84:8d8a")
	v6pkt := ipv6Header{
		version:      6,
		trafficClass: 1,
		flowLabel:    2,
		nextHdr:      132,
		hopLimit:     15,
		src:          src,
		dst:          dst,
	}
	var v6read ipv6Header

	buf := make([]byte, 40)
	writeIPv6Header(&v6pkt, buf)
	readIPv6Header(&v6read, buf)

	if v6pkt != v6read {
		t.Error("Parsed IPv6 packet isn't equivalent to input")
	}
}
