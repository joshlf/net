package tcp

import (
	"errors"
	"sync"
)

const listenQueueLen = 1024

type Listener struct {
	conns []*Conn
	// lock and unlock operate on the host's write lock;
	// close removes the listener from the host
	lock, unlock, close func()
	closed              bool

	cond *sync.Cond
	mu   sync.Mutex
}

func newListener(close func()) *Listener {
	l := &Listener{close: close}
	l.cond = sync.NewCond(&l.mu)
	return l
}

func (l *Listener) Close() error {
	// acquire a write lock on the host first to avoid deadlock
	l.lock()
	l.mu.Lock()
	defer l.unlock()
	defer l.mu.Unlock()
	if l.closed {
		return errors.New("close on already-closed Listener")
	}
	for _, conn := range l.conns {
		// TODO(joshlf): Send RST
		_ = conn
	}
	l.close()
	l.closed = true
	l.mu.Unlock()
	l.cond.Broadcast()
	return nil
}

func (l *Listener) AcceptTCP() (*Conn, error) {
	l.mu.Lock()
LOOP:
	for {
		switch {
		case l.closed:
			l.mu.Unlock()
			return nil, errors.New("accept on closed Listener")
		case len(l.conns) > 0:
			break LOOP
		}
		l.cond.Wait()
	}
	conn := l.conns[0]
	l.conns = l.conns[1:]
	l.mu.Unlock()
	return conn, nil
}

func (l *Listener) accept(conn *Conn) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	// don't need to check for l being closed because
	// l.Close removes l from the host, which means
	// l.accept is never called after l.Close
	if len(l.conns) >= listenQueueLen {
		return false
	}
	l.conns = append(l.conns, conn)
	return true
}
