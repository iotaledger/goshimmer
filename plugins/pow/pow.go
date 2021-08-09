package pow

import (
	"context"
	"crypto"
	"crypto/ed25519"
	"sync"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/logger"
	_ "golang.org/x/crypto/blake2b" // required by crypto.BLAKE2b_512

	"github.com/iotaledger/goshimmer/packages/pow"
)

// ErrMessageTooSmall is returned when the message is smaller than the 8-byte nonce.
var ErrMessageTooSmall = errors.New("message too small")

// parameters
var (
	hash = crypto.BLAKE2b_512

	// configured via parameters
	difficulty             int
	numWorkers             int
	timeout                time.Duration
	parentsRefreshInterval time.Duration
)

var (
	log *logger.Logger

	workerOnce sync.Once
	worker     *pow.Worker
)

// Worker returns the PoW worker instance of the PoW plugin.
func Worker() *pow.Worker {
	workerOnce.Do(func() {
		log = logger.NewLogger(PluginName)
		// load the parameters
		difficulty = Parameters.Difficulty
		numWorkers = Parameters.NumThreads
		timeout = Parameters.Timeout
		parentsRefreshInterval = Parameters.ParentsRefreshInterval
		// create the worker
		worker = pow.New(numWorkers)
	})
	return worker
}

// DoPOW performs the PoW on the provided msg and returns the nonce.
func DoPOW(msg []byte) (uint64, error) {
	content, err := powData(msg)
	if err != nil {
		return 0, err
	}

	// get the PoW worker
	worker := Worker()

	// log.Debugw("start PoW", "difficulty", difficulty, "numWorkers", numWorkers)

	ctx, cancel := context.WithTimeout(context.Background(), parentsRefreshInterval)
	defer cancel()
	nonce, err := worker.Mine(ctx, content[:len(content)-pow.NonceBytes], difficulty)

	// log.Debugw("PoW stopped", "nonce", nonce, "err", err)

	return nonce, err
}

// powData returns the bytes over which PoW should be computed.
func powData(msgBytes []byte) ([]byte, error) {
	contentLength := len(msgBytes) - ed25519.SignatureSize
	if contentLength < pow.NonceBytes {
		return nil, ErrMessageTooSmall
	}
	return msgBytes[:contentLength], nil
}
