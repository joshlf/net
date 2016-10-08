package net

import "github.com/juju/errors"

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

type noRoute struct {
	host string
}

func (n noRoute) Error() string {
	if n.host == "" {
		return "no route to host"
	}
	return n.host + ": no route to host"
}

// IsNoRoute returns true if err is related to there being
// no route to a particular network host.
func IsNoRoute(err error) bool {
	_, ok := errors.Cause(err).(noRoute)
	return ok
}
