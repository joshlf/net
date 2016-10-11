package buffer

// A circularBuffer is a low-level primitive used by ReadBuffer and WriteBuffer.
// It is unaware of anything TCP-related including the space of sequence numbers.
type circularBuffer struct {
	start int
	buf   []byte
}

func newCircularBuffer(n int) *circularBuffer {
	return &circularBuffer{buf: make([]byte, n)}
}

// Len returns the total length of c.
func (c *circularBuffer) Len() int {
	return len(c.buf)
}

// Advance advances the start of the buffer by n bytes. The n bytes at the
// beginning of the buffer are no longer available, while n more bytes at
// the end of the buffer are now available.
func (c *circularBuffer) Advance(n int) {
	c.start = (c.start + n) % len(c.buf)
}

// CopyFrom copies from b into the buffer at the given offset from the
// beginning. No input validation is performed, and the behavior if
// offset + len(b) > c.Len() is undefined.
func (c *circularBuffer) CopyFrom(b []byte, offset int) {
	start := c.start + offset
	if start >= len(c.buf) {
		start = start % len(c.buf)
	}

	if start+len(b) < len(c.buf) {
		copy(b, c.buf[start:])
		return
	}
	n := copy(b, c.buf[start:])
	copy(b[n:], c.buf)
}

// CopyTo copies from the buffer at the given offset from the beginning and into
// b. No input validation is performed, and the behavior if offset + len(b) >
// c.Len() is undefined.
func (c *circularBuffer) CopyTo(b []byte, offset int) {
	start := c.start + offset
	if start >= len(c.buf) {
		start = start % len(c.buf)
	}

	if start+len(b) < len(c.buf) {
		copy(c.buf[start:], b)
		return
	}
	n := copy(c.buf[start:], b)
	copy(c.buf, b[n:])
}
