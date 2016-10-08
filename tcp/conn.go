package tcp

import "sync"

type conn struct {
	state func(conn *conn, hdr *genericHeader, b []byte)

	mu sync.Mutex
}

func (conn *conn) callback(hdr *genericHeader, b []byte) {
	conn.mu.Lock()
	conn.state(conn, hdr, b)
	conn.mu.Unlock()
}
