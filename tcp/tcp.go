package tcp

import (
	"sync"

	"github.com/joshlf/net"
	"github.com/joshlf/net/internal/parse"
)

type TCPPort uint16

type ipv4FourTuple struct {
	src     net.IPv4
	srcport TCPPort
	dst     net.IPv4
	dstport TCPPort
}

type ipv4Listener struct {
	addr net.IPv4
	port TCPPort
}

// IPv4Host ... the zero value is not a valid IPv4Host
type IPv4Host struct {
	iphost    net.IPv4Host
	listeners map[ipv4Listener]struct{} // TODO(joshlf): what's the value type?
	conns     map[ipv4FourTuple]*ipv4Conn

	mu sync.RWMutex
}

func NewIPv4Host(iphost net.IPv4Host) (*IPv4Host, error) {
	host := &IPv4Host{iphost: iphost}
	iphost.RegisterIPv4Callback(host.callback, net.IPProtocolTCP)
	return host, nil
}

func (host *IPv4Host) callback(b []byte, src, dst net.IPv4) {
	var hdr tcpIPv4Header
	n, err := parseTCPIPv4Header(b, &hdr)
	if err != nil {
		// TODO(joshlf): Log it
		return
	}
	// TODO(joshlf): Validate checksum

	b = b[n:]
	host.mu.RLock()
	conn, ok := host.conns[ipv4FourTuple{
		src:     src,
		srcport: hdr.srcport,
		dst:     dst,
		dstport: hdr.dstport,
	}]
	if ok {
		conn.callback(&hdr, b)
	} else {
		listener, ok := host.listeners[ipv4Listener{
			addr: dst,
			port: hdr.dstport,
		}]
		if ok {
			_ = listener
			// TODO(joshlf)
		} else {
			// TODO(joshlf)
		}
	}
	host.mu.RUnlock()
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
