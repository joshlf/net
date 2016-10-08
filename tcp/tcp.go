package tcp

import (
	"sync"

	"github.com/joshlf/net"
)

// Port represents a TCP port.
type Port uint16

type ipv4FourTuple struct {
	src     net.IPv4
	srcport Port
	dst     net.IPv4
	dstport Port
}

type ipv4Listener struct {
	addr net.IPv4
	port Port
}

// IPv4Host ... the zero value is not a valid IPv4Host
type IPv4Host struct {
	iphost    net.IPv4Host
	listeners map[ipv4Listener]struct{} // TODO(joshlf): what's the value type?
	conns     map[ipv4FourTuple]*conn

	mu sync.RWMutex
}

func NewIPv4Host(iphost net.IPv4Host) (*IPv4Host, error) {
	host := &IPv4Host{iphost: iphost}
	iphost.RegisterIPv4Callback(host.callback, net.IPProtocolTCP)
	return host, nil
}

func (host *IPv4Host) callback(b []byte, src, dst net.IPv4) {
	var hdr tcpIPv4Header
	n, err := parseTCPIPv4Header(b, &hdr)
	if err != nil {
		// TODO(joshlf): Log it
		return
	}
	// TODO(joshlf): Validate checksum

	b = b[n:]
	host.mu.RLock()
	conn, ok := host.conns[ipv4FourTuple{
		src:     src,
		srcport: hdr.srcport,
		dst:     dst,
		dstport: hdr.dstport,
	}]
	if ok {
		conn.callback(&hdr.genericHeader, b)
	} else {
		listener, ok := host.listeners[ipv4Listener{
			addr: dst,
			port: hdr.dstport,
		}]
		if ok {
			_ = listener
			// TODO(joshlf)
		} else {
			// TODO(joshlf)
		}
	}
	host.mu.RUnlock()
}
