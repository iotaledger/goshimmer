package ledgerstate

import (
	"math"
	"testing"

	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/kvstore/mapdb"
	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/hive.go/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBookTransaction(t *testing.T) {
	branchDAG, utxoDAG := setupDependencies(t)
	defer branchDAG.Shutdown()

	wallets := createWallets(1)
	input := generateOutput(utxoDAG, wallets[0].address)

	tx := buildTransaction(utxoDAG, wallets[0], wallets[0], []*SigLockedSingleOutput{input})
	targetBranch, err := utxoDAG.BookTransaction(tx)
	require.NoError(t, err)
	assert.Equal(t, MasterBranchID, targetBranch)
}

func TestBookInvalidTransaction(t *testing.T) {
	branchDAG, utxoDAG := setupDependencies(t)
	defer branchDAG.Shutdown()

	wallets := createWallets(1)
	input := generateOutput(utxoDAG, wallets[0].address)
	tx, _ := singleInputTransaction(utxoDAG, wallets[0], wallets[0], input, false)

	cachedTxMetadata := utxoDAG.TransactionMetadata(tx.ID())
	defer cachedTxMetadata.Release()
	txMetadata := cachedTxMetadata.Unwrap()

	inputsMetadata := OutputsMetadata{}
	utxoDAG.transactionInputsMetadata(tx).Consume(func(metadata *OutputMetadata) {
		inputsMetadata = append(inputsMetadata, metadata)
	})

	utxoDAG.bookInvalidTransaction(tx, txMetadata, inputsMetadata)

	assert.Equal(t, InvalidBranchID, txMetadata.branchID)
	assert.True(t, txMetadata.Solid())
	assert.True(t, txMetadata.Finalized())

	// check that the inputs are still marked as unspent
	assert.True(t, utxoDAG.outputsUnspent(inputsMetadata))
}

func TestBookRejectedTransaction(t *testing.T) {
	branchDAG, utxoDAG := setupDependencies(t)
	defer branchDAG.Shutdown()

	wallets := createWallets(1)
	input := generateOutput(utxoDAG, wallets[0].address)
	tx, _ := singleInputTransaction(utxoDAG, wallets[0], wallets[0], input, false)

	rejectedBranch := NewConflictBranch(BranchID(tx.ID()), nil, nil)
	rejectedBranch.SetFinalized(true)
	utxoDAG.branchDAG.branchStorage.Store(rejectedBranch).Release()

	cachedTxMetadata := utxoDAG.TransactionMetadata(tx.ID())
	defer cachedTxMetadata.Release()
	txMetadata := cachedTxMetadata.Unwrap()

	inputsMetadata := OutputsMetadata{}
	utxoDAG.transactionInputsMetadata(tx).Consume(func(metadata *OutputMetadata) {
		inputsMetadata = append(inputsMetadata, metadata)
	})

	utxoDAG.bookRejectedTransaction(tx, txMetadata, inputsMetadata, rejectedBranch.ID())

	assert.Equal(t, rejectedBranch.ID(), txMetadata.branchID)
	assert.True(t, txMetadata.Solid())
	assert.True(t, txMetadata.LazyBooked())

	// check that the inputs are still marked as unspent
	assert.True(t, utxoDAG.outputsUnspent(inputsMetadata))
}

func TestBookRejectedConflictingTransaction(t *testing.T) {
	branchDAG, utxoDAG := setupDependencies(t)
	defer branchDAG.Shutdown()

	wallets := createWallets(2)
	input := generateOutput(utxoDAG, wallets[0].address)
	singleInputTransaction(utxoDAG, wallets[0], wallets[0], input, true)

	// double spend
	tx, _ := singleInputTransaction(utxoDAG, wallets[0], wallets[1], input, false)

	cachedTxMetadata := utxoDAG.TransactionMetadata(tx.ID())
	defer cachedTxMetadata.Release()
	txMetadata := cachedTxMetadata.Unwrap()

	inputsMetadata := OutputsMetadata{}
	utxoDAG.transactionInputsMetadata(tx).Consume(func(metadata *OutputMetadata) {
		inputsMetadata = append(inputsMetadata, metadata)
	})

	_, err := utxoDAG.bookRejectedConflictingTransaction(tx, txMetadata, inputsMetadata)
	require.NoError(t, err)

	utxoDAG.branchDAG.Branch(txMetadata.BranchID()).Consume(func(branch Branch) {
		assert.False(t, branch.Liked())
		assert.True(t, branch.Finalized())
		assert.True(t, txMetadata.Solid())
		assert.True(t, txMetadata.LazyBooked())
	})
}

func TestBookNonConflictingTransaction(t *testing.T) {
	branchDAG, utxoDAG := setupDependencies(t)
	defer branchDAG.Shutdown()

	wallets := createWallets(2)
	input := generateOutput(utxoDAG, wallets[0].address)
	tx, _ := singleInputTransaction(utxoDAG, wallets[0], wallets[0], input, false)

	cachedTxMetadata := utxoDAG.TransactionMetadata(tx.ID())
	defer cachedTxMetadata.Release()
	txMetadata := cachedTxMetadata.Unwrap()

	inputsMetadata := OutputsMetadata{}
	utxoDAG.transactionInputsMetadata(tx).Consume(func(metadata *OutputMetadata) {
		inputsMetadata = append(inputsMetadata, metadata)
	})

	targetBranch := utxoDAG.bookNonConflictingTransaction(tx, txMetadata, inputsMetadata, BranchIDs{MasterBranchID: types.Void})

	assert.Equal(t, MasterBranchID, targetBranch)

	utxoDAG.branchDAG.Branch(txMetadata.BranchID()).Consume(func(branch Branch) {
		assert.Equal(t, MasterBranchID, txMetadata.BranchID())
		assert.True(t, txMetadata.Solid())
	})

	inclusionState, err := utxoDAG.InclusionState(tx.ID())
	require.NoError(t, err)
	assert.Equal(t, InclusionState(Pending), inclusionState)

	// check that the inputs are marked as spent
	assert.False(t, utxoDAG.outputsUnspent(inputsMetadata))
}

func TestBookConflictingTransaction(t *testing.T) {
	branchDAG, utxoDAG := setupDependencies(t)
	defer branchDAG.Shutdown()

	wallets := createWallets(2)
	input := generateOutput(utxoDAG, wallets[0].address)
	tx1, _ := singleInputTransaction(utxoDAG, wallets[0], wallets[0], input, false)

	cachedTxMetadata := utxoDAG.TransactionMetadata(tx1.ID())
	defer cachedTxMetadata.Release()
	txMetadata := cachedTxMetadata.Unwrap()

	inputsMetadata := OutputsMetadata{}
	utxoDAG.transactionInputsMetadata(tx1).Consume(func(metadata *OutputMetadata) {
		inputsMetadata = append(inputsMetadata, metadata)
	})

	utxoDAG.bookNonConflictingTransaction(tx1, txMetadata, inputsMetadata, BranchIDs{MasterBranchID: types.Void})

	assert.Equal(t, MasterBranchID, txMetadata.BranchID())

	// double spend
	tx2, _ := singleInputTransaction(utxoDAG, wallets[0], wallets[1], input, false)

	cachedTxMetadata2 := utxoDAG.TransactionMetadata(tx2.ID())
	defer cachedTxMetadata2.Release()
	txMetadata2 := cachedTxMetadata2.Unwrap()

	inputsMetadata2 := OutputsMetadata{}
	utxoDAG.transactionInputsMetadata(tx2).Consume(func(metadata *OutputMetadata) {
		inputsMetadata2 = append(inputsMetadata2, metadata)
	})

	// determine the booking details before we book
	branchesOfInputsConflicting, normalizedBranchIDs, conflictingInputs, err := utxoDAG.determineBookingDetails(inputsMetadata2)
	require.NoError(t, err)
	assert.False(t, branchesOfInputsConflicting)

	targetBranch2 := utxoDAG.bookConflictingTransaction(tx2, txMetadata2, inputsMetadata2, normalizedBranchIDs, conflictingInputs.ByID())

	utxoDAG.branchDAG.Branch(txMetadata2.BranchID()).Consume(func(branch Branch) {
		assert.Equal(t, targetBranch2, txMetadata2.BranchID())
		assert.True(t, txMetadata2.Solid())
	})

	assert.NotEqual(t, MasterBranchID, txMetadata.BranchID())

	inclusionState, err := utxoDAG.InclusionState(tx1.ID())
	require.NoError(t, err)
	assert.Equal(t, InclusionState(Pending), inclusionState)

	inclusionState, err = utxoDAG.InclusionState(tx2.ID())
	require.NoError(t, err)
	assert.Equal(t, InclusionState(Pending), inclusionState)

	// check that the inputs are marked as spent
	assert.False(t, utxoDAG.outputsUnspent(inputsMetadata))
	assert.False(t, utxoDAG.outputsUnspent(inputsMetadata2))
}

func TestInclusionState(t *testing.T) {

	{
		branchDAG, utxoDAG := setupDependencies(t)
		defer branchDAG.Shutdown()

		wallets := createWallets(1)
		input := generateOutput(utxoDAG, wallets[0].address)
		tx, _ := singleInputTransaction(utxoDAG, wallets[0], wallets[0], input, true)

		inclusionState, err := utxoDAG.InclusionState(tx.ID())
		require.NoError(t, err)
		assert.Equal(t, InclusionState(Confirmed), inclusionState)
	}

	{
		branchDAG, utxoDAG := setupDependencies(t)
		defer branchDAG.Shutdown()

		wallets := createWallets(1)
		input := generateOutput(utxoDAG, wallets[0].address)
		tx, _ := singleInputTransaction(utxoDAG, wallets[0], wallets[0], input, false)

		inclusionState, err := utxoDAG.InclusionState(tx.ID())
		require.NoError(t, err)
		assert.Equal(t, InclusionState(Pending), inclusionState)
	}

	{
		branchDAG, utxoDAG := setupDependencies(t)
		defer branchDAG.Shutdown()

		wallets := createWallets(1)
		inputs := generateOutputs(utxoDAG, wallets[0].address, BranchIDs{InvalidBranchID: types.Void})
		tx, _ := multipleInputsTransaction(utxoDAG, wallets[0], wallets[0], inputs, false)

		inclusionState, err := utxoDAG.InclusionState(tx.ID())
		require.NoError(t, err)
		assert.Equal(t, InclusionState(Rejected), inclusionState)
	}
}

func TestConsumedBranchIDs(t *testing.T) {
	branchDAG, utxoDAG := setupDependencies(t)
	defer branchDAG.Shutdown()

	wallets := createWallets(1)
	branchIDs := BranchIDs{MasterBranchID: types.Void, InvalidBranchID: types.Void}
	inputs := generateOutputs(utxoDAG, wallets[0].address, branchIDs)
	tx, _ := multipleInputsTransaction(utxoDAG, wallets[0], wallets[0], inputs, true)

	assert.Equal(t, branchIDs, utxoDAG.consumedBranchIDs(tx.ID()))
}

func TestCreatedOutputIDsOfTransaction(t *testing.T) {
	branchDAG, utxoDAG := setupDependencies(t)
	defer branchDAG.Shutdown()

	wallets := createWallets(1)
	input := generateOutput(utxoDAG, wallets[0].address)
	tx, output := singleInputTransaction(utxoDAG, wallets[0], wallets[0], input, true)

	assert.Equal(t, []OutputID{output.ID()}, utxoDAG.createdOutputIDsOfTransaction(tx.ID()))
}

func TestConsumedOutputIDsOfTransaction(t *testing.T) {
	branchDAG, utxoDAG := setupDependencies(t)
	defer branchDAG.Shutdown()

	wallets := createWallets(1)
	input := generateOutput(utxoDAG, wallets[0].address)
	tx, _ := singleInputTransaction(utxoDAG, wallets[0], wallets[0], input, true)

	assert.Equal(t, []OutputID{input.ID()}, utxoDAG.consumedOutputIDsOfTransaction(tx.ID()))
}

func TestInputsSpentByConfirmedTransaction(t *testing.T) {
	branchDAG, utxoDAG := setupDependencies(t)
	defer branchDAG.Shutdown()

	wallets := createWallets(1)
	input := generateOutput(utxoDAG, wallets[0].address)
	tx, _ := singleInputTransaction(utxoDAG, wallets[0], wallets[0], input, true)

	outputsMetadata := OutputsMetadata{}
	utxoDAG.transactionInputsMetadata(tx).Consume(func(metadata *OutputMetadata) {
		outputsMetadata = append(outputsMetadata, metadata)
	})

	// testing before booking consumers.
	spent, err := utxoDAG.inputsSpentByConfirmedTransaction(outputsMetadata)
	assert.NoError(t, err)
	assert.False(t, spent)

	// testing after booking consumers.
	utxoDAG.bookConsumers(outputsMetadata, tx.ID(), types.True)
	spent, err = utxoDAG.inputsSpentByConfirmedTransaction(outputsMetadata)
	assert.NoError(t, err)
	assert.True(t, spent)
}

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
	tx, _ := singleInputTransaction(utxoDAG, wallets[0], wallets[1], input, true)
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
	tx, output := singleInputTransaction(utxoDAG, wallets[0], wallets[1], input, false)
	cachedInputs := utxoDAG.consumedOutputs(tx)
	inputs := cachedInputs.Unwrap()

	assert.Equal(t, input, inputs[0])

	cachedInputs.Release(true)

	// testing when not storing the inputs
	tx, _ = singleInputTransaction(utxoDAG, wallets[1], wallets[0], output, false)
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
	tx, output := singleInputTransaction(utxoDAG, wallets[0], wallets[1], input, false)
	cachedInputs := utxoDAG.consumedOutputs(tx)
	inputs := cachedInputs.Unwrap()

	assert.True(t, utxoDAG.inputsSolid(inputs))

	cachedInputs.Release()

	// testing when not storing the inputs
	tx, _ = singleInputTransaction(utxoDAG, wallets[1], wallets[0], output, false)
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
	tx, _ := singleInputTransaction(utxoDAG, wallets[0], wallets[1], input, true)
	assert.True(t, utxoDAG.unlockBlocksValid(Outputs{input}, tx))

	// testing invalid signature
	tx, _ = singleInputTransaction(utxoDAG, wallets[1], wallets[0], input, true)
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
	unlockBlock := NewSignatureUnlockBlock(w.sign(txEssence))
	unlockBlocks := make([]UnlockBlock, len(txEssence.inputs))
	for i := range txEssence.inputs {
		unlockBlocks[i] = unlockBlock
	}
	return unlockBlocks
}

func generateOutput(utxoDAG *UTXODAG, address Address) *SigLockedSingleOutput {
	output := NewSigLockedSingleOutput(100, address)
	output.SetID(NewOutputID(GenesisTransactionID, 0))
	utxoDAG.outputStorage.Store(output).Release()

	// store OutputMetadata
	metadata := NewOutputMetadata(output.ID())
	metadata.SetBranchID(MasterBranchID)
	metadata.SetSolid(true)
	utxoDAG.outputMetadataStorage.Store(metadata).Release()

	return output
}

func generateOutputs(utxoDAG *UTXODAG, address Address, branchIDs BranchIDs) (outputs []*SigLockedSingleOutput) {
	i := 0
	outputs = make([]*SigLockedSingleOutput, len(branchIDs))
	for branchID := range branchIDs {
		outputs[i] = NewSigLockedSingleOutput(100, address)
		outputs[i].SetID(NewOutputID(GenesisTransactionID, uint16(i)))
		utxoDAG.outputStorage.Store(outputs[i]).Release()

		// store OutputMetadata
		metadata := NewOutputMetadata(outputs[i].ID())
		metadata.SetBranchID(branchID)
		metadata.SetSolid(true)
		utxoDAG.outputMetadataStorage.Store(metadata).Release()
		i++
	}

	return
}

func singleInputTransaction(utxoDAG *UTXODAG, a, b wallet, outputToSpend *SigLockedSingleOutput, finalized bool) (*Transaction, *SigLockedSingleOutput) {
	input := NewUTXOInput(outputToSpend.ID())
	output := NewSigLockedSingleOutput(100, b.address)

	txEssence := NewTransactionEssence(0, NewInputs(input), NewOutputs(output))

	tx := NewTransaction(txEssence, a.unlockBlocks(txEssence))

	output.SetID(NewOutputID(tx.ID(), 0))

	// store TransactionMetadata
	transactionMetadata := NewTransactionMetadata(tx.ID())
	transactionMetadata.SetSolid(true)
	transactionMetadata.SetBranchID(MasterBranchID)

	if finalized {
		transactionMetadata.SetFinalized(true)
	}

	cachedTransactionMetadata := &CachedTransactionMetadata{CachedObject: utxoDAG.transactionMetadataStorage.ComputeIfAbsent(tx.ID().Bytes(), func(key []byte) objectstorage.StorableObject {
		transactionMetadata.Persist()
		transactionMetadata.SetModified()
		return transactionMetadata
	})}
	defer cachedTransactionMetadata.Release()

	utxoDAG.transactionStorage.Store(tx).Release()

	return tx, output
}

func multipleInputsTransaction(utxoDAG *UTXODAG, a, b wallet, outputsToSpend []*SigLockedSingleOutput, finalized bool) (*Transaction, *SigLockedSingleOutput) {
	inputs := make(Inputs, len(outputsToSpend))
	branchIDs := make(BranchIDs, len(outputsToSpend))
	for i, outputToSpend := range outputsToSpend {
		inputs[i] = NewUTXOInput(outputToSpend.ID())
		utxoDAG.OutputMetadata(outputToSpend.ID()).Consume(func(outputMetadata *OutputMetadata) {
			branchIDs[outputMetadata.BranchID()] = types.Void
		})
	}

	output := NewSigLockedSingleOutput(100, b.address)

	txEssence := NewTransactionEssence(0, inputs, NewOutputs(output))

	tx := NewTransaction(txEssence, a.unlockBlocks(txEssence))

	output.SetID(NewOutputID(tx.ID(), 0))

	// store aggreagated branch
	normalizedBranchIDs, _ := utxoDAG.branchDAG.normalizeBranches(branchIDs)
	cachedAggregatedBranch, _, _ := utxoDAG.branchDAG.aggregateNormalizedBranches(normalizedBranchIDs)
	branchID := BranchID{}
	cachedAggregatedBranch.Consume(func(branch Branch) {
		branchID = branch.ID()
	})

	// store TransactionMetadata
	transactionMetadata := NewTransactionMetadata(tx.ID())
	transactionMetadata.SetSolid(true)
	transactionMetadata.SetBranchID(branchID)
	if finalized {
		transactionMetadata.SetFinalized(true)
	}

	cachedTransactionMetadata := &CachedTransactionMetadata{CachedObject: utxoDAG.transactionMetadataStorage.ComputeIfAbsent(tx.ID().Bytes(), func(key []byte) objectstorage.StorableObject {
		transactionMetadata.Persist()
		transactionMetadata.SetModified()
		return transactionMetadata
	})}
	defer cachedTransactionMetadata.Release()

	utxoDAG.transactionStorage.Store(tx).Release()

	return tx, output
}

func buildTransaction(utxoDAG *UTXODAG, a, b wallet, outputsToSpend []*SigLockedSingleOutput) *Transaction {
	inputs := make(Inputs, len(outputsToSpend))
	for i, outputToSpend := range outputsToSpend {
		inputs[i] = NewUTXOInput(outputToSpend.ID())
	}

	output := NewSigLockedSingleOutput(100, b.address)

	txEssence := NewTransactionEssence(0, inputs, NewOutputs(output))

	tx := NewTransaction(txEssence, a.unlockBlocks(txEssence))

	// output.SetID(NewOutputID(tx.ID(), 0))

	return tx
}
