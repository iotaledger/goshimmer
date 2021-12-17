package ledgerstate

import (
	"math"
	"testing"
	"time"

	"github.com/iotaledger/goshimmer/packages/consensus/gof"
	"github.com/iotaledger/goshimmer/packages/database"

	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/identity"
	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/hive.go/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	color1 = Color{1}
	color2 = Color{2}
)

func TestExampleC(t *testing.T) {
	ledgerstate := setupDependencies(t)
	defer ledgerstate.Shutdown()

	outputs := make(map[string]*SigLockedSingleOutput)
	transactions := make(map[string]*Transaction)

	wallets := createWallets(2)
	// Prepare and book TX1
	{
		outputs["A"] = generateOutput(ledgerstate, wallets[0].address, 0)
		transactions["TX1"] = buildTransaction(ledgerstate, wallets[0], wallets[0], []*SigLockedSingleOutput{outputs["A"]})
		targetBranch1, err := ledgerstate.BookTransaction(transactions["TX1"])
		require.NoError(t, err)
		assert.Equal(t, MasterBranchID, targetBranch1)
	}

	// Prepare and book TX2
	{
		outputs["B"] = generateOutput(ledgerstate, wallets[0].address, 1)
		transactions["TX2"] = buildTransaction(ledgerstate, wallets[0], wallets[0], []*SigLockedSingleOutput{outputs["B"]})
		targetBranch2, err := ledgerstate.BookTransaction(transactions["TX2"])
		require.NoError(t, err)
		assert.Equal(t, MasterBranchID, targetBranch2)
	}

	// Prepare and book TX3
	{
		outputs["C"] = transactions["TX1"].Essence().Outputs()[0].(*SigLockedSingleOutput)
		outputs["D"] = transactions["TX2"].Essence().Outputs()[0].(*SigLockedSingleOutput)

		transactions["TX3"] = buildTransaction(ledgerstate, wallets[0], wallets[0], []*SigLockedSingleOutput{outputs["C"], outputs["D"]})
		targetBranch3, err := ledgerstate.BookTransaction(transactions["TX3"])
		require.NoError(t, err)
		assert.Equal(t, MasterBranchID, targetBranch3)
	}

	// Prepare and book Tx4 (double spending B)
	{
		transactions["TX4"] = buildTransaction(ledgerstate, wallets[0], wallets[1], []*SigLockedSingleOutput{outputs["B"]})
		targetBranch4, err := ledgerstate.BookTransaction(transactions["TX4"])
		require.NoError(t, err)
		assert.Equal(t, NewBranchID(transactions["TX4"].ID()), targetBranch4)
	}

	// Prepare and book TX5 (double spending A)
	{
		transactions["TX5"] = buildTransaction(ledgerstate, wallets[0], wallets[1], []*SigLockedSingleOutput{outputs["A"]})
		targetBranch5, err := ledgerstate.BookTransaction(transactions["TX5"])
		require.NoError(t, err)
		assert.Equal(t, NewBranchID(transactions["TX5"].ID()), targetBranch5)
	}

	// Checking TX3
	{
		// Checking that the BranchID of Tx3 is correct
		Tx3AggregatedBranch := NewAggregatedBranch(NewBranchIDs(NewBranchID(transactions["TX1"].ID()), NewBranchID(transactions["TX2"].ID())))
		ledgerstate.CachedTransactionMetadata(transactions["TX3"].ID()).Consume(func(metadata *TransactionMetadata) {
			assert.Equal(t, Tx3AggregatedBranch.ID(), metadata.BranchID())
		})

		// Checking that the parents BranchID of TX3 are TX1 and TX2
		ledgerstate.Branch(Tx3AggregatedBranch.ID()).Consume(func(branch Branch) {
			assert.Equal(t, NewBranchIDs(NewBranchID(transactions["TX1"].ID()), NewBranchID(transactions["TX2"].ID())), branch.Parents())
		})

		time.Sleep(1 * time.Second)

		ledgerstate.ChildBranches(NewBranchID(transactions["TX1"].ID())).Consume(func(childBranch *ChildBranch) {
			assert.Equal(t, AggregatedBranchType, childBranch.ChildBranchType())
		})

		ledgerstate.ChildBranches(NewBranchID(transactions["TX2"].ID())).Consume(func(childBranch *ChildBranch) {
			assert.Equal(t, AggregatedBranchType, childBranch.ChildBranchType())
		})
	}
}

func TestExampleB(t *testing.T) {
	ledgerstate := setupDependencies(t)
	defer ledgerstate.Shutdown()

	outputs := make(map[string]*SigLockedSingleOutput)
	transactions := make(map[string]*Transaction)

	wallets := createWallets(2)
	// Prepare and book TX1
	{
		outputs["A"] = generateOutput(ledgerstate, wallets[0].address, 0)
		transactions["TX1"] = buildTransaction(ledgerstate, wallets[0], wallets[0], []*SigLockedSingleOutput{outputs["A"]})
		targetBranch1, err := ledgerstate.BookTransaction(transactions["TX1"])
		require.NoError(t, err)
		assert.Equal(t, MasterBranchID, targetBranch1)
	}

	// Prepare and book TX2
	{
		outputs["B"] = generateOutput(ledgerstate, wallets[0].address, 1)
		transactions["TX2"] = buildTransaction(ledgerstate, wallets[0], wallets[0], []*SigLockedSingleOutput{outputs["B"]})
		targetBranch2, err := ledgerstate.BookTransaction(transactions["TX2"])
		require.NoError(t, err)
		assert.Equal(t, MasterBranchID, targetBranch2)
	}

	// Prepare and book TX3
	{
		outputs["C"] = transactions["TX1"].Essence().Outputs()[0].(*SigLockedSingleOutput)
		outputs["D"] = transactions["TX2"].Essence().Outputs()[0].(*SigLockedSingleOutput)

		transactions["TX3"] = buildTransaction(ledgerstate, wallets[0], wallets[0], []*SigLockedSingleOutput{outputs["C"], outputs["D"]})
		targetBranch3, err := ledgerstate.BookTransaction(transactions["TX3"])
		require.NoError(t, err)
		assert.Equal(t, MasterBranchID, targetBranch3)
	}

	// Prepare and book Tx4
	{
		transactions["TX4"] = buildTransaction(ledgerstate, wallets[0], wallets[1], []*SigLockedSingleOutput{outputs["D"]})
		targetBranch4, err := ledgerstate.BookTransaction(transactions["TX4"])
		require.NoError(t, err)
		assert.Equal(t, NewBranchID(transactions["TX4"].ID()), targetBranch4)
	}

	// Checking TX3
	{
		// Checking that the BranchID of Tx3 is correct
		Tx3BranchID := NewBranchID(transactions["TX3"].ID())
		ledgerstate.CachedTransactionMetadata(transactions["TX3"].ID()).Consume(func(metadata *TransactionMetadata) {
			assert.Equal(t, Tx3BranchID, metadata.BranchID())
		})

		// Checking that the parents BranchID of Tx3 is MasterBranchID
		ledgerstate.Branch(Tx3BranchID).Consume(func(branch Branch) {
			assert.Equal(t, NewBranchIDs(MasterBranchID), branch.Parents())
		})
	}

	// Checking TX4
	{
		// Checking that the BranchID of Tx4 is correct
		Tx4BranchID := NewBranchID(transactions["TX4"].ID())
		ledgerstate.CachedTransactionMetadata(transactions["TX4"].ID()).Consume(func(metadata *TransactionMetadata) {
			assert.Equal(t, Tx4BranchID, metadata.BranchID())
		})
		// Checking that the parents BranchID of TX4 is MasterBranchID
		ledgerstate.Branch(Tx4BranchID).Consume(func(branch Branch) {
			assert.Equal(t, NewBranchIDs(MasterBranchID), branch.Parents())
		})
	}

	// Prepare and book TX5
	{
		transactions["TX5"] = buildTransaction(ledgerstate, wallets[0], wallets[1], []*SigLockedSingleOutput{outputs["B"]})
		targetBranch5, err := ledgerstate.BookTransaction(transactions["TX5"])
		require.NoError(t, err)
		assert.Equal(t, NewBranchID(transactions["TX5"].ID()), targetBranch5)
	}

	// Checking that the BranchID of TX2 is correct and it is the parent of both TX3 and TX4.
	{
		Tx2BranchID := NewBranchID(transactions["TX2"].ID())
		ledgerstate.CachedTransactionMetadata(transactions["TX2"].ID()).Consume(func(metadata *TransactionMetadata) {
			assert.Equal(t, Tx2BranchID, metadata.BranchID())
		})

		// Checking that the parents BranchID of Tx3 is Tx2BranchID
		ledgerstate.Branch(NewBranchID(transactions["TX3"].ID())).Consume(func(branch Branch) {
			assert.Equal(t, NewBranchIDs(Tx2BranchID), branch.Parents())
		})

		// Checking that the parents BranchID of Tx4 is Tx2BranchID
		ledgerstate.Branch(NewBranchID(transactions["TX4"].ID())).Consume(func(branch Branch) {
			assert.Equal(t, NewBranchIDs(Tx2BranchID), branch.Parents())
		})
	}
}

func TestExampleA(t *testing.T) {
	ledgerstate := setupDependencies(t)
	defer ledgerstate.Shutdown()

	outputs := make(map[string]*SigLockedSingleOutput)
	transactions := make(map[string]*Transaction)

	wallets := createWallets(2)
	// Prepare and book TX1
	{
		outputs["A"] = generateOutput(ledgerstate, wallets[0].address, 0)
		transactions["TX1"] = buildTransaction(ledgerstate, wallets[0], wallets[0], []*SigLockedSingleOutput{outputs["A"]})
		targetBranch1, err := ledgerstate.BookTransaction(transactions["TX1"])
		require.NoError(t, err)
		assert.Equal(t, MasterBranchID, targetBranch1)
	}

	// Prepare and book TX2
	{
		outputs["B"] = generateOutput(ledgerstate, wallets[0].address, 1)
		transactions["TX2"] = buildTransaction(ledgerstate, wallets[0], wallets[0], []*SigLockedSingleOutput{outputs["B"]})
		targetBranch2, err := ledgerstate.BookTransaction(transactions["TX2"])
		require.NoError(t, err)
		assert.Equal(t, MasterBranchID, targetBranch2)
	}

	// Prepare and book TX3
	{
		outputs["C"] = transactions["TX1"].Essence().Outputs()[0].(*SigLockedSingleOutput)
		outputs["D"] = transactions["TX2"].Essence().Outputs()[0].(*SigLockedSingleOutput)

		transactions["TX3"] = buildTransaction(ledgerstate, wallets[0], wallets[0], []*SigLockedSingleOutput{outputs["C"], outputs["D"]})
		targetBranch3, err := ledgerstate.BookTransaction(transactions["TX3"])
		require.NoError(t, err)
		assert.Equal(t, MasterBranchID, targetBranch3)
	}

	// Prepare and book Tx4 (double spending B)
	{
		transactions["TX4"] = buildTransaction(ledgerstate, wallets[0], wallets[1], []*SigLockedSingleOutput{outputs["B"]})
		targetBranch4, err := ledgerstate.BookTransaction(transactions["TX4"])
		require.NoError(t, err)
		assert.Equal(t, NewBranchID(transactions["TX4"].ID()), targetBranch4)
	}

	// Checking TX2
	{
		Tx2BranchID := NewBranchID(transactions["TX2"].ID())
		ledgerstate.CachedTransactionMetadata(transactions["TX2"].ID()).Consume(func(metadata *TransactionMetadata) {
			assert.Equal(t, Tx2BranchID, metadata.BranchID())
		})
	}

	// Checking TX3
	{
		// Checking that the BranchID of Tx3 is correct
		Tx3BranchID := NewBranchID(transactions["TX2"].ID())
		ledgerstate.CachedTransactionMetadata(transactions["TX3"].ID()).Consume(func(metadata *TransactionMetadata) {
			assert.Equal(t, Tx3BranchID, metadata.BranchID())
		})

		// Checking that the parents BranchID of TX3 is the MasterBranchID
		ledgerstate.Branch(Tx3BranchID).Consume(func(branch Branch) {
			assert.Equal(t, NewBranchIDs(MasterBranchID), branch.Parents())
		})
	}
}

func TestBookTransaction(t *testing.T) {
	ledgerstate := setupDependencies(t)
	defer ledgerstate.Shutdown()

	wallets := createWallets(1)
	input := generateOutput(ledgerstate, wallets[0].address, 0)

	tx := buildTransaction(ledgerstate, wallets[0], wallets[0], []*SigLockedSingleOutput{input})
	targetBranch, err := ledgerstate.BookTransaction(tx)
	require.NoError(t, err)
	assert.Equal(t, MasterBranchID, targetBranch)
}

func TestBookNonConflictingTransaction(t *testing.T) {
	ledgerstate := setupDependencies(t)
	defer ledgerstate.Shutdown()

	wallets := createWallets(2)
	input := generateOutput(ledgerstate, wallets[0].address, 0)
	tx, _ := singleInputTransaction(ledgerstate, wallets[0], wallets[0], input, gof.High)

	cachedTxMetadata := ledgerstate.CachedTransactionMetadata(tx.ID())
	defer cachedTxMetadata.Release()
	txMetadata := cachedTxMetadata.Unwrap()

	inputsMetadata := OutputsMetadata{}
	ledgerstate.transactionInputsMetadata(tx).Consume(func(metadata *OutputMetadata) {
		inputsMetadata = append(inputsMetadata, metadata)
	})

	targetBranch := ledgerstate.bookNonConflictingTransaction(tx, txMetadata, inputsMetadata, BranchIDs{MasterBranchID: types.Void})

	assert.Equal(t, MasterBranchID, targetBranch)

	ledgerstate.Branch(txMetadata.BranchID()).Consume(func(branch Branch) {
		assert.Equal(t, MasterBranchID, txMetadata.BranchID())
		assert.True(t, txMetadata.Solid())
	})

	finality, err := ledgerstate.TransactionGradeOfFinality(tx.ID())
	require.NoError(t, err)
	assert.Greater(t, finality, gof.Medium)

	// check that the inputs are marked as spent
	assert.False(t, ledgerstate.outputsUnspent(inputsMetadata))
}

func TestBookConflictingTransaction(t *testing.T) {
	ledgerstate := setupDependencies(t)
	defer ledgerstate.Shutdown()

	wallets := createWallets(2)
	input := generateOutput(ledgerstate, wallets[0].address, 0)
	tx1, _ := singleInputTransaction(ledgerstate, wallets[0], wallets[0], input, gof.High)

	cachedTxMetadata := ledgerstate.CachedTransactionMetadata(tx1.ID())
	defer cachedTxMetadata.Release()
	txMetadata := cachedTxMetadata.Unwrap()

	inputsMetadata := OutputsMetadata{}
	ledgerstate.transactionInputsMetadata(tx1).Consume(func(metadata *OutputMetadata) {
		inputsMetadata = append(inputsMetadata, metadata)
	})

	ledgerstate.bookNonConflictingTransaction(tx1, txMetadata, inputsMetadata, BranchIDs{MasterBranchID: types.Void})

	assert.Equal(t, MasterBranchID, txMetadata.BranchID())

	// double spend
	tx2, _ := singleInputTransaction(ledgerstate, wallets[0], wallets[1], input)

	cachedTxMetadata2 := ledgerstate.CachedTransactionMetadata(tx2.ID())
	defer cachedTxMetadata2.Release()
	txMetadata2 := cachedTxMetadata2.Unwrap()

	inputsMetadata2 := OutputsMetadata{}
	ledgerstate.transactionInputsMetadata(tx2).Consume(func(metadata *OutputMetadata) {
		inputsMetadata2 = append(inputsMetadata2, metadata)
	})

	// determine the booking details before we book
	normalizedBranchIDs, conflictingInputs, err := ledgerstate.determineBookingDetails(inputsMetadata2)
	require.NoError(t, err)

	targetBranch2 := ledgerstate.bookConflictingTransaction(tx2, txMetadata2, inputsMetadata2, normalizedBranchIDs, conflictingInputs.ByID())

	ledgerstate.Branch(txMetadata2.BranchID()).Consume(func(branch Branch) {
		assert.Equal(t, targetBranch2, txMetadata2.BranchID())
		assert.True(t, txMetadata2.Solid())
	})

	assert.NotEqual(t, MasterBranchID, txMetadata.BranchID())

	finality, err := ledgerstate.TransactionGradeOfFinality(tx1.ID())
	require.NoError(t, err)
	assert.Greater(t, finality, gof.Medium)

	finality, err = ledgerstate.TransactionGradeOfFinality(tx2.ID())
	require.NoError(t, err)
	assert.Less(t, finality, gof.Medium)

	// check that the inputs are marked as spent
	assert.False(t, ledgerstate.outputsUnspent(inputsMetadata))
	assert.False(t, ledgerstate.outputsUnspent(inputsMetadata2))
}

func TestConsumedBranchIDs(t *testing.T) {
	ledgerstate := setupDependencies(t)
	defer ledgerstate.Shutdown()

	wallets := createWallets(1)
	branchIDs := BranchIDs{MasterBranchID: types.Void}
	inputs := generateOutputs(ledgerstate, wallets[0].address, branchIDs)
	tx := multipleInputsTransaction(ledgerstate, wallets[0], wallets[0], inputs, gof.High)

	assert.Equal(t, branchIDs, ledgerstate.consumedBranchIDs(tx.ID()))
}

func TestCreatedOutputIDsOfTransaction(t *testing.T) {
	ledgerstate := setupDependencies(t)
	defer ledgerstate.Shutdown()

	wallets := createWallets(1)
	input := generateOutput(ledgerstate, wallets[0].address, 0)
	tx, output := singleInputTransaction(ledgerstate, wallets[0], wallets[0], input, gof.High)

	assert.Equal(t, []OutputID{output.ID()}, ledgerstate.createdOutputIDsOfTransaction(tx.ID()))
}

func TestConsumedOutputIDsOfTransaction(t *testing.T) {
	ledgerstate := setupDependencies(t)
	defer ledgerstate.Shutdown()

	wallets := createWallets(1)
	input := generateOutput(ledgerstate, wallets[0].address, 0)
	tx, _ := singleInputTransaction(ledgerstate, wallets[0], wallets[0], input, gof.High)

	assert.Equal(t, []OutputID{input.ID()}, ledgerstate.consumedOutputIDsOfTransaction(tx.ID()))
}

func TestOutputsUnspent(t *testing.T) {
	ledgerstate := setupDependencies(t)
	defer ledgerstate.Shutdown()

	outputsMetadata := []*OutputMetadata{
		{
			consumerCount: 0,
		},
		{
			consumerCount: 1,
		},
	}

	assert.False(t, ledgerstate.outputsUnspent(outputsMetadata))
	assert.True(t, ledgerstate.outputsUnspent(outputsMetadata[:1]))
}

func TestConsumedOutputs(t *testing.T) {
	ledgerstate := setupDependencies(t)
	defer ledgerstate.Shutdown()

	wallets := createWallets(2)
	input := generateOutput(ledgerstate, wallets[0].address, 0)

	// testing when storing the inputs
	tx, output := singleInputTransaction(ledgerstate, wallets[0], wallets[1], input)
	cachedInputs := ledgerstate.ConsumedOutputs(tx)
	inputs := cachedInputs.Unwrap()

	assert.Equal(t, input, inputs[0])

	cachedInputs.Release(true)

	// testing when not storing the inputs
	tx, _ = singleInputTransaction(ledgerstate, wallets[1], wallets[0], output)
	cachedInputs = ledgerstate.ConsumedOutputs(tx)
	inputs = cachedInputs.Unwrap()

	assert.Equal(t, nil, inputs[0])

	cachedInputs.Release(true)
}

func TestAllOutputsExist(t *testing.T) {
	ledgerstate := setupDependencies(t)
	defer ledgerstate.Shutdown()

	wallets := createWallets(2)
	input := generateOutput(ledgerstate, wallets[0].address, 0)

	// testing when storing the inputs
	tx, output := singleInputTransaction(ledgerstate, wallets[0], wallets[1], input)
	cachedInputs := ledgerstate.ConsumedOutputs(tx)
	inputs := cachedInputs.Unwrap()

	assert.True(t, ledgerstate.allOutputsExist(inputs))

	cachedInputs.Release()

	// testing when not storing the inputs
	tx, _ = singleInputTransaction(ledgerstate, wallets[1], wallets[0], output)
	cachedInputs = ledgerstate.ConsumedOutputs(tx)
	inputs = cachedInputs.Unwrap()

	assert.False(t, ledgerstate.allOutputsExist(inputs))

	cachedInputs.Release()
}

func TestTransactionBalancesValid(t *testing.T) {
	ledgerstate := setupDependencies(t)
	defer ledgerstate.Shutdown()

	wallets := createWallets(2)

	// region COLORED COINS TESTS //////////////////////////////////////////////////////////////////////////////////////

	iColored1 := NewSigLockedColoredOutput(NewColoredBalances(map[Color]uint64{
		ColorIOTA: 1337,
		color1:    20,
		color2:    30,
	}), wallets[0].address)

	oColored1 := NewSigLockedColoredOutput(NewColoredBalances(map[Color]uint64{
		ColorIOTA: 1297,
		color1:    10,
		color2:    10,
		ColorMint: 70,
	}), wallets[1].address)

	assert.True(t, TransactionBalancesValid(Outputs{iColored1}, Outputs{oColored1}))

	iColored2 := NewSigLockedColoredOutput(NewColoredBalances(map[Color]uint64{
		ColorIOTA: 1337,
		color1:    20,
		color2:    30,
	}), wallets[0].address)

	oColored2 := NewSigLockedColoredOutput(NewColoredBalances(map[Color]uint64{
		ColorIOTA: 1357,
		color1:    10,
		color2:    10,
		ColorMint: 10,
	}), wallets[1].address)

	assert.True(t, TransactionBalancesValid(Outputs{iColored2}, Outputs{oColored2}))

	iColored3 := NewSigLockedColoredOutput(NewColoredBalances(map[Color]uint64{
		color1: 1337,
	}), wallets[0].address)

	oColored3 := NewSigLockedColoredOutput(NewColoredBalances(map[Color]uint64{
		ColorMint: 1337,
	}), wallets[1].address)

	assert.True(t, TransactionBalancesValid(Outputs{iColored3}, Outputs{oColored3}))

	// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////

	i1 := NewSigLockedSingleOutput(100, wallets[0].address)
	i2 := NewSigLockedSingleOutput(100, wallets[0].address)

	// testing happy case
	o := NewSigLockedSingleOutput(200, wallets[1].address)

	assert.True(t, TransactionBalancesValid(Outputs{i1, i2}, Outputs{o}))

	// testing creating 1 iota out of thin air
	i2 = NewSigLockedSingleOutput(99, wallets[0].address)

	assert.False(t, TransactionBalancesValid(Outputs{i1, i2}, Outputs{o}))

	// testing burning 1 iota
	i2 = NewSigLockedSingleOutput(101, wallets[0].address)

	assert.False(t, TransactionBalancesValid(Outputs{i1, i2}, Outputs{o}))

	// testing unit64 overflow
	i2 = NewSigLockedSingleOutput(math.MaxUint64, wallets[0].address)

	assert.False(t, TransactionBalancesValid(Outputs{i1, i2}, Outputs{o}))
}

func TestUnlockBlocksValid(t *testing.T) {
	ledgerstate := setupDependencies(t)
	defer ledgerstate.Shutdown()

	wallets := createWallets(2)

	input := generateOutput(ledgerstate, wallets[0].address, 0)

	// testing valid signature
	tx, _ := singleInputTransaction(ledgerstate, wallets[0], wallets[1], input, gof.High)
	assert.True(t, UnlockBlocksValid(Outputs{input}, tx))

	// testing invalid signature
	tx, _ = singleInputTransaction(ledgerstate, wallets[1], wallets[0], input, gof.High)
	assert.False(t, UnlockBlocksValid(Outputs{input}, tx))
}

func TestAddressOutputMapping(t *testing.T) {
	ledgerstate := setupDependencies(t)
	defer ledgerstate.Shutdown()

	kp := ed25519.GenerateKeyPair()
	w := wallet{
		kp,
		NewED25519Address(kp.PublicKey),
	}
	ledgerstate.addressOutputMappingStorage.Store(NewAddressOutputMapping(w.address, EmptyOutputID)).Release()
	res := ledgerstate.CachedAddressOutputMapping(w.address)
	res.Release()
	assert.Equal(t, 1, len(res))
}

func TestUTXODAG_CheckTransaction(t *testing.T) {
	ledgerstate := setupDependencies(t)
	defer ledgerstate.Shutdown()

	w := genRandomWallet()
	governingWallet := genRandomWallet()
	alias := &AliasOutput{
		outputID:         randOutputID(),
		balances:         NewColoredBalances(map[Color]uint64{ColorIOTA: DustThresholdAliasOutputIOTA}),
		aliasAddress:     *randAliasAddress(),
		stateAddress:     w.address, // alias state controller is our wallet
		stateIndex:       10,
		governingAddress: governingWallet.address,
	}
	nextAlias := alias.NewAliasOutputNext(false)
	toBeConsumedExtended := NewExtendedLockedOutput(map[Color]uint64{ColorIOTA: 1}, alias.GetAliasAddress())
	toBeConsumedExtended.SetID(randOutputID())
	inputs := NewOutputs(alias, toBeConsumedExtended)
	// book manually the outputs into utxoDAG
	for _, output := range inputs {
		// replace ColorMint color with unique color based on OutputID
		output = output.UpdateMintingColor()

		// store Output
		ledgerstate.outputStorage.Store(output).Release()

		// store OutputMetadata
		metadata := NewOutputMetadata(output.ID())
		metadata.SetBranchID(MasterBranchID)
		metadata.SetSolid(true)
		ledgerstate.outputMetadataStorage.Store(metadata).Release()
	}

	nextAliasBalance := alias.Balances().Map()
	// add 1 more iota from consumed extended output
	nextAliasBalance[ColorIOTA]++
	err := nextAlias.SetBalances(nextAliasBalance)
	assert.NoError(t, err)

	essence := NewTransactionEssence(0, time.Now(), identity.ID{}, identity.ID{}, NewInputs(toBeConsumedExtended.Input(), alias.Input()), NewOutputs(nextAlias))
	// which input index did the alias get?
	var aliasInputIndex uint16
	orderedInputs := make(Outputs, len(essence.Inputs()))
	for i, input := range essence.Inputs() {
		casted := input.(*UTXOInput)
		if casted.ReferencedOutputID() == alias.ID() {
			aliasInputIndex = uint16(i)
			orderedInputs[i] = alias
		}
		if casted.ReferencedOutputID() == toBeConsumedExtended.ID() {
			orderedInputs[i] = toBeConsumedExtended
		}
	}

	t.Run("CASE: Happy path", func(t *testing.T) {
		// create mapping from outputID to unlockBlock
		inputToUnlockMapping := make(map[OutputID]UnlockBlock)
		inputToUnlockMapping[alias.ID()] = NewSignatureUnlockBlock(w.sign(essence))
		inputToUnlockMapping[toBeConsumedExtended.ID()] = NewAliasUnlockBlock(aliasInputIndex)

		// fill unlock blocks
		unlocks := make(UnlockBlocks, len(essence.Inputs()))
		for i, input := range essence.Inputs() {
			unlocks[i] = inputToUnlockMapping[input.(*UTXOInput).ReferencedOutputID()]
		}

		tx := NewTransaction(essence, unlocks)

		bErr := ledgerstate.CheckTransaction(tx)
		assert.NoError(t, bErr)
	})

	t.Run("CASE: Tx not okay, wrong signature", func(t *testing.T) {
		// create mapping from outputID to unlockBlock
		inputToUnlockMapping := make(map[OutputID]UnlockBlock)
		inputToUnlockMapping[alias.ID()] = NewSignatureUnlockBlock(genRandomWallet().sign(essence))
		inputToUnlockMapping[toBeConsumedExtended.ID()] = NewAliasUnlockBlock(aliasInputIndex)

		// fill unlock blocks
		unlocks := make(UnlockBlocks, len(essence.Inputs()))
		for i, input := range essence.Inputs() {
			unlocks[i] = inputToUnlockMapping[input.(*UTXOInput).ReferencedOutputID()]
		}

		tx := NewTransaction(essence, unlocks)

		bErr := ledgerstate.CheckTransaction(tx)
		t.Log(bErr)
		assert.Error(t, bErr)
	})

	t.Run("CASE: Tx not okay, alias unlocked for governance", func(t *testing.T) {
		// tx alias output will be unlocked for governance
		nextAlias = alias.NewAliasOutputNext(true)
		essence.outputs = NewOutputs(nextAlias, NewSigLockedSingleOutput(1, randEd25119Address()))

		// create mapping from outputID to unlockBlock
		inputToUnlockMapping := make(map[OutputID]UnlockBlock)
		inputToUnlockMapping[alias.ID()] = NewSignatureUnlockBlock(governingWallet.sign(essence))
		inputToUnlockMapping[toBeConsumedExtended.ID()] = NewAliasUnlockBlock(aliasInputIndex)

		// fill unlock blocks
		unlocks := make(UnlockBlocks, len(essence.Inputs()))
		for i, input := range essence.Inputs() {
			unlocks[i] = inputToUnlockMapping[input.(*UTXOInput).ReferencedOutputID()]
		}

		tx := NewTransaction(essence, unlocks)

		bErr := ledgerstate.CheckTransaction(tx)
		t.Log(bErr)
		assert.Error(t, bErr)
	})
}

func setupDependencies(t *testing.T) *Ledgerstate {
	ledgerstate := New(CacheTimeProvider(database.NewCacheTimeProvider(0)))
	err := ledgerstate.Prune()
	require.NoError(t, err)

	return ledgerstate
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
	wallets := make([]wallet, n)
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
	return NewED25519Signature(w.publicKey(), w.privateKey().Sign(txEssence.Bytes()))
}

func (w wallet) unlockBlocks(txEssence *TransactionEssence) []UnlockBlock {
	unlockBlock := NewSignatureUnlockBlock(w.sign(txEssence))
	unlockBlocks := make([]UnlockBlock, len(txEssence.inputs))
	for i := range txEssence.inputs {
		unlockBlocks[i] = unlockBlock
	}
	return unlockBlocks
}

func generateOutput(ledgerstate *Ledgerstate, address Address, index uint16) *SigLockedSingleOutput {
	output := NewSigLockedSingleOutput(100, address)
	output.SetID(NewOutputID(GenesisTransactionID, index))
	ledgerstate.outputStorage.Store(output).Release()

	// store OutputMetadata
	metadata := NewOutputMetadata(output.ID())
	metadata.SetBranchID(MasterBranchID)
	metadata.SetSolid(true)
	ledgerstate.outputMetadataStorage.Store(metadata).Release()

	return output
}

func generateOutputs(ledgerstate *Ledgerstate, address Address, branchIDs BranchIDs) (outputs []*SigLockedSingleOutput) {
	i := 0
	outputs = make([]*SigLockedSingleOutput, len(branchIDs))
	for branchID := range branchIDs {
		outputs[i] = NewSigLockedSingleOutput(100, address)
		outputs[i].SetID(NewOutputID(GenesisTransactionID, uint16(i)))
		ledgerstate.outputStorage.Store(outputs[i]).Release()

		// store OutputMetadata
		metadata := NewOutputMetadata(outputs[i].ID())
		metadata.SetBranchID(branchID)
		metadata.SetSolid(true)
		ledgerstate.outputMetadataStorage.Store(metadata).Release()
		i++
	}

	return
}

func singleInputTransaction(ledgerstate *Ledgerstate, a, b wallet, outputToSpend *SigLockedSingleOutput, optionalGradeOfFinality ...gof.GradeOfFinality) (*Transaction, *SigLockedSingleOutput) {
	input := NewUTXOInput(outputToSpend.ID())
	output := NewSigLockedSingleOutput(100, b.address)

	txEssence := NewTransactionEssence(0, time.Now(), identity.ID{}, identity.ID{}, NewInputs(input), NewOutputs(output))

	tx := NewTransaction(txEssence, a.unlockBlocks(txEssence))

	// store TransactionMetadata
	transactionMetadata := NewTransactionMetadata(tx.ID())
	transactionMetadata.SetSolid(true)
	transactionMetadata.SetBranchID(MasterBranchID)

	if len(optionalGradeOfFinality) >= 1 {
		transactionMetadata.SetGradeOfFinality(optionalGradeOfFinality[0])
	} else {
		transactionMetadata.SetGradeOfFinality(gof.Low)
	}

	cachedTransactionMetadata := &CachedTransactionMetadata{CachedObject: ledgerstate.transactionMetadataStorage.ComputeIfAbsent(tx.ID().Bytes(), func(key []byte) objectstorage.StorableObject {
		transactionMetadata.Persist()
		transactionMetadata.SetModified()
		return transactionMetadata
	})}
	defer cachedTransactionMetadata.Release()

	ledgerstate.transactionStorage.Store(tx).Release()

	return tx, output
}

func multipleInputsTransaction(ledgerstate *Ledgerstate, a, b wallet, outputsToSpend []*SigLockedSingleOutput, optionalGradeOfFinality ...gof.GradeOfFinality) *Transaction {
	inputs := make(Inputs, len(outputsToSpend))
	branchIDs := make(BranchIDs, len(outputsToSpend))
	for i, outputToSpend := range outputsToSpend {
		inputs[i] = NewUTXOInput(outputToSpend.ID())
		ledgerstate.CachedOutputMetadata(outputToSpend.ID()).Consume(func(outputMetadata *OutputMetadata) {
			branchIDs[outputMetadata.BranchID()] = types.Void
		})
	}

	output := NewSigLockedSingleOutput(100, b.address)

	txEssence := NewTransactionEssence(0, time.Now(), identity.ID{}, identity.ID{}, inputs, NewOutputs(output))

	tx := NewTransaction(txEssence, a.unlockBlocks(txEssence))

	// store aggreagated branch
	resolvedConflictBranchIDs, _ := ledgerstate.ResolveConflictBranchIDs(branchIDs)
	branchID := ledgerstate.AggregateConflictBranchesID(resolvedConflictBranchIDs)

	// store TransactionMetadata
	transactionMetadata := NewTransactionMetadata(tx.ID())
	transactionMetadata.SetSolid(true)
	transactionMetadata.SetBranchID(branchID)
	if len(optionalGradeOfFinality) >= 1 {
		transactionMetadata.SetGradeOfFinality(optionalGradeOfFinality[0])
	} else {
		transactionMetadata.SetGradeOfFinality(gof.Low)
	}

	cachedTransactionMetadata := &CachedTransactionMetadata{CachedObject: ledgerstate.transactionMetadataStorage.ComputeIfAbsent(tx.ID().Bytes(), func(key []byte) objectstorage.StorableObject {
		transactionMetadata.Persist()
		transactionMetadata.SetModified()
		return transactionMetadata
	})}
	defer cachedTransactionMetadata.Release()

	ledgerstate.transactionStorage.Store(tx).Release()

	return tx
}

func buildTransaction(_ *Ledgerstate, a, b wallet, outputsToSpend []*SigLockedSingleOutput) *Transaction {
	inputs := make(Inputs, len(outputsToSpend))
	sum := uint64(0)
	for i, outputToSpend := range outputsToSpend {
		inputs[i] = NewUTXOInput(outputToSpend.ID())
		outputToSpend.Balances().ForEach(func(color Color, balance uint64) bool {
			if color == ColorIOTA {
				sum += balance
			}

			return true
		})
	}

	output := NewSigLockedSingleOutput(sum, b.address)

	txEssence := NewTransactionEssence(0, time.Now(), identity.ID{}, identity.ID{}, inputs, NewOutputs(output))

	tx := NewTransaction(txEssence, a.unlockBlocks(txEssence))

	return tx
}
