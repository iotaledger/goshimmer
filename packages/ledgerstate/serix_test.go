package ledgerstate

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/serix"
)

func TestSerixOutput(t *testing.T) {
	keyPair := ed25519.GenerateKeyPair()
	sigLockedSingleOutput := NewSigLockedSingleOutput(10, NewED25519Address(keyPair.PublicKey))
	inner := &sigLockedSingleOutputInner{
		Type:    0,
		Balance: 10,
		Address: NewED25519Address(keyPair.PublicKey),
	}

	s := serix.NewAPI()

	err := s.RegisterObjects((*Address)(nil), new(ED25519Address))
	assert.NoError(t, err)

	serixBytes, err := s.Encode(context.Background(), inner)
	assert.NoError(t, err)
	assert.Equal(t, sigLockedSingleOutput.Bytes(), serixBytes)
}
