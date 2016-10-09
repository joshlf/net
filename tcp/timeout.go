package tcp

import (
	"container/heap"
	"sync"
	"sync/atomic"
	"time"
)

// Timeouts are handled using a timeouter, which runs a single daemon
// goroutine that checks to see when the next timeout will occur,
// sleeps until that time, and then performs whatever action is
// associated with that timeout.
//
// Timeouts may also be cancelled. In these cases, it's desirable to
// avoid having the daemon acquire the global lock on the Conn
// when it's technically not necessary (having found the timeout
// cancelled, the daemon will then just release the lock without
// doing any work). Thus, each timeout object has a cancel field.
// When a timeout is cancelled, the goroutine doing the cancelling
// atomically sets the cancel field to 1. Then, when the daemon
// wakes up from sleep, it atomically loads the cancel field. If the
// field has been set to 1, then the daemon simply throws away the
// timeout object, and waits for the next timeout. If the field is
// still 0, the daemon acquires the global Conn lock (actually, it
// releases the timeouter lock and then acquires the Conn lock and
// then the timeouter lock (in that order) to avoid a deadlock
// with another goroutine, having already acquired the Conn lock,
// calling AddTimeout and thus trying to acquire the timeouter lock).
// However, there's a chance that in between the atomic load of the
// cancel field and acquiring the Conn lock, another goroutine acquired
// the Conn lock, did some work, and canceled the timeout. Thus, after
// acquiring the Conn lock, the daemon must re-check the cancel field.
// If the cancel field is 1, the daemon immediately releases the
// global Conn lock, throws the timeout away, and waits for the next
// timeout.
//
// It is the responsibility of the goroutine doing the cancelling
// to clear any record of the timeout from the Conn object. If a
// timeout has not been cancelled by the time that the daemon
// acquires the global Conn lock, then the timeout's callback is
// executed. In this case, it is the responsibility of the callback
// to clear any record of the timeout from the Conn object.

type timeout struct {
	// it's guaranteed that the Conn's lock
	// will be acquired before f is called
	f      func()
	t      time.Time
	cancel uint32 // 1 if cancelled, 0 otherwise; only access atomically
}

type timeouter struct {
	Conn *Conn

	timeouts heapTimeouts
	// used when len(timeouts) == 0 and the daemon
	// needs to wait until there are more timeouts
	wg sync.WaitGroup
	// used to indicate that the timeouter has been stopped;
	// the daemon must always check this after acquiring
	// mu and before doing any work, returning immediately
	// if stopped == true.
	stopped bool
	mu      sync.Mutex
}

// Start initializes t and spawns the daemon goroutine.
// It assumes that t.Conn has already been initialized.
// Start must be called before any call to AddTimeout.
func (t *timeouter) Start() {
	t.timeouts = nil
	go t.daemon()
}

// Stop stops t's daemon goroutine. Note that Stop
// may return before the goroutine has returned,
// but the goroutine will return eventually. Critically,
// after Stop has returned, the daemon is guaranteed
// not to interact with any memory other than t in
// any way including executing timeout callbacks,
// so the amount of time it takes to finally return
// does not affect the correctness of the rest of
// the program.
func (t *timeouter) Stop() {
	t.mu.Lock()
	t.stopped = true
	if len(t.timeouts) == 0 {
		// the daemon is blocking on t.wg
		t.wg.Done()
	}
	t.mu.Unlock()
}

// AddTimeout adds the timeout to to t's list of timeouts.
// AddTimeout must only be called while the caller holds
// the parent Conn's lock.
func (t *timeouter) AddTimeout(to *timeout) {
	t.mu.Lock()
	heap.Push(&t.timeouts, to)
	if len(t.timeouts) == 1 {
		// there were previously 0 which means that
		// the daemon is blocking on t.wg
		t.wg.Done()
	}
	t.mu.Unlock()
}

func (t *timeouter) daemon() {
	for {
		t.mu.Lock()
		if t.stopped {
			t.mu.Unlock()
			return
		}

		if len(t.timeouts) == 0 {
			// no timeouts; block until one is available
			t.wg.Add(1)
			t.mu.Unlock()
			t.wg.Wait()
			// there's a timeout now, so restart the loop
			// and do the normal sleep thing
			continue
		}

		to := t.peek()
		now := time.Now()
		if now.Before(to.t) {
			t.mu.Unlock()
			time.Sleep(to.t.Sub(now))
			t.mu.Lock()
			if t.stopped {
				t.mu.Unlock()
				return
			}
		}

		to = heap.Pop(&t.timeouts).(*timeout)
		if atomic.LoadUint32(&to.cancel) == 0 {
			// it wasn't cancelled; we now have to release t.mu
			// before acquiring t.Conn.mu in order to avoid a
			// deadlock with another goroutine calling t.AddTimeout.

			t.mu.Unlock()
			t.Conn.mu.Lock()
			t.mu.Lock()
			if t.stopped {
				t.mu.Unlock()
				t.Conn.mu.Unlock()
				return
			}

			// The only modifications to t that are allowed by
			// goroutines other than this one are stopping t
			// (which we just checked for) and inserting things
			// into the t.timeouts heap. Something being inserted
			// into the t.timeouts heap doesn't invalidate the
			// current timeout we're working on, so we can ignore
			// it. If any timeouts that were inserted were
			// supposed to fire already, they will be handled
			// in the next loop iteration.

			if atomic.LoadUint32(&to.cancel) == 0 {
				// it wasn't cancelled between checking to.cancel
				// and acquiring t.Conn.mu
				to.f()
			}
			t.Conn.mu.Unlock()
		}
		t.mu.Unlock()
	}
}

// assumes len(t.timeouts) > 0
func (t *timeouter) peek() *timeout {
	// From the container/heap documentation:
	//
	// Any type that implements heap.Interface may be used as a
	// min-heap with the following invariants (established after
	// Init has been called or if the data is empty or sorted)...
	//
	// Since a sorted list is a valid heap, it must mean that
	// the smallest element is stored in index 0. Thus, the following
	// is not only safe because of the implementation of the
	// heap package, but is actually safe as long as the package's
	// public documentation holds.
	return t.timeouts[0]
}

type heapTimeouts []*timeout

func (h *heapTimeouts) Len() int           { return len(*h) }
func (h *heapTimeouts) Less(i, j int) bool { return (*h)[i].t.Before((*h)[j].t) }
func (h *heapTimeouts) Swap(i, j int)      { (*h)[i], (*h)[j] = (*h)[j], (*h)[i] }
func (h *heapTimeouts) Push(x interface{}) { *h = append(*h, x.(*timeout)) }
func (h *heapTimeouts) Pop() interface{} {
	x := (*h)[len(*h)]
	*h = (*h)[:len(*h)-1]
	return x
}
