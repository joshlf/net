package tcp

import (
	"sync"
	"time"

	"github.com/joshlf/net/tcp/internal/buffer"
	"github.com/joshlf/net/tcp/internal/timeout"
)

type state uint8

const (
	stateListen state = iota
	stateSYNRcvd
	stateSYNSent
	stateEstablished
	stateFINWait1
	stateFINWait2
	stateClosing
	stateTimeWait
	stateCloseWait
	stateLastACK
	stateClosed
)

type seq uint32

type Conn struct {
	state    state
	statefn  func(conn *Conn, hdr *genericHeader, b []byte)
	timeoutd *timeout.Daemon
	incoming buffer.ReadBuffer
	outgoing buffer.WriteBuffer

	// client stuff
	readCond, writeCond  sync.Cond
	rdeadline, wdeadline time.Time
	rdhandle, wdhandle   *timeout.Timeout // guaranteed to be nil if canceled

	mu sync.Mutex
}

func newListenConn() *Conn {
	// TODO(joshlf): Set buffer size and sequence numbers appropriately
	c := &Conn{
		state: stateListen,
		statefn: (*Conn).listen,
		incoming: *buffer.NewReadBuffer(1024, 0),
		outgoing: *buffer.NewWriteBuffer(1024),
	}
	c.timeoutd = timeout.NewDaemon(&c.mu)
	c.readCond.L = &c.mu
	c.writeCond.L = &c.mu
	return c
}

func (conn *Conn) callback(hdr *genericHeader, b []byte) { conn.statefn(conn, hdr, b) }

func (conn *Conn) listen(hdr *genericHeader, b []byte) {

}
