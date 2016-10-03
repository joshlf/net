package net

// arp represents an instance of the ARP protocol.
type arp struct {
}

// newARP creates a new ARP instance; hw and net must be non-zero
func newARP(hw MAC, net IPv4) (*arp, error) {
	if hw == (MAC{}) || net == (IPv4{}) {
		panic("new arp instance with zero addr")
	}
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

const arpHeaderLen = 28

// https://en.wikipedia.org/wiki/Address_Resolution_Protocol#Packet_structure
type arpHeader struct {
	HTYPE, PTYPE uint16
	HLEN, PLEN   byte // HLEN is 6; PLEN is 4
	OPER         uint16
	SHA          MAC
	SPA          IPv4
	THA          MAC
	TPA          IPv4
}
