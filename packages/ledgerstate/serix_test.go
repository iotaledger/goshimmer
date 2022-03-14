package ledgerstate

import (
	"context"
	"testing"

	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/serix"
	"github.com/stretchr/testify/assert"
)

func TestSerixOutput(t *testing.T) {
	keyPair := ed25519.GenerateKeyPair()
	sigLockedSingleOutput := NewSigLockedSingleOutput(10, NewED25519Address(keyPair.PublicKey))
	sigLockedSingleOutputInner := &sigLockedSingleOutputInner{
		Type:    SigLockedColoredOutputType,
		Balance: 10,
		Address: NewED25519Address(keyPair.PublicKey),
	}

	s := serix.NewAPI()

	// TODO: need to register address interface

	serixBytes, err := s.Encode(context.Background(), sigLockedSingleOutputInner)
	assert.NoError(t, err)
	assert.Equal(t, sigLockedSingleOutput.Bytes(), serixBytes)
}
