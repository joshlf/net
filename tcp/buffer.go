package tcp

type interval struct {
	// index of the first non-filled byte
	end int
	// index of the first filled byte after the gap;
	// sentinal value of -1 if there is no next
	next int
}

type readBuffer struct {
	seq   seq // logical sequence number of the first byte in the buffer
	start int // index into buf/intervals of the "beginning" of the buffer

	// invariant: len(buf) == len(intervals), and this is
	// a power of two (so that the mod operation is cheap)
	buf       []byte
	intervals []interval
}

// all operations on readBuffer are made in the logical index space;
// only getInterval, copyFrom, and copyTo are aware of the physical
// memory layout of the buffer within the buf and intervals struct
// fields

func (r *readBuffer) Insert(b []byte, seq seq) {
	// logical offset from the 0th byte in the buffer
	// offset := int(seq - r.seq)
}

// get the interval at the logical offset from the beginning of r
func (r *readBuffer) getInterval(offset int) *interval {
	idx := r.start + offset
	if idx >= len(r.intervals) {
		idx = idx % len(r.intervals)
	}
	return &r.intervals[idx]
}

// copy r into b starting at the logical offset
// from the beginning of r
func (r *readBuffer) copyFrom(b []byte, offset int) {
	start := r.start + offset
	if start >= len(r.buf) {
		start = start % len(r.buf)
	}

	if start+len(b) < len(r.buf) {
		copy(b, r.buf[start:])
		return
	}
	n := copy(b, r.buf[start:])
	copy(b[n:], r.buf)
}

// copy b into r starting at the logical offset
// from the beginning of r
func (r *readBuffer) copyTo(b []byte, offset int) {
	start := r.start + offset
	if start >= len(r.buf) {
		start = start % len(r.buf)
	}

	if start+len(b) < len(r.buf) {
		copy(r.buf[start:], b)
		return
	}
	n := copy(r.buf[start:], b)
	copy(r.buf, b[n:])
}
