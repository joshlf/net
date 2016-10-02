package net

import "sync"

// TODO(joshlf): Make multiple power-of-two-sized
// pools and have getByteSlice pick appropriately.

var byteSlicePool = sync.Pool{
	New: func() interface{} { return make([]byte, 16) },
}

func getByteSlice(n int) []byte {
	b := byteSlicePool.Get().([]byte)
	if len(b) < n {
		byteSlicePool.Put(b)
		return make([]byte, n)
	}
	return b[:n]
}

func putByteSlice(b []byte) {
	byteSlicePool.Put(b)
}
