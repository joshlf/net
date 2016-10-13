package net

import "github.com/joshlf/net/internal/errors"

// IsMTU returns true if err is an MTU-related error.
func IsMTU(err error) bool {
	return errors.IsMTU(err)
}

// IsNoRoute returns true if err is a route-related error.
func IsNoRoute(err error) bool {
	return errors.IsNoRoute(err)
}

// IsTimeout returns true if err has a Timeout() bool method that returns true.
func IsTimeout(err error) bool {
	return errors.IsTimeout(err)
}
