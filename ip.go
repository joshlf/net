package net

import (
	"fmt"
	"net"

	"github.com/juju/errors"
)

// IPv4 is an IPv4 address
type IPv4 [4]byte

func (IPv4) isIP() {}

// IPVersion returns i's IP version - 4.
func (i IPv4) IPVersion() int { return 4 }

func (i IPv4) String() string {
	return fmt.Sprintf("%d.%d.%d.%d", i[0], i[1], i[2], i[3])
}

// IPv6 is an IPv6 address
type IPv6 [16]byte

func (IPv6) isIP() {}

// IPVersion returns i's IP version - 6.
func (i IPv6) IPVersion() int { return 6 }

func (i IPv6) String() string {
	panic("not implemented")
	// TODO(joshlf): see Go's implementation
}

// IP is an IPv4 or IPv6 address. It is only implemented by IPv4 and IPv6.
type IP interface {
	// IPVersion is the IP's version - 4 or 6.
	IPVersion() int
	isIP()
}

// IPv4Subnet is an IPv4 address and subnet mask. NOTE: Because address bits
// that are not in the netmask do not affect equality, it is not safe to
// determine subnet equality by comparing two IPv4Subnets using ==. Instead,
// use the Equal method.
type IPv4Subnet struct {
	Addr    IPv4
	Netmask IPv4
}

func (IPv4Subnet) isIPSubnet() {}

// IPVersion returns sub's IP version - 4.
func (sub IPv4Subnet) IPVersion() int { return 4 }

// Equal determines whether sub is equal to other.
func (sub IPv4Subnet) Equal(other IPv4Subnet) bool {
	if sub.Netmask != other.Netmask {
		return false
	}
	// NOTE: Safe to modify sub and other because they're
	// passed by value.
	for i, b := range sub.Addr {
		sub.Addr[i] = b & sub.Netmask[i]
	}
	for i, b := range other.Addr {
		other.Addr[i] = b & other.Netmask[i]
	}
	return sub.Addr == other.Addr
}

// Has returns true if addr is in the subnet sub.
func (sub IPv4Subnet) Has(addr IPv4) bool {
	// keep only the network bits so that addr
	// now represents just the network address
	for i, b := range addr {
		addr[i] = b & sub.Netmask[i]
	}
	return addr == sub.Addr
}

// IPv6Subnet is an IPv6 address and subnet mask. NOTE: Because address bits
// that are not in the netmask do not affect equality, it is not safe to
// determine subnet equality by comparing two IPv6Subnets using ==. Instead,
// use the Equal method.
type IPv6Subnet struct {
	Addr    IPv6
	Netmask IPv6
}

func (IPv6Subnet) isIPSubnet() {}

// IPVersion returns sub's IP version - 6.
func (sub IPv6Subnet) IPVersion() int { return 6 }

// Equal determines whether sub is equal to other.
func (sub IPv6Subnet) Equal(other IPv6Subnet) bool {
	if sub.Netmask != other.Netmask {
		return false
	}
	// NOTE: Safe to modify sub and other because they're
	// passed by value.
	for i, b := range sub.Addr {
		sub.Addr[i] = b & sub.Netmask[i]
	}
	for i, b := range other.Addr {
		other.Addr[i] = b & other.Netmask[i]
	}
	return sub.Addr == other.Addr
}

// Has returns true if addr is in the subnet sub.
func (sub IPv6Subnet) Has(addr IPv6) bool {
	// keep only the network bits so that addr
	// now represents just the network address
	for i, b := range addr {
		addr[i] = b & sub.Netmask[i]
	}
	return addr == sub.Addr
}

// IPSubnet is an IPv4 or IPv6 subnet. It is only implemented by IPv4Subnet and
// IPv6Subnet.
type IPSubnet interface {
	// IPVersion returns the subnet's IP version - 4 or 6.
	IPVersion() int
	isIPSubnet()
}

// SubnetEqual is a generic version of IPv4Subnet.Equal or IPv6Subnet.Equal.
// If a and b are not the same IP version, SubnetEqual returns false.
func SubnetEqual(a, b IPSubnet) bool {
	a4, aOK := a.(IPv4Subnet)
	b4, bOK := b.(IPv4Subnet)
	switch {
	case aOK && bOK:
		return a4.Equal(b4)
	case !aOK && !bOK:
		return a.(IPv6Subnet).Equal(b.(IPv6Subnet))
	default:
		return false
	}
}

// SubnetHas is a generic version of IPv4Subnet.Has or IPv6Subnet.Has.
// If sub and addr are not the same IP version, SubnetHas returns false.
func SubnetHas(sub IPSubnet, addr IP) bool {
	sub4, subOK := sub.(IPv4Subnet)
	addr4, addrOK := addr.(IPv4)
	switch {
	case subOK && addrOK:
		return sub4.Has(addr4)
	case !subOK && !addrOK:
		return sub.(IPv6Subnet).Has(addr.(IPv6))
	default:
		return false
	}
}

// ParseIP parses s as an IP address, returning the result. The string s
// can be in dotted decimal ("192.0.2.1") or IPv6 ("2001:db8::68") form.
func ParseIP(s string) (IP, error) {
	ip := net.ParseIP(s)
	switch {
	case ip == nil:
		return nil, errors.Errorf("parse IP: %v", s)
	case ip.To16() == nil:
		var ipv4 IPv4
		copy(ipv4[:], ip.To4())
		return ipv4, nil
	default:
		var ipv6 IPv6
		copy(ipv6[:], ip.To16())
		return ipv6, nil
	}
}

// ParseIPv4 is like ParseIP, but for IPv4 addresses only.
func ParseIPv4(s string) (IPv4, error) {
	ip, err := ParseIP(s)
	switch {
	case err != nil:
		return IPv4{}, err
	case ip.IPVersion() == 6:
		return IPv4{}, errors.New("parse IPv4: argument is IPv6 address")
	default:
		return ip.(IPv4), nil
	}
}

// ParseIPv6 is like ParseIP, but for IPv6 addresses only.
func ParseIPv6(s string) (IPv6, error) {
	ip, err := ParseIP(s)
	switch {
	case err != nil:
		return IPv6{}, err
	case ip.IPVersion() == 4:
		return IPv6{}, errors.New("parse IPv6: argument is IPv4 address")
	default:
		return ip.(IPv6), nil
	}
}

// ParseCIDR parses s as a CIDR notation IP address and mask,
// like "192.0.2.0/24" or "2001:db8::/32", as defined in
// RFC 4632 and RFC 4291.
//
// It returns the IP address and the network implied by the IP
// and mask. For example, ParseCIDR("198.51.100.1/24") returns
// the IP address 198.51.100.1 and the network 198.51.100.0/24.
func ParseCIDR(s string) (IP, IPSubnet, error) {
	ip, ipnet, err := net.ParseCIDR(s)
	if err != nil {
		return nil, nil, err
	}
	if ip.To16() == nil {
		var ipv4 IPv4
		var ipnet4 IPv4Subnet
		copy(ipv4[:], ip.To4())
		copy(ipnet4.Addr[:], ipnet.IP.To4())
		copy(ipnet4.Netmask[:], ipnet.Mask)
		return ipv4, ipnet4, nil
	}
	var ipv6 IPv6
	var ipnet6 IPv6Subnet
	copy(ipv6[:], ip.To16())
	copy(ipnet6.Addr[:], ipnet.IP.To16())
	copy(ipnet6.Netmask[:], ipnet.Mask)
	return ipv6, ipnet6, nil
}

// ParseCIDRIPv4 is like ParseCIDR, but for IPv4 addresses only.
func ParseCIDRIPv4(s string) (IPv4, IPv4Subnet, error) {
	ip, ipnet, err := ParseCIDR(s)
	if err != nil {
		return IPv4{}, IPv4Subnet{}, err
	}
	ipv4, ok := ip.(IPv4)
	if ok {
		return ipv4, ipnet.(IPv4Subnet), nil
	}
	return IPv4{}, IPv4Subnet{}, errors.New("parse IPv4 CIDR: argument is IPv6 subnet")
}

// ParseCIDRIPv6 is like ParseCIDR, but for IPv6 addresses only.
func ParseCIDRIPv6(s string) (IPv6, IPv6Subnet, error) {
	ip, ipnet, err := ParseCIDR(s)
	if err != nil {
		return IPv6{}, IPv6Subnet{}, err
	}
	ipv6, ok := ip.(IPv6)
	if ok {
		return ipv6, ipnet.(IPv6Subnet), nil
	}
	return IPv6{}, IPv6Subnet{}, errors.New("parse IPv6 CIDR: argument is IPv4 subnet")
}

// // InSubnet returns true if addr is in the subnet defined by subnet and netmask.
// // If addr, subnet, and netmask are not all IPv4 addresses or not all IPv6
// // addresses, InSubnet returns false
// func InSubnet(addr, subnet, netmask IP) bool {
// 	addr4, addr4OK := addr.(IPv4)
// 	subnet4, subnet4OK := subnet.(IPv4)
// 	netmask4, netmask4OK := netmask.(IPv4)
// 	switch {
// 	case addr4OK && subnet4OK && netmask4OK:
// 		return InSubnet(addr4, subnet4, netmask4)
// 	case !addr4OK && !subnet4OK && !netmask4OK:
// 		return InSubnet(addr.(IPv6), subnet.(IPv6), netmask.(IPv6))
// 	default:
// 		return false
// 	}
// }

// // IPv4InSubnet is like InSubnet, but for IPv4 addresses.
// func IPv4InSubnet(addr, subnet, netmask IPv4) bool {
// 	// keep only the network bits so that addr
// 	// now represents just the network address
// 	for i, b := range addr {
// 		addr[i] = b & netmask[i]
// 	}
// 	return addr == subnet
// }
//
// // IPv6InSubnet is like InSubnet, but for IPv6 addresses.
// func IPv6InSubnet(addr, subnet, netmask IPv6) bool {
// 	// keep only the network bits so that addr
// 	// now represents just the network address
// 	for i, b := range addr {
// 		addr[i] = b & netmask[i]
// 	}
// 	return addr == subnet
// }

// // IPv4Header represents an IPv4 header.
// type IPv4Header struct {
// 	// TODO(joshlf)
// }
//
// func (*IPv4Header) isIPHeader() {}
//
// // EncodedLen returns the length of the encoded version of hdr in bytes.
// func (hdr *IPv4Header) EncodedLen() int {
// 	panic("not implemented")
// }
//
// // Marshal encodes hdr into b. If b is not long enough
// // (len(b) < hdr.EncodedLen()), Marshal wil panic.
// func (hdr *IPv4Header) Marshal(b []byte) {
// 	panic("not implemented")
// }
//
// // Unmarshal decodes from b into hdr.
// func (hdr *IPv4Header) Unmarshal(b []byte) error {
// 	panic("not implemented")
// }

//
// // IPv6Header represents an IPv6 header.
// type IPv6Header struct {
// 	// TODO(joshlf)
// }
//
// func (*IPv6Header) isIPHeader() {}
//
// // EncodedLen returns the length of the encoded version of hdr in bytes.
// func (hdr *IPv6Header) EncodedLen() int {
// 	panic("not implemented")
// }
//
// // Marshal encodes hdr into b. If b is not long enough
// // (len(b) < hdr.EncodedLen()), Marshal wil panic.
// func (hdr *IPv6Header) Marshal(b []byte) {
// 	panic("not implemented")
// }
//
// // Unmarshal decodes from b into hdr.
// func (hdr *IPv6Header) Unmarshal(b []byte) error {
// 	panic("not implemented")
// }
//
// // IPHeader is an IPv4 or IPv6 header. It is only implemented by *IPv4Header
// // and *IPv6Header.
// type IPHeader interface {
// 	// EncodedLen returns the length of the encoded
// 	// version of the header in bytes.
// 	EncodedLen() int
// 	// Marshal encodes the header into b. If b is not
// 	// long enough (len(b) < EncodedLen()), Marshal
// 	// wil panic.
// 	Marshal(b []byte)
// 	// Unmarshal decodes from b.
// 	Unmarshal(b []byte) error
// 	isIPHeader()
// }
