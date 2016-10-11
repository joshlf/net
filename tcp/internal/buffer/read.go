package buffer

type interval struct {
	// index of the first non-filled byte
	end int
	// index of the first filled byte after the gap;
	// sentinal value of -1 if there is no next
	next int
}

type ReadBuffer struct {
	seq   uint32 // logical sequence number of the first byte in the buffer
	start int    // index into buf/intervals of the "beginning" of the buffer

	// invariant: buf.len() == len(intervals), and this is
	// a power of two (so that the mod operation is cheap)
	buf       circularBuffer
	intervals []interval
}

func NewReadBuffer(n int) *ReadBuffer {
	return &ReadBuffer{
		buf:       *newCircularBuffer(n),
		intervals: make([]interval, n),
	}
}

// all operations on ReadBuffer are made in the logical index space;
// only getInterval, copyFrom, and copyTo are aware of the physical
// memory layout of the buffer within the buf and intervals struct
// fields

func (r *ReadBuffer) Insert(b []byte, seq uint32) {
	// logical offset from the 0th byte in the buffer
	// offset := int(seq - r.seq)
}

// get the interval at the logical offset from the beginning of r
func (r *ReadBuffer) getInterval(offset int) *interval {
	idx := r.start + offset
	if idx >= len(r.intervals) {
		idx = idx % len(r.intervals)
	}
	return &r.intervals[idx]
}
