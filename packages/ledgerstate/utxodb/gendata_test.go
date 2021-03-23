package utxodb

import (
	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/mr-tron/base58"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestGenSeed(t *testing.T) {
	seed := ed25519.NewSeed()
	seedStr := base58.Encode(seed.Bytes())
	t.Logf("seed: %s", seedStr)

	seedBin, err := base58.Decode(seedStr)
	assert.NoError(t, err)

	seedRecover := ed25519.NewSeed(seedBin)
	seedRecoverStr := base58.Encode(seedRecover.Bytes())
	assert.EqualValues(t, seedStr, seedRecoverStr)
}
