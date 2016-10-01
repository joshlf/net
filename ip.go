package net

// IPv4 is an IPv4 address
type IPv4 [4]byte

func (IPv4) isIP() {}

// IPv6 is an IPv6 address
type IPv6 [16]byte

func (IPv6) isIP() {}

// IP is an IPv4 or IPv6 address. It is only implemented by IPv4 and IPv6.
type IP interface {
	isIP()
}

// InSubnet returns true if addr is in the subnet defined by subnet and netmask.
// If addr, subnet, and netmask are not all IPv4 addresses or not all IPv6
// addresses, InSubnet returns false
func InSubnet(addr, subnet, netmask IP) bool {
	addr4, addr4OK := addr.(IPv4)
	subnet4, subnet4OK := subnet.(IPv4)
	netmask4, netmask4OK := netmask.(IPv4)
	switch {
	case addr4OK && subnet4OK && netmask4OK:
		return InSubnet(addr4, subnet4, netmask4)
	case !addr4OK && !subnet4OK && !netmask4OK:
		return InSubnet(addr.(IPv6), subnet.(IPv6), netmask.(IPv6))
	default:
		return false
	}
}

// InSubnetv4 is like InSubnet, but for IPv4 addresses.
func InSubnetv4(addr, subnet, netmask IPv4) bool {
	// keep only the network bits so that addr
	// now represents just the network address
	for i, b := range addr {
		addr[i] = b & netmask[i]
	}
	return addr == subnet
}

// InSubnetv6 is like InSubnet, but for IPv6 addresses.
func InSubnetv6(addr, subnet, netmask IPv6) bool {
	// keep only the network bits so that addr
	// now represents just the network address
	for i, b := range addr {
		addr[i] = b & netmask[i]
	}
	return addr == subnet
}
