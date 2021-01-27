package ledgerstate

import (
	"math"
	"testing"

	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/kvstore/mapdb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOutputsUnspent(t *testing.T) {
	branchDAG, utxoDAG := setupDependencies(t)
	defer branchDAG.Shutdown()

	outputsMetadata := []*OutputMetadata{
		{
			consumerCount: 0,
		},
		{
			consumerCount: 1,
		},
	}

	assert.False(t, utxoDAG.outputsUnspent(outputsMetadata))
	assert.True(t, utxoDAG.outputsUnspent(outputsMetadata[:1]))
}

func TestInputsInRejectedBranch(t *testing.T) {
	branchDAG, utxoDAG := setupDependencies(t)
	defer branchDAG.Shutdown()
	wallets := createWallets(2)
	input := generateOutput(utxoDAG, wallets[0].address)
	tx, _ := makeSimpleTransaction(utxoDAG, wallets[0], wallets[1], input, true)
	cachedRejectedBranch, _ := branchDAG.branchStorage.StoreIfAbsent(NewConflictBranch(NewBranchID(tx.ID()), nil, nil))

	(&CachedBranch{CachedObject: cachedRejectedBranch}).Consume(func(branch Branch) {
		branch.SetPreferred(false)
		branch.SetLiked(false)
		branch.SetFinalized(true)
		branch.SetInclusionState(Rejected)
	})

	outputsMetadata := []*OutputMetadata{
		{
			branchID: MasterBranchID,
		},
		{
			branchID: NewBranchID(tx.ID()),
		},
	}

	rejected, rejectedBranch := utxoDAG.inputsInRejectedBranch(outputsMetadata)
	assert.True(t, rejected)
	assert.Equal(t, NewBranchID(tx.ID()), rejectedBranch)

	rejected, rejectedBranch = utxoDAG.inputsInRejectedBranch(outputsMetadata[:1])
	assert.False(t, rejected)
	assert.Equal(t, MasterBranchID, rejectedBranch)
}

func TestInputsInInvalidBranch(t *testing.T) {
	branchDAG, utxoDAG := setupDependencies(t)
	defer branchDAG.Shutdown()

	outputsMetadata := []*OutputMetadata{
		{
			branchID: InvalidBranchID,
		},
		{
			branchID: MasterBranchID,
		},
	}

	assert.True(t, utxoDAG.inputsInInvalidBranch(outputsMetadata))
	assert.False(t, utxoDAG.inputsInInvalidBranch(outputsMetadata[1:]))
}
func TestConsumedOutputs(t *testing.T) {
	branchDAG, utxoDAG := setupDependencies(t)
	defer branchDAG.Shutdown()

	wallets := createWallets(2)
	input := generateOutput(utxoDAG, wallets[0].address)

	// testing when storing the inputs
	tx, output := makeSimpleTransaction(utxoDAG, wallets[0], wallets[1], input, true)
	cachedInputs := utxoDAG.consumedOutputs(tx)
	inputs := cachedInputs.Unwrap()

	assert.Equal(t, input, inputs[0])

	cachedInputs.Release(true)

	// testing when not storing the inputs
	tx, _ = makeSimpleTransaction(utxoDAG, wallets[1], wallets[0], output, false)
	cachedInputs = utxoDAG.consumedOutputs(tx)
	inputs = cachedInputs.Unwrap()

	assert.Equal(t, nil, inputs[0])

	cachedInputs.Release(true)
}

func TestInputsSolid(t *testing.T) {
	branchDAG, utxoDAG := setupDependencies(t)
	defer branchDAG.Shutdown()

	wallets := createWallets(2)
	input := generateOutput(utxoDAG, wallets[0].address)

	// testing when storing the inputs
	tx, output := makeSimpleTransaction(utxoDAG, wallets[0], wallets[1], input, true)
	cachedInputs := utxoDAG.consumedOutputs(tx)
	inputs := cachedInputs.Unwrap()

	assert.True(t, utxoDAG.inputsSolid(inputs))

	cachedInputs.Release()

	// testing when not storing the inputs
	tx, _ = makeSimpleTransaction(utxoDAG, wallets[1], wallets[0], output, false)
	cachedInputs = utxoDAG.consumedOutputs(tx)
	inputs = cachedInputs.Unwrap()

	assert.False(t, utxoDAG.inputsSolid(inputs))

	cachedInputs.Release()
}

func TestTransactionBalancesValid(t *testing.T) {
	branchDAG, utxoDAG := setupDependencies(t)
	defer branchDAG.Shutdown()

	wallets := createWallets(2)

	i1 := NewSigLockedSingleOutput(100, wallets[0].address)
	i2 := NewSigLockedSingleOutput(100, wallets[0].address)

	// testing happy case
	o := NewSigLockedSingleOutput(200, wallets[1].address)

	assert.True(t, utxoDAG.transactionBalancesValid(Outputs{i1, i2}, Outputs{o}))

	// testing creating 1 iota out of thin air
	i2 = NewSigLockedSingleOutput(99, wallets[0].address)

	assert.False(t, utxoDAG.transactionBalancesValid(Outputs{i1, i2}, Outputs{o}))

	// testing burning 1 iota
	i2 = NewSigLockedSingleOutput(101, wallets[0].address)

	assert.False(t, utxoDAG.transactionBalancesValid(Outputs{i1, i2}, Outputs{o}))

	// testing unit64 overflow
	i2 = NewSigLockedSingleOutput(math.MaxUint64, wallets[0].address)

	assert.False(t, utxoDAG.transactionBalancesValid(Outputs{i1, i2}, Outputs{o}))
}

func TestUnlockBlocksValid(t *testing.T) {
	branchDAG, utxoDAG := setupDependencies(t)
	defer branchDAG.Shutdown()

	wallets := createWallets(2)

	input := generateOutput(utxoDAG, wallets[0].address)

	// testing valid signature
	tx, _ := makeSimpleTransaction(utxoDAG, wallets[0], wallets[1], input, true)
	assert.True(t, utxoDAG.unlockBlocksValid(Outputs{input}, tx))

	// testing invalid signature
	tx, _ = makeSimpleTransaction(utxoDAG, wallets[1], wallets[0], input, true)
	assert.False(t, utxoDAG.unlockBlocksValid(Outputs{input}, tx))

}

func setupDependencies(t *testing.T) (*BranchDAG, *UTXODAG) {
	store := mapdb.NewMapDB()
	branchDAG := NewBranchDAG(store)
	err := branchDAG.Prune()
	require.NoError(t, err)

	return branchDAG, NewUTXODAG(store, branchDAG)
}

type wallet struct {
	keyPair ed25519.KeyPair
	address *ED25519Address
}

func (w wallet) privateKey() ed25519.PrivateKey {
	return w.keyPair.PrivateKey
}

func (w wallet) publicKey() ed25519.PublicKey {
	return w.keyPair.PublicKey
}

func createWallets(n int) []wallet {
	wallets := make([]wallet, 2)
	for i := 0; i < n; i++ {
		kp := ed25519.GenerateKeyPair()
		wallets[i] = wallet{
			kp,
			NewED25519Address(kp.PublicKey),
		}
	}
	return wallets
}

func (w wallet) sign(txEssence *TransactionEssence) *ED25519Signature {
	return NewED25519Signature(w.publicKey(), ed25519.Signature(w.privateKey().Sign(txEssence.Bytes())))
}

func (w wallet) unlockBlocks(txEssence *TransactionEssence) []UnlockBlock {
	return []UnlockBlock{NewSignatureUnlockBlock(w.sign(txEssence))}
}

func generateOutput(utxoDAG *UTXODAG, address Address) *SigLockedSingleOutput {
	output := NewSigLockedSingleOutput(100, address)
	output.SetID(NewOutputID(GenesisTransactionID, 1))
	utxoDAG.outputStorage.StoreIfAbsent(output)
	return output
}

func makeSimpleTransaction(utxoDAG *UTXODAG, a, b wallet, outputToSpend *SigLockedSingleOutput, store bool) (*Transaction, *SigLockedSingleOutput) {
	input := NewUTXOInput(outputToSpend.ID())
	output := NewSigLockedSingleOutput(100, b.address)

	txEssence := NewTransactionEssence(0, NewInputs(input), NewOutputs(output))

	tx := NewTransaction(txEssence, a.unlockBlocks(txEssence))

	outputToSpend.SetID(NewOutputID(tx.ID(), 0))

	if store {
		utxoDAG.outputStorage.StoreIfAbsent(outputToSpend)
	}

	return tx, output
}
