package tcp

import (
	"github.com/joshlf/net/internal/errors"
	"github.com/joshlf/net/internal/parse"
)

type optionType uint8

const (
	optionTypeEnd optionType = 0
	optionTypeNOP optionType = 1
	optionTypeMSS optionType = 2
)

type genericHeader struct {
	seq     uint32
	ack     uint32
	dataOff uint8 // 4 bits
	flags
	window   uint16
	checksum uint16
	urgptr   uint16

	// options
	mss    uint16
	mssSet bool
}

type tcpIPv4Header struct {
	srcport Port
	dstport Port
	genericHeader
}

// TODO(joshlf): Actually check error conditions

// returns the number of bytes consumed from b unless an error is returned
func parseTCPIPv4Header(b []byte, hdr *tcpIPv4Header) (n int, err error) {
	if len(b) < 20 {
		return 0, errors.Errorf("invalid header length: %v", len(b))
	}

	hdr.srcport = Port(parse.GetUint16(&b))
	hdr.dstport = Port(parse.GetUint16(&b))
	hdr.seq = parse.GetUint32(&b)
	hdr.ack = parse.GetUint32(&b)
	hdr.dataOff = b[0] >> 5
	hdr.flags = flags(b[0]&1)<<7 | flags(b[1])
	b = b[2:]
	hdr.window = parse.GetUint16(&b)
	hdr.checksum = parse.GetUint16(&b)
	hdr.urgptr = parse.GetUint16(&b)

	hdrlen := 20
	switch {
	case hdr.dataOff > 15:
		// don't check for hdr.dataOff < 5 because Postel's Law
		return 0, errors.Errorf("invalid data offset: %v", hdr.dataOff)
	case hdr.dataOff > 5:
		// deal with options
		hdrlen = int(hdr.dataOff) * 4
		if len(b) < hdrlen-20 {
			// 20 bytes consumed so far
			return 0, errors.Errorf("header length %v too short for data offset: %v", len(b)+20, hdr.dataOff)
		}

		// since options could be malformed such that we eat more bytes
		// than we have, defer here; we know that panics will only happen
		// because of index out-of-bounds, so this is safe
		defer func() {
			r := recover()
			if r != nil {
				err = errors.New("malformed options")
			}
		}()

	LOOP:
		for len(b) > 0 {
			typ := optionType(parse.GetByte(&b))
			switch typ {
			case optionTypeEnd:
				break LOOP
			case optionTypeNOP:
				continue
			case optionTypeMSS:
				parse.GetByte(&b) // we know the length
				hdr.mss = parse.GetUint16(&b)
				hdr.mssSet = true
			default:
				// we don't know what this option is,
				// but at least we can skip it
				olen := int(parse.GetByte(&b))
				// we already chomped the first 2 bytes
				parse.GetBytes(&b, 2-olen)
			}
		}
	}
	return hdrlen, nil
}

// returns the number of bytes consumed from b; if hdr.mssOn, len(b) >= 24
func writeTCPIPv4Header(b []byte, hdr *tcpIPv4Header) (int, error) {
	parse.PutUint16(&b, uint16(hdr.srcport))
	parse.PutUint16(&b, uint16(hdr.dstport))
	parse.PutUint32(&b, uint32(hdr.seq))
	parse.PutUint32(&b, uint32(hdr.ack))

	hdr.dataOff = 5
	if hdr.mssSet {
		hdr.dataOff = 6
	}
	b[0] = (hdr.dataOff << 5) | uint8(hdr.flags>>8)
	b[1] = uint8(hdr.flags)
	b = b[2:]

	parse.PutUint16(&b, uint16(hdr.window))
	parse.PutUint16(&b, uint16(hdr.checksum))
	parse.PutUint16(&b, uint16(hdr.urgptr))

	hdrlen := 20
	if hdr.mssSet {
		hdrlen = 24
		parse.PutByte(&b, byte(optionTypeMSS))
		parse.PutByte(&b, 4) // length of option
		parse.PutUint16(&b, hdr.mss)
	}

	return hdrlen, nil
}

type flags uint16

func (f flags) NS() bool  { return f&0x100 != 0 }
func (f flags) CWR() bool { return f&0x80 != 0 }
func (f flags) ECE() bool { return f&0x40 != 0 }
func (f flags) URG() bool { return f&0x20 != 0 }
func (f flags) ACK() bool { return f&0x10 != 0 }
func (f flags) PSH() bool { return f&0x8 != 0 }
func (f flags) RST() bool { return f&0x4 != 0 }
func (f flags) SYN() bool { return f&0x2 != 0 }
func (f flags) FIN() bool { return f&0x1 != 0 }
func (f *flags) SetNS(on bool) {
	if on {
		*f |= 0x100
	} else {
		*f &= ^flags(0x100)
	}
}
func (f *flags) SetCWR(on bool) {
	if on {
		*f |= 0x80
	} else {
		*f &= ^flags(0x80)
	}
}
func (f *flags) SetECE(on bool) {
	if on {
		*f |= 0x40
	} else {
		*f &= ^flags(0x40)
	}
}
func (f *flags) SetURG(on bool) {
	if on {
		*f |= 0x20
	} else {
		*f &= ^flags(0x20)
	}
}
func (f *flags) SetACK(on bool) {
	if on {
		*f |= 0x10
	} else {
		*f &= ^flags(0x10)
	}
}
func (f *flags) SetPSH(on bool) {
	if on {
		*f |= 0x8
	} else {
		*f &= ^flags(0x8)
	}
}
func (f *flags) SetRST(on bool) {
	if on {
		*f |= 0x4
	} else {
		*f &= ^flags(0x4)
	}
}
func (f *flags) SetSYN(on bool) {
	if on {
		*f |= 0x2
	} else {
		*f &= ^flags(0x2)
	}
}
func (f *flags) SetFIN(on bool) {
	if on {
		*f |= 0x1
	} else {
		*f &= ^flags(0x1)
	}
}
