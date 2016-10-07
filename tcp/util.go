package tcp

import "encoding/binary"

func getBytes(b *[]byte, n int) []byte {
	ret := (*b)[:n]
	*b = (*b)[n:]
	return ret
}

func getByte(b *[]byte) byte {
	ret := (*b)[0]
	*b = (*b)[1:]
	return ret
}

func getUint16(b *[]byte) uint16 {
	return binary.BigEndian.Uint16(getBytes(b, 2))
}

func getUint32(b *[]byte) uint32 {
	return binary.BigEndian.Uint32(getBytes(b, 4))
}

func putUint16(b *[]byte, n uint16) {
	binary.BigEndian.PutUint16(getBytes(b, 2), n)
}

func putUint32(b *[]byte, n uint32) {
	binary.BigEndian.PutUint32(getBytes(b, 4), n)
}
