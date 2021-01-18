package ledgerstate

import (
	"math"
	"testing"

	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/kvstore/mapdb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTransactionBalancesValid(t *testing.T) {
	branchDAG := NewBranchDAG(mapdb.NewMapDB())
	err := branchDAG.Prune()
	require.NoError(t, err)
	defer branchDAG.Shutdown()

	u := NewUTXODAG(mapdb.NewMapDB(), branchDAG)

	// generate ED25519 public key
	keyPairSource := ed25519.GenerateKeyPair()
	addressSource := NewED25519Address(keyPairSource.PublicKey)
	keyPairDest := ed25519.GenerateKeyPair()
	addressDest := NewED25519Address(keyPairDest.PublicKey)

	i1 := NewSigLockedSingleOutput(100, addressSource)
	i2 := NewSigLockedSingleOutput(100, addressSource)

	// testing happy case
	o := NewSigLockedSingleOutput(200, addressDest)

	assert.True(t, u.transactionBalancesValid(Outputs{i1, i2}, Outputs{o}))

	// testing creating 1 iota out of thin air
	i2 = NewSigLockedSingleOutput(99, addressSource)

	assert.False(t, u.transactionBalancesValid(Outputs{i1, i2}, Outputs{o}))

	// testing burning 1 iota
	i2 = NewSigLockedSingleOutput(101, addressSource)

	assert.False(t, u.transactionBalancesValid(Outputs{i1, i2}, Outputs{o}))

	// testing unit64 overflow
	i2 = NewSigLockedSingleOutput(math.MaxUint64, addressSource)

	assert.False(t, u.transactionBalancesValid(Outputs{i1, i2}, Outputs{o}))
}
