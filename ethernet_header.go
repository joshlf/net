package net

import "github.com/joshlf/net/internal/parse"

const (
	// length in bytes of an ethernet frame header
	// which does not include an IEEE 802.1Q tag
	ethernetHeaderLen = 14
	// ...which does include an IEEE 802.1Q tag
	ethernetHeaderLen8021 = 18
)

type ethernetHeader struct {
	src, dst MAC
	// IEEE 802.1Q Header. Either 0 (not present), or the first
	// two bytes are 0x8100, and the second two bytes are the
	// Tag control information (TCI).
	// See https://en.wikipedia.org/wiki/IEEE_802.1Q#Frame_format
	// for more details.
	ieee8021Q uint32
	et        EtherType
}

func (e ethernetHeader) EncodedLen() int {
	if e.Has8021Q() {
		return ethernetHeaderLen8021
	}
	return ethernetHeaderLen
}

// Returns true if e.ieee8021Q != 0.
func (e ethernetHeader) Has8021Q() bool {
	return e.ieee8021Q != 0
}

// Returns the priority code point (3 bits).
// Assumes e.Has8021Q() == true.
func (e ethernetHeader) PCP() uint8 {
	return uint8((e.ieee8021Q >> 13) & 3)
}

// Returns the drop eligible indicator (1 bit).
// Assumes e.Has8021Q() == true.
func (e ethernetHeader) DEI() uint8 {
	return uint8((e.ieee8021Q >> 12) & 1)
}

// Returns the VLAN identifier (12 bits).
// Assumes e.Has8021Q() == true.
func (e ethernetHeader) VID() uint16 {
	return uint16(e.ieee8021Q & 0xFFF)
}

func parseEthernetHeader(b []byte) (eh ethernetHeader, err error) {
	// we use getByte and getBytes to consume b;
	// they panic with an appropriate error if b
	// is not long enough, and we return that error
	defer func() {
		r := recover()
		if r != nil {
			err = r.(error)
		}
	}()

	const tpid = 0x8100
	copy(eh.dst[:], parse.GetBytes(&b, 6))
	copy(eh.src[:], parse.GetBytes(&b, 6))
	eh.et = EtherType(parse.GetUint16(&b))
	if eh.et == tpid {
		eh.ieee8021Q = uint32(eh.et) << 16
		eh.ieee8021Q |= uint32(parse.GetUint16(&b))
		eh.et = EtherType(parse.GetUint16(&b))
	}

	return eh, nil
}

// assumes that b is long enough to hold the encoding of eh
func writeEthernetHeader(eh ethernetHeader, b []byte) {
	copy(parse.GetBytes(&b, 6), eh.dst[:])
	copy(parse.GetBytes(&b, 6), eh.src[:])
	if eh.Has8021Q() {
		// explicitly set the TPID
		eh.ieee8021Q &= 0xFFFF
		eh.ieee8021Q |= 0x81000000
		parse.PutUint32(&b, eh.ieee8021Q)
	}
	parse.PutUint16(&b, uint16(eh.et))
}
