package buffer

import (
	"fmt"
	"math/rand"
	"testing"
)

func BenchmarkMod(b *testing.B) {
	// so the compiler doesn't optimize away our mods
	var collector int
	b.Run("non-power-of-two", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			collector = i % 1023
		}
	})
	b.Run("power-of-two", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			collector = i % 1024
		}
	})
	if b.N == 0 {
		// b.N will never be 0, but the compiler doesn't know that
		fmt.Println(collector)
	}
}

func TestIntervalAllocator(t *testing.T) {
	const allocatorSize = 4

	allocator := newIntervalAllocator(allocatorSize)
	// keep track of allocated indices so we can check
	// to make sure we never get a double-allocation
	allocated := make(map[int]bool)

	allocate := func() {
		idx := allocator.New()
		if allocated[idx] {
			t.Fatalf("double-allocation of index %v", idx)
		}
		allocated[idx] = true
	}
	free := func() {
		var idx int
		for i := range allocated {
			idx = i
			break
		}
		allocator.Free(idx)
		delete(allocated, idx)
	}

	for i := 0; i < 1024; i++ {
		switch len(allocated) {
		case 0:
			// nothing to free; have to allocate
			allocate()
		case allocatorSize:
			// nothing to allocate; have to free
			free()
		default:
			if rand.Int()%2 == 0 {
				allocate()
			} else {
				free()
			}
		}
	}
}

func BenchmarkIntervalAllocator(b *testing.B) {
	slabsize := 65536
	ia := newIntervalAllocator(slabsize)

	b.Run("intervalAllocator", func(b *testing.B) {
		b.StopTimer()
		for i := 0; i < b.N; {
			n := slabsize
			quit := false
			if b.N-i < slabsize {
				n = b.N - i
				quit = true
			}
			i += n

			b.StartTimer()
			for j := 0; j < n; j++ {

			}
			b.StopTimer()
			if !quit {
				for i := 0; i < slabsize; i++ {
					ia.Free(i)
				}
			}
		}
	})

	b.Run("new", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = new(interval)
		}
	})
}

func TestReadBuffer(t *testing.T) {
	// t.Skip()

	type readBufferWriteAction struct {
		n   int // number of bytes to write
		seq uint32
	}
	type readBufferTest struct {
		// initial sequence number
		seq uint32
		// buffer size
		n int
		// actions to perform
		actions []readBufferWriteAction
		// state to expect after actions are performed
		intervals []interval
		next      uint32
	}

	// indices given explicitly to make finding the nth test easier
	// when reading the output of a test failure
	tests := []readBufferTest{
		0: {},
		1: {seq: 1, next: 1},
		2: {n: 1024, actions: []readBufferWriteAction{{n: 3}},
			next: 3, intervals: []interval{{begin: 0, len: 3}}},
		3: {seq: 2, n: 1024, actions: []readBufferWriteAction{{n: 3, seq: 2}},
			next: 5, intervals: []interval{{begin: 0, len: 3}}},
		4: {n: 1024, actions: []readBufferWriteAction{{n: 3, seq: 1}},
			next: 0, intervals: []interval{{begin: 1, len: 3}}},
		5: {n: 1024, actions: []readBufferWriteAction{{n: 3, seq: 1}, {n: 1, seq: 0}},
			next: 4, intervals: []interval{{begin: 0, len: 4}}},
		6: {n: 1024, actions: []readBufferWriteAction{{n: 3, seq: 1}, {n: 1, seq: 0}, {n: 3, seq: 10}, {n: 3, seq: 6}, {n: 1, seq: 9}},
			next: 4, intervals: []interval{{begin: 0, len: 4}, {begin: 6, len: 7}}},
		7: {n: 1024, actions: []readBufferWriteAction{{n: 3, seq: 1}, {n: 1, seq: 0}, {n: 3, seq: 10}, {n: 3, seq: 6}, {n: 1, seq: 9}, {n: 1, seq: 5}, {n: 1, seq: 4}},
			next: 13, intervals: []interval{{begin: 0, len: 13}}},

		// test writes which overlap with existing intervals
		// A: -###
		// B: ####
		8: {n: 1024, actions: []readBufferWriteAction{{seq: 1, n: 3}, {seq: 0, n: 4}},
			next: 4, intervals: []interval{{begin: 0, len: 4}}},
		// A: -###------
		// B: ##########
		9: {n: 1024, actions: []readBufferWriteAction{{seq: 1, n: 3}, {seq: 0, n: 10}},
			next: 10, intervals: []interval{{begin: 0, len: 10}}},
		// A: ####----####
		// B: --########--
		10: {n: 1024, actions: []readBufferWriteAction{{seq: 0, n: 4}, {seq: 8, n: 4}, {seq: 2, n: 8}},
			next: 12, intervals: []interval{{begin: 0, len: 12}}},
		// A: ----####----####
		// B: ------####------
		11: {n: 1024, actions: []readBufferWriteAction{{seq: 4, n: 4}, {seq: 12, n: 4}, {seq: 6, n: 4}},
			next: 0, intervals: []interval{{begin: 4, len: 6}, {begin: 12, len: 4}}},
		// A: ----####----####--
		// B: ------############
		12: {n: 1024, actions: []readBufferWriteAction{{seq: 4, n: 4}, {seq: 12, n: 4}, {seq: 6, n: 12}},
			next: 0, intervals: []interval{{begin: 4, len: 14}}},
		// A: ---##
		// B: --##-
		// C: -##--
		// D: ##---
		13: {n: 1024, actions: []readBufferWriteAction{{seq: 3, n: 2}, {seq: 2, n: 2}, {seq: 1, n: 2}, {seq: 0, n: 2}},
			next: 5, intervals: []interval{{begin: 0, len: 5}}},
	}

	intervalSlicesEqual := func(a, b []interval) bool {
		if len(a) != len(b) {
			return false
		}
		for i, aa := range a {
			bb := b[i]
			if aa.begin != bb.begin || aa.len != bb.len {
				return false
			}
		}
		return true
	}
	sprintIntervalSlice := func(ints []interval) string {
		str := "["
		for i, ivl := range ints {
			if i > 0 {
				str += " "
			}
			str += fmt.Sprintf("{begin:%v len:%v}", ivl.begin, ivl.len)
		}
		return str + "]"
	}
	sprintState := func(ints []interval, next uint32) string {
		return fmt.Sprintf("{next:%v intervals:%v}", next, sprintIntervalSlice(ints))
	}

	for i, test := range tests {
		rb := NewReadBuffer(test.n, test.seq)
		for _, act := range test.actions {
			rb.Write(make([]byte, act.n), act.seq)
		}

		var intervals []interval
		curidx := rb.firstInterval
		for curidx != -1 {
			intervals = append(intervals, rb.intervals.intervals[curidx])
			curidx = rb.intervals.intervals[curidx].next
		}
		next := rb.Next()
		if next != test.next || !intervalSlicesEqual(test.intervals, intervals) {
			t.Errorf("test %v:\n\tgot:  %v\n\twant: %v", i, sprintState(intervals, next), sprintState(test.intervals, test.next))
		}
	}
}

func BenchmarkReadBuffer(b *testing.B) {
	// a writePattern is a function which writes b into rb in a given pattern
	// such that, at the end, every byte in rb has been written exactly once
	type writePattern func(rb *ReadBuffer, b []byte, count int)

	// size and blockSize must be powers of two, and size >= blockSize
	getRun := func(size, blockSize int, f writePattern) func(b *testing.B) {
		count := size / blockSize
		rb := NewReadBuffer(size, 0)
		buf := make([]byte, blockSize)
		return func(b *testing.B) {
			// just zero out the relevant fields to reset rb
			for idx := rb.firstInterval; idx != -1; idx = rb.intervals.intervals[idx].next {
				rb.intervals.Free(idx)
			}
			rb.firstInterval = -1
			rb.buf.start = 0

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				f(rb, buf, count)
			}
			b.SetBytes(int64(size))
		}
	}

	patterns := []struct {
		name    string
		pattern writePattern
	}{
		{"in-order", func(rb *ReadBuffer, b []byte, count int) {
			for i := 0; i < count; i++ {
				rb.Write(b, uint32(i*len(b)))
			}
		}},
		{"evens-then-odds", func(rb *ReadBuffer, b []byte, count int) {
			for i := 0; i < count; i += 2 {
				rb.Write(b, uint32(i*len(b)))
			}
			for i := 1; i < count; i += 2 {
				rb.Write(b, uint32(i*len(b)))
			}
		}},
	}

	for _, pattern := range patterns {
		for size := 64; size <= 65536; size *= 2 {
			for blockSize := 64; blockSize <= 1024; blockSize *= 2 {
				if blockSize > size {
					continue
				}
				b.Run(fmt.Sprintf("%5v/%5v/%v", size, blockSize, pattern.name),
					getRun(size, blockSize, pattern.pattern))
			}
		}
	}
}

func BenchmarkCircularBuffer(b *testing.B) {
	getRun := func(size, bufsize int, f func(*circularBuffer, []byte, int)) func(b *testing.B) {
		buf := make([]byte, size)
		return func(b *testing.B) {
			cbuf := newCircularBuffer(bufsize)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				f(cbuf, buf, 0)
				cbuf.Advance(len(buf))
			}
			b.StopTimer()
			b.SetBytes(int64(len(buf)))
		}
	}

	sizes := []int{1, 2, 4, 5, 16, 17, 64, 65, 256, 257}

	for _, size := range sizes {
		b.Run(fmt.Sprintf("copyTo/pow2/%v", size), getRun(size, 65536, (*circularBuffer).CopyTo))
	}
	for _, size := range sizes {
		b.Run(fmt.Sprintf("copyTo/%v", size), getRun(size, 65535, (*circularBuffer).CopyTo))
	}
	for _, size := range sizes {
		b.Run(fmt.Sprintf("copyFrom/pow2/%v", size), getRun(size, 65536, (*circularBuffer).CopyFrom))
	}
	for _, size := range sizes {
		b.Run(fmt.Sprintf("copyFrom/%v", size), getRun(size, 65535, (*circularBuffer).CopyFrom))
	}
}
