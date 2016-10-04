package net

import "sync"

// A syncer is used to synchronize access to fields of a structure,
// and also to manage daemon goroutines. All read-only accesses
// should be protected by calling syncer.RLock. All write accesses
// should be protected by calling syncer.Lock. Daemon goroutines
// must only be spawned using syncer.SpawnDaemon, and should
// only be stopped using syncer.StopDaemons.
type syncer struct {
	stop chan struct{}
	wg   sync.WaitGroup
	sync.RWMutex
}

// SpawnDaemon runs f in a separate goroutine. It is f's responsibility
// to periodically check to see whether f.StopChan() is closed, and to
// return when it is.
//
// It is the caller's responsibility to ensure that s.Lock has been called
// prior to calling SpawnDaemon.
func (s *syncer) SpawnDaemon(f func()) {
	if s.stop == nil {
		// no daemons have been spawned yet
		s.stop = make(chan struct{})
	}
	s.wg.Add(1)
	go f()
}

// StopDaemons stops all daemons by closing the channel returned by
// StopChan and waiting for all daemons to return.
//
// It is the caller's responsibility to ensure that s.Lock has been called
// prior to calling SpawnDaemon.
func (s *syncer) StopDaemons() {
	close(s.stop)
	s.wg.Wait()
	s.stop = nil // match the check in SpawnDaemon
}

// StopChan returns a channel which, when closed, instructs all spawned
// daemons to return. Daemons spawned using SpawnDaemon may call StopChan
// without acquiring s.Lock or s.RLock. However, it is not necessarily
// safe for other goroutines to do this.
func (s *syncer) StopChan() <-chan struct{} {
	// we know that it's safe for daemons to call this without synchronization
	// because s.stop is only modified before any daemons have been spawned and
	// after all daemons have returned.
	return s.stop
}
