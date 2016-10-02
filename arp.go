package net

// arp represents an instance of the ARP protocol.
type arp struct {
}

func newARP() (*arp, error) {
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
