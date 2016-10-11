package buffer

import (
	"fmt"
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
