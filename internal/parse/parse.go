package parse

import "encoding/binary"

// GetBytes returns (*b)[:n] and sets *b = (*b)[n:]
func GetBytes(b *[]byte, n int) []byte {
	ret := (*b)[:n]
	*b = (*b)[n:]
	return ret
}

// GetByte is equivalent to GetBytes(b, 1).
func GetByte(b *[]byte) byte {
	ret := (*b)[0]
	*b = (*b)[1:]
	return ret
}

// GetUint16 calls GetBytes(b, 2) and converts the result to a uint16 using
// big endian encoding.
func GetUint16(b *[]byte) uint16 {
	return binary.BigEndian.Uint16(GetBytes(b, 2))
}

// GetUint32 calls GetBytes(b, 4) and converts the result to a uint32 using
// big endian encoding.
func GetUint32(b *[]byte) uint32 {
	return binary.BigEndian.Uint32(GetBytes(b, 4))
}

// PutUint16 calls GetBytes(b, 2) and encodes n into the result using big endian
// encoding.
func PutUint16(b *[]byte, n uint16) {
	binary.BigEndian.PutUint16(GetBytes(b, 2), n)
}

// PutUint32 calls GetBytes(b, 4) and encodes n into the result using big endian
// encoding.
func PutUint32(b *[]byte, n uint32) {
	binary.BigEndian.PutUint32(GetBytes(b, 4), n)
}
