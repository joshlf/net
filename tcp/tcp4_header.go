package tcp

import "github.com/joshlf/net/internal/parse"

type tcpIPv4Header struct {
	srcport  TCPPort
	dstport  TCPPort
	seq      uint32
	ack      uint32
	dataOff  uint8 // 4 bits
	flags    tcpIPv4Flags
	window   uint16
	checksum uint16
	urgptr   uint16
}

// TODO(joshlf): Actually check error conditions

// returns the number of bytes consumed from b
func parseTCPIPv4Header(b []byte, hdr *tcpIPv4Header) (int, error) {
	hdr.srcport = TCPPort(parse.GetUint16(&b))
	hdr.dstport = TCPPort(parse.GetUint16(&b))
	hdr.seq = parse.GetUint32(&b)
	hdr.ack = parse.GetUint32(&b)
	hdr.dataOff = b[0] >> 5
	hdr.flags = tcpIPv4Flags(b[0]&1)<<7 | tcpIPv4Flags(b[1])
	b = b[2:]
	hdr.window = parse.GetUint16(&b)
	hdr.checksum = parse.GetUint16(&b)
	hdr.urgptr = parse.GetUint16(&b)
	return 20, nil
}

// returns the number of bytes consumed from b
func writeTCPIPv4Header(b []byte, hdr *tcpIPv4Header) (int, error) {
	parse.PutUint16(&b, uint16(hdr.srcport))
	parse.PutUint16(&b, uint16(hdr.dstport))
	parse.PutUint32(&b, uint32(hdr.seq))
	parse.PutUint32(&b, uint32(hdr.ack))
	b[0] = (hdr.dataOff << 5) | uint8(hdr.flags>>8)
	b[1] = uint8(hdr.flags)
	b = b[2:]
	parse.PutUint16(&b, uint16(hdr.window))
	parse.PutUint16(&b, uint16(hdr.checksum))
	parse.PutUint16(&b, uint16(hdr.urgptr))
	return 20, nil
}

type tcpIPv4Flags uint16

func (t tcpIPv4Flags) NS() bool  { return t&0x100 != 0 }
func (t tcpIPv4Flags) CWR() bool { return t&0x80 != 0 }
func (t tcpIPv4Flags) ECE() bool { return t&0x40 != 0 }
func (t tcpIPv4Flags) URG() bool { return t&0x20 != 0 }
func (t tcpIPv4Flags) ACK() bool { return t&0x10 != 0 }
func (t tcpIPv4Flags) PSH() bool { return t&0x8 != 0 }
func (t tcpIPv4Flags) RST() bool { return t&0x4 != 0 }
func (t tcpIPv4Flags) SYN() bool { return t&0x2 != 0 }
func (t tcpIPv4Flags) FIN() bool { return t&0x1 != 0 }
func (t *tcpIPv4Flags) SetNS(on bool) {
	if on {
		*t |= 0x100
	} else {
		*t &= ^tcpIPv4Flags(0x100)
	}
}
func (t *tcpIPv4Flags) SetCWR(on bool) {
	if on {
		*t |= 0x80
	} else {
		*t &= ^tcpIPv4Flags(0x80)
	}
}
func (t *tcpIPv4Flags) SetECE(on bool) {
	if on {
		*t |= 0x40
	} else {
		*t &= ^tcpIPv4Flags(0x40)
	}
}
func (t *tcpIPv4Flags) SetURG(on bool) {
	if on {
		*t |= 0x20
	} else {
		*t &= ^tcpIPv4Flags(0x20)
	}
}
func (t *tcpIPv4Flags) SetACK(on bool) {
	if on {
		*t |= 0x10
	} else {
		*t &= ^tcpIPv4Flags(0x10)
	}
}
func (t *tcpIPv4Flags) SetPSH(on bool) {
	if on {
		*t |= 0x8
	} else {
		*t &= ^tcpIPv4Flags(0x8)
	}
}
func (t *tcpIPv4Flags) SetRST(on bool) {
	if on {
		*t |= 0x4
	} else {
		*t &= ^tcpIPv4Flags(0x4)
	}
}
func (t *tcpIPv4Flags) SetSYN(on bool) {
	if on {
		*t |= 0x2
	} else {
		*t &= ^tcpIPv4Flags(0x2)
	}
}
func (t *tcpIPv4Flags) SetFIN(on bool) {
	if on {
		*t |= 0x1
	} else {
		*t &= ^tcpIPv4Flags(0x1)
	}
}
