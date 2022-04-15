package faucet

import (
	"testing"

	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/identity"
	"github.com/stretchr/testify/assert"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
)

func TestSerixRequest(t *testing.T) {
	keyPair := ed25519.GenerateKeyPair()
	obj := NewRequest(ledgerstate.NewED25519Address(keyPair.PublicKey), identity.ID{}, identity.ID{}, 5)

	assert.Equal(t, obj.BytesOld(), obj.Bytes())

	objRestored, _, err := FromBytes(obj.Bytes())
	assert.NoError(t, err)
	assert.Equal(t, obj.Bytes(), objRestored.Bytes())
}
