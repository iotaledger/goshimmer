package pow

import (
	"context"
	"encoding/binary"
	"hash"
	"math"
	"math/big"
	"sync"
	"sync/atomic"

	"github.com/cockroachdb/errors"
	"golang.org/x/crypto/blake2b"
)

// errors returned by the PoW
var (
	ErrCancelled = errors.New("canceled")
	ErrDone      = errors.New("done")
)

// NonceBytes specifies the number of bytes required for the nonce.
const NonceBytes = 8

// Hash identifies a cryptographic hash function that is implemented in another package.
type Hash interface {
	// Size returns the length, in bytes, of a digest resulting from the given hash function.
	Size() int
	// New returns a new hash.Hash calculating the given hash function.
	New() hash.Hash
}

// The Worker provides PoW functionality using an arbitrary hash function.
type Worker struct {
	numWorkers int
}

// New creates a new PoW based on the provided hash.
// The optional numWorkers specifies how many go routines are used to mine.
func New(numWorkers ...int) *Worker {
	w := &Worker{
		numWorkers: 1,
	}
	if len(numWorkers) > 0 && numWorkers[0] > 0 {
		w.numWorkers = numWorkers[0]
	}
	return w
}

// Mine performs the PoW.
// It appends the 8-byte nonce to the provided msg and tries to find a nonce
// until the target number of leading zeroes is reached.
// The computation can be be canceled using the provided ctx.
func (w *Worker) Mine(ctx context.Context, msg []byte, target int) (uint64, error) {
	var (
		done    uint32
		counter uint64
		wg      sync.WaitGroup
		results = make(chan uint64, w.numWorkers)
		closing = make(chan struct{})
	)

	// stop when the context has been canceled
	go func() {
		select {
		case <-ctx.Done():
			atomic.StoreUint32(&done, 1)
		case <-closing:
			return
		}
	}()

	workerWidth := math.MaxUint64 / uint64(w.numWorkers)
	for i := 0; i < w.numWorkers; i++ {
		startNonce := uint64(i) * workerWidth
		wg.Add(1)
		go func() {
			defer wg.Done()

			nonce, workerErr := w.worker(msg, startNonce, target, &done, &counter)
			if workerErr != nil {
				return
			}
			atomic.StoreUint32(&done, 1)
			results <- nonce
		}()
	}
	wg.Wait()
	close(results)
	close(closing)

	nonce, ok := <-results
	if !ok {
		return 0, ErrCancelled
	}
	return nonce, nil
}

// LeadingZeros returns the number of leading zeros in the digest of the given data.
func (w *Worker) LeadingZeros(data []byte) (int, error) {
	digest := blake2b.Sum512(data)
	asAnInt := new(big.Int).SetBytes(digest[:])
	return 8*blake2b.Size - asAnInt.BitLen(), nil
}

// LeadingZerosWithNonce returns the number of leading zeros in the digest
// after the provided 8-byte nonce is appended to msg.
func (w *Worker) LeadingZerosWithNonce(msg []byte, nonce uint64) (int, error) {
	buf := make([]byte, len(msg)+NonceBytes)
	copy(buf, msg)
	putUint64(buf[len(msg):], nonce)

	return w.LeadingZeros(buf)
}

func (w *Worker) worker(msg []byte, startNonce uint64, target int, done *uint32, counter *uint64) (uint64, error) {
	buf := make([]byte, len(msg)+NonceBytes)
	copy(buf, msg)
	asAnInt := new(big.Int)

	for nonce := startNonce; ; {
		if atomic.LoadUint32(done) != 0 {
			break
		}
		atomic.AddUint64(counter, 1)

		// write nonce in the buffer
		putUint64(buf[len(msg):], nonce)

		digest := blake2b.Sum512(buf)
		asAnInt.SetBytes(digest[:])
		leadingZeros := 8*blake2b.Size - asAnInt.BitLen()
		if leadingZeros >= target {
			return nonce, nil
		}
		nonce++

	}
	return 0, ErrDone
}

func putUint64(b []byte, v uint64) {
	binary.LittleEndian.PutUint64(b, v)
}
