package errors

import "github.com/juju/errors"

// TODO(joshlf): Add parse error type?

// New is equivalent to New from the github.com/juju/errors package.
func New(message string) error {
	return errors.New(message)
}

// Annotate is equivalent to Annotate from the github.com/juju/errors package.
func Annotate(other error, message string) error {
	return errors.Annotate(other, message)
}

// Annotatef is equivalent to Annotatef from the github.com/juju/errors package.
func Annotatef(other error, format string, args ...interface{}) error {
	return errors.Annotatef(other, format, args)
}

// Errorf is equivalent to Errorf from the github.com/juju/errors package.
func Errorf(format string, args ...interface{}) error {
	return errors.Errorf(format, args...)
}

// Cause is equivalent to Cause from the github.com/juju/errors package.
func Cause(err error) error {
	return errors.Cause(err)
}

type mtu struct {
	errors.Err
}

// MTUf constructs a new MTU error.
func MTUf(format string, args ...interface{}) error {
	err := errors.NewErr(format, args...)
	err.SetLocation(1)
	return &mtu{err}
}

// IsMTU returns true if err is an MTU error as constructed using MTUf.
func IsMTU(err error) bool {
	_, ok := errors.Cause(err).(*mtu)
	return ok
}

type timeout struct {
	errors.Err
}

func (t *timeout) Timeout() bool { return true }

// Timeoutf constructs a new timeout error with a Timeout() bool method that
// returns true.
func Timeoutf(format string, args ...interface{}) error {
	err := errors.NewErr(format, args...)
	err.SetLocation(1)
	return &timeout{err}
}

// IsTimeout returns true if err has a Timeout() bool method that returns true.
func IsTimeout(err error) bool {
	type timeouter interface {
		Timeout() bool
	}
	to, ok := errors.Cause(err).(timeouter)
	return ok && to.Timeout()
}

type noRoute struct {
	errors.Err
}

// NewNoRoute constructs a new error indicating that there is no route to the
// given host.
func NewNoRoute(host string) error {
	var err errors.Err
	if host == "" {
		err = errors.NewErr("no route to host")
	} else {
		err = errors.NewErr(host + ": no route to host")
	}
	err.SetLocation(1)
	return &noRoute{err}
}

// IsNoRoute returns true if err is a no route-related error as constructed
// using NewNoRoute.
func IsNoRoute(err error) bool {
	_, ok := errors.Cause(err).(*noRoute)
	return ok
}
