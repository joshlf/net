package buffer

// A ReadBuffer is a buffer capable of storing sparse blocks of bytes,
// and automatically coalescing neighboring blocks when possible.
type ReadBuffer struct {
	seq           uint32 // logical sequence number of the first byte in the buffer
	firstInterval int    // index of first interval or -1 if none

	buf       circularBuffer
	intervals intervalAllocator
}

// NewReadBuffer creates a new ReadBuffer whose first byte has the given
// sequence number and which has an underlying buffer of size n.
func NewReadBuffer(n int, seq uint32) *ReadBuffer {
	return &ReadBuffer{
		seq:           seq,
		firstInterval: -1,
		buf:           *newCircularBuffer(n),
		intervals:     newIntervalAllocator(n),
	}
}

// Write writes b into r at the given sequence number, and returns the sequence
// number of the next byte which has not been written.
func (r *ReadBuffer) Write(b []byte, seq uint32) (next uint32) {
	r.write(b, int(seq-r.seq))
	return r.Next()
}

// ReadAndAdvance reads from r into b, and advances the beginning of r
// to the first byte not read into b. If len(b) bytes are not available
// at the beginning of r, the behavior of ReadAndAdvance is undefined.
func (r *ReadBuffer) ReadAndAdvance(b []byte) {
	r.buf.CopyFrom(b, 0)
	r.seq += uint32(len(b))
	for idx := r.firstInterval; idx != -1; idx = r.intervals.intervals[idx].next {
		r.intervals.intervals[idx].begin += len(b)
	}
}

// Next returns the sequence number of the next byte which has not been written.
func (r *ReadBuffer) Next() uint32 {
	return uint32(int(r.seq) + r.Available())
}

// Available returns the number of bytes available to be read from the beginning
// of the buffer.
func (r *ReadBuffer) Available() int {
	if r.firstInterval == -1 || r.intervals.intervals[r.firstInterval].begin != 0 {
		// there are no intervals, or the offset from the
		// beginning of the first interval is non-zero
		return 0
	}
	return r.intervals.intervals[r.firstInterval].len
}

func (r *ReadBuffer) write(b []byte, offset int) {
	r.buf.CopyTo(b, offset)
	if r.firstInterval == -1 {
		idx := r.intervals.New()
		ivl := &r.intervals.intervals[idx]
		ivl.begin = offset
		ivl.len = len(b)
		ivl.next = -1
		r.firstInterval = idx
		// no need to coalesce; this is the only interval
		return
	}

	// keep track of the index of cur, although for r.firstInterval,
	// curidx is undefined (which we signify with -1)
	curidx := -1
	cur := &r.firstInterval
	next := *cur
	for next != -1 && r.intervals.intervals[next].begin < offset {
		curidx = next
		cur = &r.intervals.intervals[next].next
		next = *cur
	}
	idx := r.intervals.New()
	ivl := &r.intervals.intervals[idx]
	ivl.begin = offset
	ivl.len = len(b)
	ivl.next = next
	*cur = idx
	if curidx == -1 {
		// we added to the beginning of the list,
		// so start coalescing at the new interval
		r.coalesce(idx)
	} else {
		r.coalesce(curidx)
	}
}

// coalesce starts at the given first interval, and runs until it finds a gap
func (r *ReadBuffer) coalesce(firstInterval int) {
	cur := &r.intervals.intervals[firstInterval]
	nextidx := cur.next
	for nextidx != -1 {
		next := &r.intervals.intervals[nextidx]
		if cur.begin+cur.len < next.begin {
			break
		}

		if cur.len < (next.begin-cur.begin)+next.len {
			cur.len = (next.begin - cur.begin) + next.len
		}
		cur.next = next.next
		r.intervals.Free(nextidx)
		nextidx = cur.next
	}
}

type interval struct {
	begin int // offset from the beginning of the buffer
	len   int
	// begin, end int // [begin, end) as offsets from the beginning of the buffer
	next int // index into intervals slice of next interval or -1 if none

	// for internal allocator use: index of next free interval object
	nextFree int
}

// An intervalAllocator is a slab allocator for intervals.
// It is faster than using the Go allocator because of
// simplicity and lack of synchronization, and the resultant
// objects are contiguous, and thus cache-friendly.
type intervalAllocator struct {
	intervals []interval
	firstFree int
}

func newIntervalAllocator(n int) intervalAllocator {
	ia := intervalAllocator{intervals: make([]interval, n)}
	for i := 0; i < n-1; i++ {
		ia.intervals[i].nextFree = i + 1
	}
	return ia
}

// New allocates a new interval from i and returns its index.
// If no intervals are free, the behavior of New is undefined.
func (i *intervalAllocator) New() int {
	idx := i.firstFree
	i.firstFree = i.intervals[idx].nextFree
	return idx
}

// Free frees the interval at the given index.
func (i *intervalAllocator) Free(idx int) {
	i.intervals[idx].nextFree = i.firstFree
	i.firstFree = idx
}
