package tcp

import (
	"fmt"
	"math/rand"
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

var stateStrs = [...]string{
	stateListen:      "LISTEN",
	stateSYNRcvd:     "SYN_RCVD",
	stateSYNSent:     "SYN_SENT",
	stateEstablished: "ESTABLISHED",
	stateFINWait1:    "FIN_WAIT_1",
	stateFINWait2:    "FIN_WAIT_2",
	stateClosing:     "CLOSING",
	stateTimeWait:    "TIME_WAIT",
	stateCloseWait:   "CLOSE_WAIT",
	stateLastACK:     "LAST_ACK",
	stateClosed:      "CLOSED",
}

func (s state) String() string {
	if int(s) >= len(stateStrs) {
		return fmt.Sprintf("UNKNOWN_STATE(%v)", int(s))
	}
	return stateStrs[int(s)]
}

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
	// TODO(joshlf): Set buffer size appropriately
	c := &Conn{
		state:    stateListen,
		statefn:  (*Conn).listen,
		outgoing: *buffer.NewWriteBuffer(1024, rand.Uint32()),
	}
	c.timeoutd = timeout.NewDaemon(&c.mu)
	c.readCond.L = &c.mu
	c.writeCond.L = &c.mu
	return c
}

func (conn *Conn) callback(hdr *genericHeader, b []byte) { conn.statefn(conn, hdr, b) }

func (conn *Conn) listen(hdr *genericHeader, b []byte) {
	conn.mu.Lock()
	defer conn.mu.Unlock()
	if !hdr.SYN() {
		panic("internal error: non-SYN delivered to state LISTEN")
	}

	conn.incoming = *buffer.NewReadBuffer(1024, hdr.seq+1)
	// TODO(joshlf)
}

func (conn *Conn) sendReset(hdr *genericHeader) {
	// See "Reset Generation," https://tools.ietf.org/html/rfc793#page-36
	// TODO(joshlf)
}

// State returns the name of the TCP state that conn is currently in.
func (conn *Conn) State() string {
	conn.mu.Lock()
	str := conn.state.String()
	conn.mu.Unlock()
	return str
}
