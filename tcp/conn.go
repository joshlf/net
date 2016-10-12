package tcp

import (
	"sync"
	"time"

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

	// client stuff
	readCond, writeCond  sync.Cond
	rdeadline, wdeadline time.Time
	rdhandle, wdhandle   *timeout.Timeout // guaranteed to be nil if canceled

	mu sync.Mutex
}

func newConn() *Conn {
	c := &Conn{}
	c.timeoutd = timeout.NewDaemon(&c.mu)
	c.readCond.L = &c.mu
	c.writeCond.L = &c.mu
	return c
}

func (conn *Conn) callback(hdr *genericHeader, b []byte) { conn.statefn(conn, hdr, b) }

func (conn *Conn) listen(hdr *genericHeader, b []byte) {

}
