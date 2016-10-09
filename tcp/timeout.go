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
// still 0, the daemon acquires the global Conn lock. However, there's
// a chance that in between the atomic load of the cancel field
// and acquiring the Conn lock, another goroutine acquired the
// Conn lock, did some work, and canceled the timeout. Thus, after
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
	f      func()
	t      time.Time
	cancel uint32 // 1 if cancelled, 0 otherwise; only access atomically
}

type timeouter struct {
	conn     *Conn
	timeouts heapTimeouts
	// used when len(timeouts) == 0 and the daemon
	// needs to wait until there are more timeouts
	wg sync.WaitGroup

	mu sync.Mutex
}

// assumes the caller as acquired the parent Conn's lock
func (t *timeouter) addTimeout(to *timeout) {
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
		if len(t.timeouts) == 0 {
			// no timeouts; block until one is available
			t.wg.Add(1)
			t.mu.Unlock()
			t.wg.Wait()
			// there's a timeout now, so restart the loop
			// and do the normal sleep thing
			continue
		}

		// poor man's peek
		to := heap.Pop(&t.timeouts).(*timeout)
		heap.Push(&t.timeouts, to)
		t.mu.Unlock()
		time.Sleep(to.t.Sub(time.Now()))

		t.mu.Lock()
		to = heap.Pop(&t.timeouts).(*timeout)
		if atomic.LoadUint32(&to.cancel) == 0 {
			// it wasn't cancelled
			t.conn.mu.Lock()
			if atomic.LoadUint32(&to.cancel) == 0 {
				// it wasn't cancelled between checking to.cancel
				// and acquirint t.conn.mu
				to.f()
			}
			t.conn.mu.Unlock()
		}
		t.mu.Unlock()
	}
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
