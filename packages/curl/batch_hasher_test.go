package curl

import (
	"sync"
	"testing"

	"github.com/iotaledger/iota.go/trinary"
	"golang.org/x/crypto/blake2b"
	"golang.org/x/crypto/ed25519"
)

type zeroReader struct{}

func (zeroReader) Read(buf []byte) (int, error) {
	for i := range buf {
		buf[i] = 0
	}
	return len(buf), nil
}

func BenchmarkEd25519(b *testing.B) {
	var zero zeroReader
	public, private, _ := ed25519.GenerateKey(zero)

	message := make([]byte, 75)
	sig := ed25519.Sign(private, message)

	b.ResetTimer()

	var wg sync.WaitGroup
	for i := 0; i < b.N; i++ {
		wg.Add(1)

		go func() {
			if !ed25519.Verify(public, message, sig) {
				panic("valid signature rejected")
			}

			wg.Done()
		}()
	}
	wg.Wait()
}

var sampleTransactionData = make([]byte, 750)

func BenchmarkBytesToTrits(b *testing.B) {
	bytes := blake2b.Sum512(sampleTransactionData)

	b.ResetTimer()

	var wg sync.WaitGroup
	for i := 0; i < b.N; i++ {
		wg.Add(1)

		go func() {
			_, _ = trinary.BytesToTrits(bytes[:])

			wg.Done()
		}()
	}
	wg.Wait()
}

func BenchmarkBlake2b(b *testing.B) {
	var wg sync.WaitGroup
	for i := 0; i < b.N; i++ {
		wg.Add(1)

		go func() {
			blake2b.Sum256(sampleTransactionData)

			wg.Done()
		}()
	}
	wg.Wait()
}

func BenchmarkBatchHasher_Hash(b *testing.B) {
	batchHasher := NewBatchHasher(243, 81)
	tritsToHash := make(trinary.Trits, 7500)

	b.ResetTimer()

	var wg sync.WaitGroup
	for i := 0; i < b.N; i++ {
		wg.Add(1)

		go func() {
			batchHasher.Hash(tritsToHash)

			wg.Done()
		}()
	}
	wg.Wait()
}
