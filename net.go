package net

import (
	"time"

	"github.com/juju/errors"
)

// ReadDeadliner is the interface that wraps the SetReadDeadline method.
type ReadDeadliner interface {
	// SetReadDeadline sets the deadline for future read-related calls
	// (Read, ReadFrom, etc). If the deadline is reached, these calls
	// will fail with a timeout (see IsTimeout) instead of blocking.
	// A zero value for t means read calls will not time out.
	SetReadDeadline(t time.Time) error
}

// WriteDeadliner is the interface that wraps the SetWriteDeadline method.
type WriteDeadliner interface {
	// SetWriteDeadline sets the deadline for future write-related calls
	// (Write, WriteTo, etc). If the deadline is reached, these calls
	// will fail with a timeout (see IsTimeout) instead of blocking.
	// A zero value for t means write calls will not time out.
	SetWriteDeadline(t time.Time) error
}

// Deadliner is the type that wraps all three deadline-related methods.
type Deadliner interface {
	ReadDeadliner
	WriteDeadliner
	SetDeadline(t time.Time) error // Call SetReadDeadline(t) and SetWriteDeadline(t)
}

type mtuErr string

func (m mtuErr) Error() string { return string(m) }

// IsMTU returns true if err is an MTU-related error
// created by this package.
func IsMTU(err error) bool {
	_, ok := errors.Cause(err).(mtuErr)
	return ok
}

type timeout string

func (t timeout) Error() string { return string(t) }
func (t timeout) Timeout() bool { return true }

// IsTimeout returns true if err is a timeout-related error,
// as defined by having a Timeout() bool method which returns
// true.
func IsTimeout(err error) bool {
	type timeout interface {
		Timeout() bool
	}
	te, ok := errors.Cause(err).(timeout)
	return ok && te.Timeout()
}
