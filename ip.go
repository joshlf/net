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

// IPv4InSubnet is like InSubnet, but for IPv4 addresses.
func IPv4InSubnet(addr, subnet, netmask IPv4) bool {
	// keep only the network bits so that addr
	// now represents just the network address
	for i, b := range addr {
		addr[i] = b & netmask[i]
	}
	return addr == subnet
}

// IPv6InSubnet is like InSubnet, but for IPv6 addresses.
func IPv6InSubnet(addr, subnet, netmask IPv6) bool {
	// keep only the network bits so that addr
	// now represents just the network address
	for i, b := range addr {
		addr[i] = b & netmask[i]
	}
	return addr == subnet
}

// IPv4Header represents an IPv4 header.
type IPv4Header struct {
	// TODO(joshlf)
}

func (*IPv4Header) isIPHeader() {}

// EncodedLen returns the length of the encoded version of hdr in bytes.
func (hdr *IPv4Header) EncodedLen() int {
	panic("not implemented")
}

// Marshal encodes hdr into b. If b is not long enough
// (len(b) < hdr.EncodedLen()), Marshal wil panic.
func (hdr *IPv4Header) Marshal(b []byte) {
	panic("not implemented")
}

// Unmarshal decodes from b into hdr.
func (hdr *IPv4Header) Unmarshal(b []byte) error {
	panic("not implemented")
}

// IPv6Header represents an IPv6 header.
type IPv6Header struct {
	// TODO(joshlf)
}

func (*IPv6Header) isIPHeader() {}

// EncodedLen returns the length of the encoded version of hdr in bytes.
func (hdr *IPv6Header) EncodedLen() int {
	panic("not implemented")
}

// Marshal encodes hdr into b. If b is not long enough
// (len(b) < hdr.EncodedLen()), Marshal wil panic.
func (hdr *IPv6Header) Marshal(b []byte) {
	panic("not implemented")
}

// Unmarshal decodes from b into hdr.
func (hdr *IPv6Header) Unmarshal(b []byte) error {
	panic("not implemented")
}

// IPHeader is an IPv4 or IPv6 header. It is only implemented by *IPv4Header
// and *IPv6Header.
type IPHeader interface {
	// EncodedLen returns the length of the encoded
	// version of the header in bytes.
	EncodedLen() int
	// Marshal encodes the header into b. If b is not
	// long enough (len(b) < EncodedLen()), Marshal
	// wil panic.
	Marshal(b []byte)
	// Unmarshal decodes from b.
	Unmarshal(b []byte) error
	isIPHeader()
}
