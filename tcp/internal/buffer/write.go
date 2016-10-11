package buffer

// A WriteBuffer buffers data written by the client of a TCP connection
// before it is acknowledged by the other side of the connection.
type WriteBuffer struct {
	seq uint32 // logical sequence number of the first byte in the buffer
	len int
	buf circularBuffer
}

// NewWriteBuffer creates a new WriteBuffer with a total buffer capacity of n.
func NewWriteBuffer(n int) *WriteBuffer {
	return &WriteBuffer{buf: *newCircularBuffer(n)}
}

// Len returns the number of bytes currently stored in w.
func (w *WriteBuffer) Len() int {
	return w.len
}

// Cap returns the number of bytes available for storage in w.
// w.Len() + w.Cap() is the total capacity of w.
func (w *WriteBuffer) Cap() int {
	return w.buf.Len() - w.len
}

// Write appends b to the data currently stored in w, increasing the length of
// w to w.Len() + len(b). It performs no input validation, and the behavior
// if len(b) > w.Cap() is undefined.
func (w *WriteBuffer) Write(b []byte) {
	w.buf.CopyTo(b, w.len)
	w.len += len(b)
}

// Read reads into b starting at the given offset into w. It performs no input
// validation, and the behavior if offset + len(b) > w.Len() is undefined.
func (w *WriteBuffer) Read(b []byte, offset int) {
	// TODO(joshlf): Maybe have offset be a uint32?
	w.buf.CopyFrom(b, offset)
}

// Advance advances the beginning of w by n bytes, also increasing w's capacity
// by n bytes.
func (w *WriteBuffer) Advance(n int) {
	// TODO(joshlf): Maybe have n be a uint32?
	w.buf.Advance(n)
	w.len -= n
}
