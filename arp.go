package net

// arp represents an instance of the ARP protocol.
type arp struct {
}

// newARP creates a new ARP instance. If non-zero,
// addr4 and/or addr6 are taken to be this device's
// IP addresses, and will be used when responding
// to ARP requests.
func newARP(addr4 IPv4, addr6 IPv6) (*arp, error) {
	panic("not implemented")
}

func (a *arp) HandlePacket(src, dst MAC, payload []byte) error {
	panic("not implemented")
}

func (a *arp) LookupIPv4(ip IPv4) (MAC, error) {
	panic("not implemented")
}

func (a *arp) LookupIPv6(ip IPv6) (MAC, error) {
	panic("not implemented")
}

func (a *arp) Stop() {

}
