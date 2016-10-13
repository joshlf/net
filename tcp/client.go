package tcp

import (
	"time"

	"github.com/joshlf/net/internal/errors"
	"github.com/joshlf/net/tcp/internal/timeout"
)

var (
	timeoutErr = errors.Timeoutf("i/o timeout")
)

// TODO(joshlf): Deal with EOFs for reading and writing

// Read implements the net.Conn Read method.
func (c *Conn) Read(b []byte) (n int, err error) {
	if len(b) == 0 {
		return 0, nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	if reachedDeadline(c.rdeadline) {
		return 0, timeoutErr
	}

	for n = c.incoming.Available(); n == 0; n = c.incoming.Available() {
		c.readCond.Wait()
		if reachedDeadline(c.rdeadline) {
			return 0, timeoutErr
		}
	}

	if n > len(b) {
		n = len(b)
	}

	c.incoming.ReadAndAdvance(b[:n])
	return n, nil
}

// Write implements the net.Conn Write method.
func (c *Conn) Write(b []byte) (n int, err error) {
	if len(b) == 0 {
		return 0, nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	if reachedDeadline(c.wdeadline) {
		return 0, timeoutErr
	}

	for len(b) > 0 {
		var avail int
		for avail = c.outgoing.Cap(); avail == 0; avail = c.outgoing.Cap() {
			c.writeCond.Wait()
			if reachedDeadline(c.wdeadline) {
				// we may have already written some data; return n
				return n, timeoutErr
			}
		}
		if avail > len(b) {
			avail = len(b)
		}
		c.outgoing.Write(b[:avail])
		b = b[avail:]
		n += avail
	}

	return n, nil
}

// NOTE(joshlf): The deadline mechanism is a tad subtle, so we document it
// explicitly here. We don't distinguish between read and write deadlines
// here; the algorithm is identical in both caes.
//
// A deadline affects callers to Read or Write. At any given point in time,
// these can be broken into three categories: those which are waiting on
// the condition variable, those which have the mutex, and those which have
// not yet acquired the mutex in the first place.
//
// When a deadline is set, two things happen: the deadline is recorded
// in the Conn structure, and a timeout is set to fire after the deadline
// has passed (note that the deadline might be in the past, in which case
// the timeout will fire as soon as the timeout daemon gets around to it).
//
// At this point, the deadline-setting code has acquired the mutex, so we
// know that all callers to Read or Write fall into only one of two categories:
// those which are waiting on the condition variable, and those which have
// not yet acquired the mutex in the first place. Here we make use of the
// crucial monotonicity property of timeout.NowMonotonic, and the guarantee
// that timeouts will never execute before their scheduled times.
//
// Let the deadline (which is also the time at which the timeout is scheduled
// to fire) be D. Let the time at which the timeout fires be F. It is
// guaranteed that D <= F. The callback (invoked at time F) broadcasts on
// the condition variable, waking up all waiting callers to Read and Write.
// Let the time at which any given caller acquires the mutex be C. Since
// the thread executing the callback obtains the mutex before calling it,
// and releases the mutex afterwards, we have that D <= F <= C. When the
// caller to Read or Write acquires the mutex at C, it checks the current
// time using timeout.NowMonotonic. Because of the monotonicity property,
// and since D <= C, we know that the time returned by timeout.NowMonotonic
// is not earlier than D. Thus, when the caller checks to see whether the
// deadline arrives, it is guaranteed to find that, yes, the deadline has
// arrived. By monotonicity, any caller in the future will find the same
// to be true.

// SetDeadline implements the net.Conn SetDeadline method.
func (c *Conn) SetDeadline(t time.Time) {
	t = timeToMonotonic(t)
	c.mu.Lock()
	c.setReadDeadline(t)
	c.setWriteDeadline(t)
	c.mu.Unlock()
}

// SetReadDeadline implements the net.Conn SetReadDeadline method.
func (c *Conn) SetReadDeadline(t time.Time) {
	t = timeToMonotonic(t)
	c.mu.Lock()
	c.setReadDeadline(t)
	c.mu.Unlock()
}

// SetWriteDeadline implements the net.Conn SetWriteDeadline method.
func (c *Conn) SetWriteDeadline(t time.Time) {
	t = timeToMonotonic(t)
	c.mu.Lock()
	c.setWriteDeadline(t)
	c.mu.Unlock()
}

func (c *Conn) setReadDeadline(t time.Time) {
	c.rdeadline = t
	c.rdhandle.Cancel()
	if t == (time.Time{}) {
		return
	}
	c.setReadTimeout()
}

func (c *Conn) setWriteDeadline(t time.Time) {
	c.wdeadline = t
	c.wdhandle.Cancel()
	if t == (time.Time{}) {
		return
	}
	c.setWriteTimeout()
}

func (c *Conn) setReadTimeout() {
	c.rdhandle = c.timeoutd.AddTimeout(c.readTimeoutCallback, c.rdeadline)
}

func (c *Conn) setWriteTimeout() {
	c.wdhandle = c.timeoutd.AddTimeout(c.writeTimeoutCallback, c.wdeadline)
}

func (c *Conn) readTimeoutCallback()  { c.rdhandle = nil; c.readCond.Broadcast() }
func (c *Conn) writeTimeoutCallback() { c.wdhandle = nil; c.writeCond.Broadcast() }

func reachedDeadline(t time.Time) bool {
	// important that we check that now >= t, not now > t (see notes above)
	return t != (time.Time{}) && !timeout.NowMonotonic().Before(t)
}

// Converts a time.Time obtained using time.Now to a roughly-equivalent time
// in the space used by timeout.NowMonotonic.
func timeToMonotonic(t time.Time) time.Time {
	diff := t.Sub(time.Now())
	return timeout.NowMonotonic().Add(diff)
}
