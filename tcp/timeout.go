package tcp

import (
	"container/heap"
	"sync"
	"sync/atomic"
	"time"
)

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
			to.f()
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
