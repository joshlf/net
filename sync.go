package net

import "sync"

// A syncer is used to synchronize access to fields of a structure,
// and also to manage daemon goroutines. All read-only accesses
// should be protected by calling syncer.RLock. All write accesses
// should be protected by calling syncer.Lock. Bringing a device up
// should only be done using BringUp, including spawning any daemon
// goroutines. Bringing a device down should only be done using
// BringDown, including terminating any daemon goroutines.
//
// All methods of syncer are concurrency-safe; no synchronization
// (including calling the Lock method) is required to call any method.
//
// The zero value syncer is a valid syncer.
type syncer struct {
	stop chan struct{} // nil if down
	wg   sync.WaitGroup
	up   sync.Mutex

	sync.RWMutex
}

// BringUp brings s up by calling pre, then spawning the daemons. If pre returns
// a non-nil error, then that error is immediately returned; the daemons are not
// spawned, and s is not brought up. The function  calling BringUp must not be
// holding any locks associated with s when calling BringUp. If s is already up,
// BringUp is a no-op.
//
// Note that BringUp does not acquire s.Lock or s.RLock, so it is the
// responsibility of pre to acquire any necessary locks.
func (s *syncer) BringUp(pre func() error, daemons ...func()) error {
	s.up.Lock()
	defer s.up.Unlock()
	if s.stop != nil {
		return nil
	}

	err := pre()
	if err != nil {
		return err
	}

	s.stop = make(chan struct{})
	s.wg.Add(len(daemons))
	for _, d := range daemons {
		go func(d func()) { d(); s.wg.Done() }(d)
	}
	return nil
}

// BringDown brings down s by stopping all daemons, and then calling post. The
// return value of post is returned from BringDown. If s is already down,
// BringDown is a no-op.
//
// Note that BringDown does not acquire s.Lock or s.RLock, so it is the
// responsibility of post to acquire any necessary locks.
func (s *syncer) BringDown(post func() error) error {
	s.up.Lock()
	defer s.up.Unlock()
	if s.stop == nil {
		return nil
	}

	close(s.stop)
	s.wg.Wait()
	s.stop = nil
	s.wg = sync.WaitGroup{}
	return post()
}

// StopChan returns a channel which, when closed, instructs all spawned
// daemons to return. StopChan must only be called from daemon goroutines
// spawned using BringUp - for all other callers, its behavior is undefined,
// and the returned channel may be corrupted.
func (s *syncer) StopChan() <-chan struct{} {
	// we know that it's safe for daemons to call this without synchronization
	// because s.stop is only modified before any daemons have been spawned and
	// after all daemons have returned.
	return s.stop
}
