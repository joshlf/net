package tcp

import "sync"

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
	state   state
	statefn func(conn *Conn, hdr *genericHeader, b []byte)

	// client stuff
	readCond  *sync.Cond
	writeCond *sync.Cond

	mu sync.Mutex
}

func (conn *Conn) callback(hdr *genericHeader, b []byte) { conn.statefn(conn, hdr, b) }

func (conn *Conn) listen(hdr *genericHeader, b []byte) {

}
