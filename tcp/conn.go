package tcp

import "sync"

type ipv4Conn struct {
	state func(conn *ipv4Conn, hdr *tcpIPv4Header, b []byte)

	mu sync.Mutex
}

func (conn *ipv4Conn) callback(hdr *tcpIPv4Header, b []byte) {
	conn.mu.Lock()
	conn.state(conn, hdr, b)
	conn.mu.Unlock()
}
