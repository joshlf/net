package internal

import (
	"bufio"
	"io"
	"strings"

	"github.com/joshlf/net"
	"github.com/joshlf/net/internal/errors"
)

// RouteEntry is an entry in a routing table whose next hop is an IP address.
type RouteEntry struct {
	Subnet  net.IPSubnet
	Nexthop net.IP
}

// RouteDeviceEntry is an entry in a routing table whose next hop is a
// locally-connected network through the named device.
type RouteDeviceEntry struct {
	Subnet net.IPSubnet
	Device string
}

// ParseRouteEntry parses an IP route entry of the form
//  0.0.0.0/0 10.0.0.1
// or a device route entry of the form
//  10.0.0.0/8 eth0
// where the device is a human-readable device name.
//
// If no error is encountered, the first return value is either of type
// RouteEntry or of type RouteDeviceEntry. If it is of type RouteEntry, then
// the Subnet and Nexthop fields are guaranteed to be the same IP version - 4
// or 6.
func ParseRouteEntry(s string) (interface{}, error) {
	fields := strings.Fields(s)
	if len(fields) != 2 {
		return nil, errors.Errorf("parse route entry: unexpected number of whitespace-separated fields: %v", len(fields))
	}
	_, subnet, err := net.ParseCIDR(fields[0])
	if err != nil {
		return nil, errors.Annotate(err, "parse route entry")
	}
	nexthop, err := net.ParseIP(fields[1])
	switch {
	case err != nil:
		// assume it's meant to be a device name
		return RouteDeviceEntry{
			Subnet: subnet,
			Device: fields[1],
		}, nil
	case subnet.IPVersion() == nexthop.IPVersion():
		return RouteEntry{
			Subnet:  subnet,
			Nexthop: nexthop,
		}, nil
	default:
		return nil, errors.New("parse route entry: mismatched target and next hop IP versions")
	}
}

// ParseRouteFile reads lines one at a time from r, and parses them into route
// entries using ParseRouteEntry, returning all parsed entries. If an error is
// encountered that is a result of the contents of the file, not an error in
// reading from r, its string form (obtained through the Error method) will be
// annotated with the 1-indexed line number on which the error was encountered.
func ParseRouteFile(r io.Reader) ([]interface{}, error) {
	var routes []interface{}
	s := bufio.NewScanner(r)
	var lnum int
	for s.Scan() {
		lnum++
		route, err := ParseRouteEntry(s.Text())
		if err != nil {
			return nil, errors.Annotatef(err, "parse route file: line %v", lnum)
		}
		routes = append(routes, route)
	}
	if err := s.Err(); err != nil {
		return nil, err
	}
	return routes, nil
}

// ParsePairs reads lines one at a time from r, and parses them into two
// whitespace-separated fields (each line is a pair of fields), returning all
// parsed pairs. If more than two fields are found on a line, the second through
// last are joined with spaces, and considered to be the second field.
// ParsePairs is intended to be used as a helper when parsing device definition
// files. Each line consists of a device name followed by a device definition.
// Since each device type will have its own format for defining parameters,
// the device definitions are left up to the caller to parse.
//
// If an error is encountered that is a result of the contents of the file,
// not an error in reading from r, its string form (obtained through the Error
// method) will be annotated with the 1-indexed line number on which the error
// was encountered.
func ParsePairs(r io.Reader) (pairs [][2]string, err error) {
	s := bufio.NewScanner(r)
	var lnum int
	for s.Scan() {
		lnum++
		fields := strings.Fields(s.Text())
		switch len(fields) {
		case 0:
			continue
		case 1:
			return nil, errors.Errorf("parse pairs: line %v: need two or more whitespace-separated fields", lnum)
		default:
			pairs = append(pairs, [2]string{
				fields[0],
				strings.Join(fields[1:], " "),
			})
		}
	}
	if err := s.Err(); err != nil {
		return nil, err
	}
	return pairs, nil
}
