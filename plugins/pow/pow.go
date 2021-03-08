package pow

import (
	"context"
	"crypto"
	"crypto/ed25519"
	"errors"
	"sync"
	"time"

	"github.com/iotaledger/goshimmer/packages/pow"
	"github.com/iotaledger/goshimmer/plugins/config"
	"github.com/iotaledger/hive.go/logger"
	_ "golang.org/x/crypto/blake2b" // required by crypto.BLAKE2b_512
)

var (
	// ErrMessageTooSmall is returned when the message is smaller than the 8-byte nonce.
	ErrMessageTooSmall = errors.New("message too small")
)

// parameters
var (
	hash = crypto.BLAKE2b_512

	// configured via parameters
	difficulty int
	numWorkers int
	timeout    time.Duration
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
		difficulty = config.Node().Int(CfgPOWDifficulty)
		pow.BaseDifficulty = difficulty
		pow.ApowWindow = config.Node().Int(CfgPOWWindow)
		pow.AdaptiveRate = config.Node().Float64(CfgPOWRate)
		numWorkers = config.Node().Int(CfgPOWNumThreads)
		timeout = config.Node().Duration(CfgPOWTimeout)
		// create the worker
		worker = pow.New(hash, numWorkers)
	})
	return worker
}

// DoPOW performs the PoW on the provided msg and returns the nonce.
func DoPOW(msg []byte, d int) (uint64, error) {
	content, err := powData(msg)
	if err != nil {
		return 0, err
	}

	// get the PoW worker
	worker := Worker()

	log.Debugw("start PoW", "difficulty", d, "numWorkers", numWorkers)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	nonce, err := worker.Mine(ctx, content[:len(content)-pow.NonceBytes], d)

	log.Debugw("PoW stopped", "nonce", nonce, "err", err)

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
