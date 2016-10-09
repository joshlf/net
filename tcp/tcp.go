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

type ipv4TwoTuple struct {
	addr net.IPv4
	port Port
}

// IPv4Host ... the zero value is not a valid IPv4Host
type IPv4Host struct {
	iphost    net.IPv4Host
	listeners map[ipv4TwoTuple]*Listener
	conns     map[ipv4FourTuple]*Conn

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
	host.handle(b, src, dst, &hdr)
}

func (host *IPv4Host) handle(b []byte, src, dst net.IPv4, hdr *tcpIPv4Header) {
	fourtuple := ipv4FourTuple{
		src: src, srcport: hdr.srcport,
		dst: dst, dstport: hdr.dstport,
	}
	twotuple := ipv4TwoTuple{addr: dst, port: hdr.dstport}

	host.mu.RLock()
	conn, ok := host.conns[fourtuple]
	if ok {
		conn.callback(&hdr.genericHeader, b)
		host.mu.RUnlock()
		return
	}

	if _, ok = host.listeners[twotuple]; !ok {
		host.mu.RUnlock()
		// TODO(joshlf): Send ICMP or RST
		return
	}

	// We have a listener listening on this ip:port combination,
	// but we only have a read lock, so we can't modify the map;
	// release the read lock, acquire a write lock, and then
	// double-check every thing in case anything changed in the
	// interim
	host.mu.RUnlock()
	host.mu.Lock()

	conn, ok = host.conns[fourtuple]
	if ok {
		// This is an extremely rare case: a connection was created
		// in between us releasing and re-acquiring the lock. We
		// will now call the connection callback under a write lock,
		// which is globally exclusive. This is expensive, but this
		// condition is rare enough that it's not worth optimizing,
		// which would likely make the solution far more complex.
		conn.callback(&hdr.genericHeader, b)
		host.mu.Unlock()
		return
	}

	listener, ok := host.listeners[twotuple]
	if !ok {
		// This is unlikely to happen - the listener disappeared
		// in the time between us releasing and re-acquiring the
		// lock - but that's fine; just send an ICMP or RST as
		// normal.
		host.mu.Unlock()
		// TODO(joshlf): Send ICMP or RST
	}

	// We actually have a new connection - construct it
	// and inform the listener about it
	c := &Conn{}
	ok = listener.accept(c)
	if !ok {
		// The listener didn't have room in its buffer;
		// just drop the segment on the floor and let them
		// retry or time out
		host.mu.Unlock()
		return
	}

	// We have a connection that has been successfully accepted;
	// put it in the map and then start the whole process of segment
	// handling over again. We need to release the write lock and
	// re-acquire the read lock anyway, so easier to just start
	// from scratch.
	host.conns[fourtuple] = c
	host.mu.Unlock()
	host.handle(b, src, dst, hdr)
}
