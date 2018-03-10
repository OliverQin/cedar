package bundle

import (
	"crypto/rand"
	"testing"
	"time"
)

func TestRngDistribution(t *testing.T) {
	//rng := NewCryptoRng()
	rng := DefaultRNG

	var histo [256]int
	for i := 0; i < (1 << 20); i++ {
		j := rng.Uint16()
		histo[j/256]++
	}
	for i := 0; i < 256; i++ {
		if histo[i] < 3796 || histo[i] > 4396 {
			t.Error("Bin", i, "=", histo[i], "(Should between [3796, 4396])")
		} else {
			t.Log("Bin", i, "=", histo[i])
		}
	}
}

func TestReadBlock(t *testing.T) {
	a := make([]byte, 23333)
	DefaultRNG.Read(a)
}

func BenchmarkRngRead(b *testing.B) {
	a := make([]byte, 23333)
	for i := 0; i < b.N; i++ {
		DefaultRNG.Read(a)
	}
}

func BenchmarkRngSystemRead(b *testing.B) {
	a := make([]byte, 23333)
	for i := 0; i < b.N; i++ {
		rand.Read(a)
	}
}

func BenchmarkRngReadGoroutine(b *testing.B) {
	a := make([][]byte, b.N)
	for i := 0; i < b.N; i++ {
		a[i] = make([]byte, 23333)
		go DefaultRNG.Read(a[i])
	}
}

func BenchmarkRngSystemReadGoroutine(b *testing.B) {
	a := make([][]byte, b.N)
	for i := 0; i < b.N; i++ {
		a[i] = make([]byte, 23333)
		go rand.Read(a[i])
	}
}

func BenchmarkRngFake(b *testing.B) {
	a := make([][]byte, b.N)
	for i := 0; i < b.N; i++ {
		a[i] = make([]byte, 23333)
		go time.Sleep(5 * time.Second)
	}
}
