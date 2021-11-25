package ledgerstate

import (
	"bytes"
	"testing"
	"time"

	"github.com/iotaledger/hive.go/identity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

/*
func TestLedgerstate_MergeToMaster(t *testing.T) {
	ledgerstate := setupDependencies(t)
	defer ledgerstate.Shutdown()

	manaPledgeID := identity.GenerateIdentity().ID()
	wallets := make(map[string]wallet)
	outputs := make(map[string]Output)
	inputs := make(map[string]Input)
	transactions := make(map[string]*Transaction)
	branches := make(map[string]BranchID)

	setupScenario(t, wallets, outputs, ledgerstate, inputs, manaPledgeID, transactions, branches)

	// merge A to Master
	{
		err := ledgerstate.MergeToMaster(branches["A"])
		require.NoError(t, err)

		assertBranchID(t, ledgerstate, transactions["A"], MasterBranchID)
		assertBranchID(t, ledgerstate, transactions["B"], branches["B"])
		assertBranchID(t, ledgerstate, transactions["C"], branches["C"])
		assertBranchID(t, ledgerstate, transactions["D"], branches["D"])
		assertBranchID(t, ledgerstate, transactions["E"], branches["C"])
		assertBranchID(t, ledgerstate, transactions["F"], branches["F"])
		assertBranchID(t, ledgerstate, transactions["G"], branches["G"])
	}

	// merge C to Master
	{
		err := ledgerstate.MergeToMaster(branches["C"])
		require.NoError(t, err)

		assertBranchID(t, ledgerstate, transactions["A"], MasterBranchID)
		assertBranchID(t, ledgerstate, transactions["B"], branches["B"])
		assertBranchID(t, ledgerstate, transactions["C"], MasterBranchID)
		assertBranchID(t, ledgerstate, transactions["D"], branches["D"])
		assertBranchID(t, ledgerstate, transactions["E"], MasterBranchID)
		assertBranchID(t, ledgerstate, transactions["F"], branches["F"])
		assertBranchID(t, ledgerstate, transactions["G"], branches["G"])
	}
}

func TestLedgerstate_MergeToMasterLeaf(t *testing.T) {
	ledgerstate := setupDependencies(t)
	defer ledgerstate.Shutdown()

	manaPledgeID := identity.GenerateIdentity().ID()
	wallets := make(map[string]wallet)
	outputs := make(map[string]Output)
	inputs := make(map[string]Input)
	transactions := make(map[string]*Transaction)
	branches := make(map[string]BranchID)

	setupScenario(t, wallets, outputs, ledgerstate, inputs, manaPledgeID, transactions, branches)

	// merge F to Master
	{
		err := ledgerstate.MergeToMaster(branches["F"])
		require.NoError(t, err)

		assertBranchID(t, ledgerstate, transactions["A"], MasterBranchID)
		assertBranchID(t, ledgerstate, transactions["B"], branches["B"])
		assertBranchID(t, ledgerstate, transactions["C"], MasterBranchID)
		assertBranchID(t, ledgerstate, transactions["D"], branches["D"])
		assertBranchID(t, ledgerstate, transactions["E"], MasterBranchID)
		assertBranchID(t, ledgerstate, transactions["F"], MasterBranchID)
		assertBranchID(t, ledgerstate, transactions["G"], branches["G"])
	}
}
*/

func assertBranchID(t *testing.T, ledgerstate *Ledgerstate, transaction *Transaction, branchID BranchID) {
	assert.True(t, ledgerstate.CachedTransactionMetadata(transaction.ID()).Consume(func(transactionMetadata *TransactionMetadata) {
		assert.Equal(t, branchID, transactionMetadata.BranchID())
	}))

	for _, output := range transaction.Essence().Outputs() {
		assert.True(t, ledgerstate.CachedOutputMetadata(output.ID()).Consume(func(outputMetadata *OutputMetadata) {
			assert.Equal(t, branchID, outputMetadata.BranchID())
		}))
	}
}

func setupScenario(t *testing.T, wallets map[string]wallet, outputs map[string]Output, ledgerstate *Ledgerstate, inputs map[string]Input, manaPledgeID identity.ID, transactions map[string]*Transaction, branches map[string]BranchID) {
	// create genesis outputs
	{
		wallets["GENESIS_1"] = createWallets(1)[0]
		outputs["GENESIS_1"] = generateOutput(ledgerstate, wallets["GENESIS_1"].address, 0)
		inputs["GENESIS_1"] = NewUTXOInput(outputs["GENESIS_1"].ID())

		wallets["GENESIS_2"] = createWallets(1)[0]
		outputs["GENESIS_2"] = generateOutput(ledgerstate, wallets["GENESIS_2"].address, 1)
		inputs["GENESIS_2"] = NewUTXOInput(outputs["GENESIS_2"].ID())
	}

	// issue Transaction A
	{
		wallets["A"] = createWallets(1)[0]
		outputs["A"] = NewSigLockedSingleOutput(100, wallets["A"].address)

		transactionEssence := NewTransactionEssence(0, time.Now(), manaPledgeID, manaPledgeID,
			NewInputs(inputs["GENESIS_1"]),
			NewOutputs(outputs["A"]),
		)

		transactions["A"] = NewTransaction(transactionEssence, []UnlockBlock{
			NewSignatureUnlockBlock(
				NewED25519Signature(
					wallets["GENESIS_1"].keyPair.PublicKey,
					wallets["GENESIS_1"].keyPair.PrivateKey.Sign(transactionEssence.Bytes()),
				),
			),
		})

		branches["A"] = NewBranchID(transactions["A"].ID())
		RegisterBranchIDAlias(branches["A"], "BranchA")

		outputs["A"].SetID(NewOutputID(transactions["A"].ID(), 0))
		inputs["A"] = NewUTXOInput(outputs["A"].ID())

		_, err := ledgerstate.BookTransaction(transactions["A"])
		require.NoError(t, err)

		assert.True(t, ledgerstate.CachedTransactionMetadata(transactions["A"].ID()).Consume(func(transactionMetadata *TransactionMetadata) {
			assert.Equal(t, MasterBranchID, transactionMetadata.BranchID())
		}))
	}

	// issue Transaction B (conflicting with A)
	{
		wallets["B"] = createWallets(1)[0]
		outputs["B"] = NewSigLockedSingleOutput(100, wallets["B"].address)

		transactionEssence := NewTransactionEssence(0, time.Now(), manaPledgeID, manaPledgeID,
			NewInputs(inputs["GENESIS_1"]),
			NewOutputs(outputs["B"]),
		)

		transactions["B"] = NewTransaction(transactionEssence, []UnlockBlock{
			NewSignatureUnlockBlock(
				NewED25519Signature(
					wallets["GENESIS_1"].keyPair.PublicKey,
					wallets["GENESIS_1"].keyPair.PrivateKey.Sign(transactionEssence.Bytes()),
				),
			),
		})

		branches["B"] = NewBranchID(transactions["B"].ID())
		RegisterBranchIDAlias(branches["B"], "BranchB")

		outputs["B"].SetID(NewOutputID(transactions["B"].ID(), 0))
		inputs["B"] = NewUTXOInput(outputs["B"].ID())

		_, err := ledgerstate.BookTransaction(transactions["B"])
		require.NoError(t, err)

		assert.True(t, ledgerstate.CachedTransactionMetadata(transactions["A"].ID()).Consume(func(transactionMetadata *TransactionMetadata) {
			assert.Equal(t, branches["A"], transactionMetadata.BranchID())
		}))
		assert.True(t, ledgerstate.CachedTransactionMetadata(transactions["B"].ID()).Consume(func(transactionMetadata *TransactionMetadata) {
			assert.Equal(t, branches["B"], transactionMetadata.BranchID())
		}))
	}

	// issue Transaction C
	{
		wallets["C"] = createWallets(1)[0]
		outputs["C"] = NewSigLockedSingleOutput(100, wallets["C"].address)

		transactionEssence := NewTransactionEssence(0, time.Now(), manaPledgeID, manaPledgeID,
			NewInputs(inputs["GENESIS_2"]),
			NewOutputs(outputs["C"]),
		)

		transactions["C"] = NewTransaction(transactionEssence, []UnlockBlock{
			NewSignatureUnlockBlock(
				NewED25519Signature(
					wallets["GENESIS_2"].keyPair.PublicKey,
					wallets["GENESIS_2"].keyPair.PrivateKey.Sign(transactionEssence.Bytes()),
				),
			),
		})

		branches["C"] = NewBranchID(transactions["C"].ID())
		RegisterBranchIDAlias(branches["C"], "BranchC")

		outputs["C"].SetID(NewOutputID(transactions["C"].ID(), 0))
		inputs["C"] = NewUTXOInput(outputs["C"].ID())

		_, err := ledgerstate.BookTransaction(transactions["C"])
		require.NoError(t, err)

		assert.True(t, ledgerstate.CachedTransactionMetadata(transactions["A"].ID()).Consume(func(transactionMetadata *TransactionMetadata) {
			assert.Equal(t, branches["A"], transactionMetadata.BranchID())
		}))
		assert.True(t, ledgerstate.CachedTransactionMetadata(transactions["B"].ID()).Consume(func(transactionMetadata *TransactionMetadata) {
			assert.Equal(t, branches["B"], transactionMetadata.BranchID())
		}))
		assert.True(t, ledgerstate.CachedTransactionMetadata(transactions["C"].ID()).Consume(func(transactionMetadata *TransactionMetadata) {
			assert.Equal(t, MasterBranchID, transactionMetadata.BranchID())
		}))
	}

	// issue Transaction D (conflicting with C)
	{
		wallets["D"] = createWallets(1)[0]
		outputs["D"] = NewSigLockedSingleOutput(100, wallets["D"].address)

		transactionEssence := NewTransactionEssence(0, time.Now(), manaPledgeID, manaPledgeID,
			NewInputs(inputs["GENESIS_2"]),
			NewOutputs(outputs["D"]),
		)

		transactions["D"] = NewTransaction(transactionEssence, []UnlockBlock{
			NewSignatureUnlockBlock(
				NewED25519Signature(
					wallets["GENESIS_2"].keyPair.PublicKey,
					wallets["GENESIS_2"].keyPair.PrivateKey.Sign(transactionEssence.Bytes()),
				),
			),
		})

		branches["D"] = NewBranchID(transactions["D"].ID())
		RegisterBranchIDAlias(branches["D"], "BranchD")

		outputs["D"].SetID(NewOutputID(transactions["D"].ID(), 0))
		inputs["D"] = NewUTXOInput(outputs["D"].ID())

		_, err := ledgerstate.BookTransaction(transactions["D"])
		require.NoError(t, err)

		assert.True(t, ledgerstate.CachedTransactionMetadata(transactions["A"].ID()).Consume(func(transactionMetadata *TransactionMetadata) {
			assert.Equal(t, branches["A"], transactionMetadata.BranchID())
		}))
		assert.True(t, ledgerstate.CachedTransactionMetadata(transactions["B"].ID()).Consume(func(transactionMetadata *TransactionMetadata) {
			assert.Equal(t, branches["B"], transactionMetadata.BranchID())
		}))
		assert.True(t, ledgerstate.CachedTransactionMetadata(transactions["C"].ID()).Consume(func(transactionMetadata *TransactionMetadata) {
			assert.Equal(t, branches["C"], transactionMetadata.BranchID())
		}))
		assert.True(t, ledgerstate.CachedTransactionMetadata(transactions["D"].ID()).Consume(func(transactionMetadata *TransactionMetadata) {
			assert.Equal(t, branches["D"], transactionMetadata.BranchID())
		}))
	}

	// issue Transaction E (combining Branch A and C)
	{
		wallets["E"] = createWallets(1)[0]
		outputs["E"] = NewSigLockedSingleOutput(200, wallets["E"].address)

		transactionEssence := NewTransactionEssence(0, time.Now(), manaPledgeID, manaPledgeID,
			NewInputs(inputs["A"], inputs["C"]),
			NewOutputs(outputs["E"]),
		)

		unlockBlocks := []UnlockBlock{
			NewSignatureUnlockBlock(
				NewED25519Signature(
					wallets["A"].keyPair.PublicKey,
					wallets["A"].keyPair.PrivateKey.Sign(transactionEssence.Bytes()),
				),
			),
			NewSignatureUnlockBlock(
				NewED25519Signature(
					wallets["C"].keyPair.PublicKey,
					wallets["C"].keyPair.PrivateKey.Sign(transactionEssence.Bytes()),
				),
			),
		}

		if bytes.Compare(inputs["A"].Bytes(), inputs["C"].Bytes()) >= 0 {
			unlockBlocks[0], unlockBlocks[1] = unlockBlocks[1], unlockBlocks[0]
		}

		transactions["E"] = NewTransaction(transactionEssence, unlockBlocks)

		branches["A+C"] = NewAggregatedBranch(NewBranchIDs(
			branches["A"],
			branches["C"],
		)).ID()
		RegisterBranchIDAlias(branches["A+C"], "BranchA+C")

		outputs["E"].SetID(NewOutputID(transactions["E"].ID(), 0))
		inputs["E"] = NewUTXOInput(outputs["E"].ID())

		_, err := ledgerstate.BookTransaction(transactions["E"])
		require.NoError(t, err)

		assert.True(t, ledgerstate.CachedTransactionMetadata(transactions["A"].ID()).Consume(func(transactionMetadata *TransactionMetadata) {
			assert.Equal(t, branches["A"], transactionMetadata.BranchID())
		}))
		assert.True(t, ledgerstate.CachedTransactionMetadata(transactions["B"].ID()).Consume(func(transactionMetadata *TransactionMetadata) {
			assert.Equal(t, branches["B"], transactionMetadata.BranchID())
		}))
		assert.True(t, ledgerstate.CachedTransactionMetadata(transactions["C"].ID()).Consume(func(transactionMetadata *TransactionMetadata) {
			assert.Equal(t, branches["C"], transactionMetadata.BranchID())
		}))
		assert.True(t, ledgerstate.CachedTransactionMetadata(transactions["D"].ID()).Consume(func(transactionMetadata *TransactionMetadata) {
			assert.Equal(t, branches["D"], transactionMetadata.BranchID())
		}))
		assert.True(t, ledgerstate.CachedTransactionMetadata(transactions["E"].ID()).Consume(func(transactionMetadata *TransactionMetadata) {
			assert.Equal(t, NewAggregatedBranch(NewBranchIDs(
				branches["A"],
				branches["C"],
			)).ID(), transactionMetadata.BranchID())
		}))
	}

	// issue Transaction F
	{
		wallets["F"] = createWallets(1)[0]
		outputs["F"] = NewSigLockedSingleOutput(200, wallets["F"].address)

		transactionEssence := NewTransactionEssence(0, time.Now(), manaPledgeID, manaPledgeID,
			NewInputs(inputs["E"]),
			NewOutputs(outputs["F"]),
		)

		transactions["F"] = NewTransaction(transactionEssence, []UnlockBlock{
			NewSignatureUnlockBlock(
				NewED25519Signature(
					wallets["E"].keyPair.PublicKey,
					wallets["E"].keyPair.PrivateKey.Sign(transactionEssence.Bytes()),
				),
			),
		})

		branches["F"] = NewBranchID(transactions["F"].ID())
		RegisterBranchIDAlias(branches["F"], "BranchF")

		outputs["F"].SetID(NewOutputID(transactions["F"].ID(), 0))
		inputs["F"] = NewUTXOInput(outputs["F"].ID())

		_, err := ledgerstate.BookTransaction(transactions["F"])
		require.NoError(t, err)

		assert.True(t, ledgerstate.CachedTransactionMetadata(transactions["A"].ID()).Consume(func(transactionMetadata *TransactionMetadata) {
			assert.Equal(t, branches["A"], transactionMetadata.BranchID())
		}))
		assert.True(t, ledgerstate.CachedTransactionMetadata(transactions["B"].ID()).Consume(func(transactionMetadata *TransactionMetadata) {
			assert.Equal(t, branches["B"], transactionMetadata.BranchID())
		}))
		assert.True(t, ledgerstate.CachedTransactionMetadata(transactions["C"].ID()).Consume(func(transactionMetadata *TransactionMetadata) {
			assert.Equal(t, branches["C"], transactionMetadata.BranchID())
		}))
		assert.True(t, ledgerstate.CachedTransactionMetadata(transactions["D"].ID()).Consume(func(transactionMetadata *TransactionMetadata) {
			assert.Equal(t, branches["D"], transactionMetadata.BranchID())
		}))
		assert.True(t, ledgerstate.CachedTransactionMetadata(transactions["E"].ID()).Consume(func(transactionMetadata *TransactionMetadata) {
			assert.Equal(t, branches["A+C"], transactionMetadata.BranchID())
		}))
		assert.True(t, ledgerstate.CachedTransactionMetadata(transactions["F"].ID()).Consume(func(transactionMetadata *TransactionMetadata) {
			assert.Equal(t, branches["A+C"], transactionMetadata.BranchID())
		}))
	}

	// issue Transaction G
	{
		wallets["G"] = createWallets(1)[0]
		outputs["G"] = NewSigLockedSingleOutput(200, wallets["G"].address)

		transactionEssence := NewTransactionEssence(0, time.Now(), manaPledgeID, manaPledgeID,
			NewInputs(inputs["E"]),
			NewOutputs(outputs["G"]),
		)

		transactions["G"] = NewTransaction(transactionEssence, []UnlockBlock{
			NewSignatureUnlockBlock(
				NewED25519Signature(
					wallets["E"].keyPair.PublicKey,
					wallets["E"].keyPair.PrivateKey.Sign(transactionEssence.Bytes()),
				),
			),
		})

		branches["G"] = NewBranchID(transactions["G"].ID())
		RegisterBranchIDAlias(branches["G"], "BranchG")

		outputs["G"].SetID(NewOutputID(transactions["G"].ID(), 0))
		inputs["G"] = NewUTXOInput(outputs["G"].ID())

		_, err := ledgerstate.BookTransaction(transactions["G"])
		require.NoError(t, err)

		assert.True(t, ledgerstate.CachedTransactionMetadata(transactions["A"].ID()).Consume(func(transactionMetadata *TransactionMetadata) {
			assert.Equal(t, branches["A"], transactionMetadata.BranchID())
		}))
		assert.True(t, ledgerstate.CachedTransactionMetadata(transactions["B"].ID()).Consume(func(transactionMetadata *TransactionMetadata) {
			assert.Equal(t, branches["B"], transactionMetadata.BranchID())
		}))
		assert.True(t, ledgerstate.CachedTransactionMetadata(transactions["C"].ID()).Consume(func(transactionMetadata *TransactionMetadata) {
			assert.Equal(t, branches["C"], transactionMetadata.BranchID())
		}))
		assert.True(t, ledgerstate.CachedTransactionMetadata(transactions["D"].ID()).Consume(func(transactionMetadata *TransactionMetadata) {
			assert.Equal(t, branches["D"], transactionMetadata.BranchID())
		}))
		assert.True(t, ledgerstate.CachedTransactionMetadata(transactions["E"].ID()).Consume(func(transactionMetadata *TransactionMetadata) {
			assert.Equal(t, branches["A+C"], transactionMetadata.BranchID())
		}))
		assert.True(t, ledgerstate.CachedTransactionMetadata(transactions["F"].ID()).Consume(func(transactionMetadata *TransactionMetadata) {
			assert.Equal(t, branches["F"], transactionMetadata.BranchID())
		}))
		assert.True(t, ledgerstate.CachedTransactionMetadata(transactions["G"].ID()).Consume(func(transactionMetadata *TransactionMetadata) {
			assert.Equal(t, branches["G"], transactionMetadata.BranchID())
		}))
	}
}
