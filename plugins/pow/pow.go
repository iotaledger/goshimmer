package pow

import (
	"context"
	"crypto"
	"errors"
	"sync"
	"time"

	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/message"
	"github.com/iotaledger/goshimmer/packages/pow"
	"github.com/iotaledger/goshimmer/plugins/config"
	_ "golang.org/x/crypto/blake2b" // required by crypto.BLAKE2b_512
)

var (
	// ErrInvalidPOWDifficultly is returned when the nonce of a message does not fulfill the PoW difficulty.
	ErrInvalidPOWDifficultly = errors.New("invalid PoW")
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
	workerOnce sync.Once
	worker     *pow.Worker
)

// Worker returns the PoW worker instance of the PoW plugin.
func Worker() *pow.Worker {
	workerOnce.Do(func() {
		difficulty = config.Node.GetInt(CfgPOWDifficulty)
		numWorkers = config.Node.GetInt(CfgPOWNumThreads)
		timeout = config.Node.GetDuration(CfgPOWTimeout)

		worker = newWorker()
	})
	return worker
}

// DoPOW performs the PoW on the provided msg and returns the nonce.
func DoPOW(msg *message.Message) (uint64, error) {
	content, err := powData(msg)
	if err != nil {
		return 0, err
	}

	worker := Worker()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return worker.Mine(ctx, content, difficulty)
}

// ValidatePOW returns an error when the PoW of the provided msg in invalid.
func ValidatePOW(msg *message.Message) error {
	content, err := powData(msg)
	if err != nil {
		return err
	}
	zeros, err := Worker().LeadingZerosWithNonce(content, msg.Nonce())
	if err != nil {
		return err
	}
	if zeros < difficulty {
		return ErrInvalidPOWDifficultly
	}
	return nil
}

// powData returns the bytes over which PoW should be computed.
func powData(msg *message.Message) ([]byte, error) {
	msgBytes := msg.Bytes()

	contentLength := len(msgBytes) - len(msg.Signature()) - 8
	return msgBytes[:contentLength], nil
}

func newWorker() *pow.Worker {
	return pow.New(hash, numWorkers)
}
