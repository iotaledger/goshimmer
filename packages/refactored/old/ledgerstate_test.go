package old

import (
	"bytes"
	"fmt"
	"testing"
	"time"

	"github.com/iotaledger/hive.go/identity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	branchdag2 "github.com/iotaledger/goshimmer/packages/refactored/ledger/branchdag"

	"github.com/iotaledger/goshimmer/packages/refactored/branchdag"
	"github.com/iotaledger/goshimmer/packages/refactored/txvm"
	utxo2 "github.com/iotaledger/goshimmer/packages/refactored/types/utxo"
)

func TestLedgerstate_SetBranchConfirmed(t *testing.T) {
	ledgerstate := setupDependencies(t)
	defer ledgerstate.Shutdown()

	manaPledgeID := identity.GenerateIdentity().ID()
	wallets := make(map[string]txvm.wallet)
	outputs := make(map[string]utxo2.Output)
	inputs := make(map[string]utxo2.Input)
	transactions := make(map[string]utxo2.Transaction)
	branches := make(map[string]branchdag.BranchIDs)

	setupScenarioBottomLayer(t, wallets, outputs, ledgerstate, inputs, manaPledgeID, transactions, branches)

	// Mark A as Confirmed
	{
		require.True(t, ledgerstate.BranchDAG.SetBranchConfirmed(getSingleBranch(branches, "A")))

		assertBranchIDs(t, ledgerstate, transactions["A"], branches["A"])
		assertBranchIDs(t, ledgerstate, transactions["B"], branches["B"])
		assertBranchIDs(t, ledgerstate, transactions["C"], branches["C"])
		assertBranchIDs(t, ledgerstate, transactions["D"], branches["D"])
		assertBranchIDs(t, ledgerstate, transactions["H"], branches["H"])
		assertBranchIDs(t, ledgerstate, transactions["I"], branches["I"])

		assert.Equal(t, branchdag.Confirmed, ledgerstate.BranchDAG.InclusionState(branches["A"]))
		assert.Equal(t, branchdag.Pending, ledgerstate.BranchDAG.InclusionState(branches["C"]))
	}

	setupScenarioMiddleLayer(t, wallets, outputs, ledgerstate, inputs, manaPledgeID, transactions, branches)

	// When creating the middle layer the new transaction E should be booked only under its Pending parent C
	{
		assertBranchIDs(t, ledgerstate, transactions["A"], branches["A"])
		assertBranchIDs(t, ledgerstate, transactions["B"], branches["B"])
		assertBranchIDs(t, ledgerstate, transactions["C"], branches["C"])
		assertBranchIDs(t, ledgerstate, transactions["D"], branches["D"])
		assertBranchIDs(t, ledgerstate, transactions["H"], branches["H"])
		assertBranchIDs(t, ledgerstate, transactions["I"], branches["I"])
		assertBranchIDs(t, ledgerstate, transactions["E"], branches["C"])
	}

	setupScenarioTopLayer1(t, wallets, outputs, ledgerstate, inputs, manaPledgeID, transactions, branches)

	// When creating the first transaction of top layer it should be booked under the Pending parent C
	{
		assertBranchIDs(t, ledgerstate, transactions["A"], branches["A"])
		assertBranchIDs(t, ledgerstate, transactions["B"], branches["B"])
		assertBranchIDs(t, ledgerstate, transactions["C"], branches["C"])
		assertBranchIDs(t, ledgerstate, transactions["D"], branches["D"])
		assertBranchIDs(t, ledgerstate, transactions["H"], branches["H"])
		assertBranchIDs(t, ledgerstate, transactions["I"], branches["I"])
		assertBranchIDs(t, ledgerstate, transactions["E"], branches["C"])

		// Branches F & G are spawned by the fork of G
		assertBranchIDs(t, ledgerstate, transactions["F"], branches["C"])
	}

	setupScenarioTopLayer2(t, wallets, outputs, ledgerstate, inputs, manaPledgeID, transactions, branches)

	// When creating the conflicting TX of the top layer branches F & G are spawned by the fork of G
	{
		assertBranchIDs(t, ledgerstate, transactions["A"], branches["A"])
		assertBranchIDs(t, ledgerstate, transactions["B"], branches["B"])
		assertBranchIDs(t, ledgerstate, transactions["C"], branches["C"])
		assertBranchIDs(t, ledgerstate, transactions["D"], branches["D"])
		assertBranchIDs(t, ledgerstate, transactions["H"], branches["H"])
		assertBranchIDs(t, ledgerstate, transactions["I"], branches["I"])
		assertBranchIDs(t, ledgerstate, transactions["E"], branches["C"])

		// Branches F & G are spawned by the fork of G
		assertBranchIDs(t, ledgerstate, transactions["F"], branches["F"])
		assertBranchIDs(t, ledgerstate, transactions["G"], branches["G"])

		ledgerstate.BranchDAG.Branch(getSingleBranch(branches, "F")).Consume(func(branch *branchdag.Branch) {
			assert.Equal(t, branches["C"], branch.Parents())
		})

		ledgerstate.BranchDAG.Branch(getSingleBranch(branches, "G")).Consume(func(branch *branchdag.Branch) {
			assert.Equal(t, branches["C"], branch.Parents())
		})
	}

	require.True(t, ledgerstate.BranchDAG.SetBranchConfirmed(getSingleBranch(branches, "D")))

	setupScenarioTopTopLayer(t, wallets, outputs, ledgerstate, inputs, manaPledgeID, transactions, branches)

	// TX L aggregates a child (G) of a Rejected branch (C) and a pending branch H, resulting in G+H
	{
		assertBranchIDs(t, ledgerstate, transactions["A"], branches["A"])
		assertBranchIDs(t, ledgerstate, transactions["B"], branches["B"])
		assertBranchIDs(t, ledgerstate, transactions["C"], branches["C"])
		assertBranchIDs(t, ledgerstate, transactions["D"], branches["D"])
		assertBranchIDs(t, ledgerstate, transactions["H"], branches["H"])
		assertBranchIDs(t, ledgerstate, transactions["I"], branches["I"])
		assertBranchIDs(t, ledgerstate, transactions["E"], branches["C"])
		assertBranchIDs(t, ledgerstate, transactions["F"], branches["F"])
		assertBranchIDs(t, ledgerstate, transactions["G"], branches["G"])
		assertBranchIDs(t, ledgerstate, transactions["L"], branches["G+H"])

		assert.Equal(t, branchdag.Confirmed, ledgerstate.BranchDAG.InclusionState(branches["D"]))
		assert.Equal(t, branchdag.Rejected, ledgerstate.BranchDAG.InclusionState(branches["C"]))
		assert.Equal(t, branchdag.Rejected, ledgerstate.BranchDAG.InclusionState(branches["F"]))
		assert.Equal(t, branchdag.Rejected, ledgerstate.BranchDAG.InclusionState(branches["G"]))
		assert.Equal(t, branchdag.Rejected, ledgerstate.BranchDAG.InclusionState(branches["G+H"]))
		assert.Equal(t, branchdag.Pending, ledgerstate.BranchDAG.InclusionState(branches["H"]))
		assert.Equal(t, branchdag.Pending, ledgerstate.BranchDAG.InclusionState(branches["I"]))
	}

	require.True(t, ledgerstate.BranchDAG.SetBranchConfirmed(getSingleBranch(branches, "H")))

	setupScenarioTopTopTopLayer(t, wallets, outputs, ledgerstate, inputs, manaPledgeID, transactions)

	// The new TX M should be now booked under G, as the branch G+H got H confirmed, transforming it into
	// a Branch again: just G because we don't propagate H further.
	{
		assertBranchIDs(t, ledgerstate, transactions["A"], branches["A"])
		assertBranchIDs(t, ledgerstate, transactions["B"], branches["B"])
		assertBranchIDs(t, ledgerstate, transactions["C"], branches["C"])
		assertBranchIDs(t, ledgerstate, transactions["D"], branches["D"])
		assertBranchIDs(t, ledgerstate, transactions["H"], branches["H"])
		assertBranchIDs(t, ledgerstate, transactions["I"], branches["I"])
		assertBranchIDs(t, ledgerstate, transactions["E"], branches["C"])
		assertBranchIDs(t, ledgerstate, transactions["F"], branches["F"])
		assertBranchIDs(t, ledgerstate, transactions["G"], branches["G"])
		assertBranchIDs(t, ledgerstate, transactions["L"], branches["G+H"])
		assertBranchIDs(t, ledgerstate, transactions["M"], branches["G"])

		assert.Equal(t, branchdag.Confirmed, ledgerstate.BranchDAG.InclusionState(branches["D"]))
		assert.Equal(t, branchdag.Rejected, ledgerstate.BranchDAG.InclusionState(branches["C"]))
		assert.Equal(t, branchdag.Rejected, ledgerstate.BranchDAG.InclusionState(branches["F"]))
		assert.Equal(t, branchdag.Rejected, ledgerstate.BranchDAG.InclusionState(branches["G"]))
		assert.Equal(t, branchdag.Rejected, ledgerstate.BranchDAG.InclusionState(branches["G+H"]))
		assert.Equal(t, branchdag.Confirmed, ledgerstate.BranchDAG.InclusionState(branches["H"]))
		assert.Equal(t, branchdag.Rejected, ledgerstate.BranchDAG.InclusionState(branches["I"]))
	}
}

func assertBranchIDs(t *testing.T, ledgerstate *Ledger, transaction *Transaction, expectedBranchIDs branchdag.BranchIDs) {
	assert.True(t, ledgerstate.CachedTransactionMetadata(transaction.ID()).Consume(func(transactionMetadata *TransactionMetadata) {
		assert.Equal(t, expectedBranchIDs, transactionMetadata.BranchIDs(), transactionMetadata.String(), expectedBranchIDs.String())
	}))

	for _, output := range transaction.Essence().Outputs() {
		assert.True(t, ledgerstate.CachedOutputMetadata(output.ID()).Consume(func(outputMetadata *OutputMetadata) {
			assert.Equal(t, expectedBranchIDs, outputMetadata.BranchIDs(), outputMetadata.String(), expectedBranchIDs.String())
		}))
	}
}

func setupScenarioBottomLayer(t *testing.T, wallets map[string]txvm.wallet, outputs map[string]utxo2.Output, ledgerstate *Ledger, inputs map[string]utxo2.Input, manaPledgeID identity.ID, transactions map[string]utxo2.Transaction, branches map[string]branchdag.BranchIDs) {
	// create genesis outputs
	{
		wallets["GENESIS_1"] = txvm.createWallets(1)[0]
		outputs["GENESIS_1"] = generateOutput(ledgerstate, wallets["GENESIS_1"].address, 0)
		inputs["GENESIS_1"] = devnetvm.NewUTXOInput(outputs["GENESIS_1"].ID())

		wallets["GENESIS_2"] = txvm.createWallets(1)[0]
		outputs["GENESIS_2"] = generateOutput(ledgerstate, wallets["GENESIS_2"].address, 1)
		inputs["GENESIS_2"] = NewUTXOInput(outputs["GENESIS_2"].ID())

		wallets["GENESIS_3"] = txvm.createWallets(1)[0]
		outputs["GENESIS_3"] = generateOutput(ledgerstate, wallets["GENESIS_3"].address, 2)
		inputs["GENESIS_3"] = NewUTXOInput(outputs["GENESIS_3"].ID())
	}

	// issue Transaction A
	{
		wallets["A"] = txvm.createWallets(1)[0]
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

		addBranchAndRegister(branches, transactions, "A")

		outputs["A"].SetID(NewOutputID(transactions["A"].ID(), 0))
		inputs["A"] = NewUTXOInput(outputs["A"].ID())

		_, err := ledgerstate.BookTransaction(transactions["A"])
		require.NoError(t, err)

		assert.True(t, ledgerstate.CachedTransactionMetadata(transactions["A"].ID()).Consume(func(transactionMetadata *TransactionMetadata) {
			assert.Equal(t, branchdag.NewBranchIDs(branchdag.MasterBranchID), transactionMetadata.BranchIDs())
		}))
	}

	// issue Transaction B (conflicting with A)
	{
		wallets["B"] = txvm.createWallets(1)[0]
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

		addBranchAndRegister(branches, transactions, "B")

		outputs["B"].SetID(NewOutputID(transactions["B"].ID(), 0))
		inputs["B"] = NewUTXOInput(outputs["B"].ID())

		_, err := ledgerstate.BookTransaction(transactions["B"])
		require.NoError(t, err)

		assert.True(t, ledgerstate.CachedTransactionMetadata(transactions["A"].ID()).Consume(func(transactionMetadata *TransactionMetadata) {
			assert.Equal(t, branches["A"], transactionMetadata.BranchIDs())
		}))
		assert.True(t, ledgerstate.CachedTransactionMetadata(transactions["B"].ID()).Consume(func(transactionMetadata *TransactionMetadata) {
			assert.Equal(t, branches["B"], transactionMetadata.BranchIDs())
		}))
	}

	// issue Transaction C
	{
		wallets["C"] = txvm.createWallets(1)[0]
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

		addBranchAndRegister(branches, transactions, "C")

		outputs["C"].SetID(NewOutputID(transactions["C"].ID(), 0))
		inputs["C"] = NewUTXOInput(outputs["C"].ID())

		_, err := ledgerstate.BookTransaction(transactions["C"])
		require.NoError(t, err)

		assert.True(t, ledgerstate.CachedTransactionMetadata(transactions["A"].ID()).Consume(func(transactionMetadata *TransactionMetadata) {
			assert.Equal(t, branches["A"], transactionMetadata.BranchIDs())
		}))
		assert.True(t, ledgerstate.CachedTransactionMetadata(transactions["B"].ID()).Consume(func(transactionMetadata *TransactionMetadata) {
			assert.Equal(t, branches["B"], transactionMetadata.BranchIDs())
		}))
		assert.True(t, ledgerstate.CachedTransactionMetadata(transactions["C"].ID()).Consume(func(transactionMetadata *TransactionMetadata) {
			assert.Equal(t, branchdag.NewBranchIDs(branchdag.MasterBranchID), transactionMetadata.BranchIDs())
		}))
	}

	// issue Transaction D (conflicting with C)
	{
		wallets["D"] = txvm.createWallets(1)[0]
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

		addBranchAndRegister(branches, transactions, "D")

		outputs["D"].SetID(NewOutputID(transactions["D"].ID(), 0))
		inputs["D"] = NewUTXOInput(outputs["D"].ID())

		_, err := ledgerstate.BookTransaction(transactions["D"])
		require.NoError(t, err)

		assert.True(t, ledgerstate.CachedTransactionMetadata(transactions["A"].ID()).Consume(func(transactionMetadata *TransactionMetadata) {
			assert.Equal(t, branches["A"], transactionMetadata.BranchIDs())
		}))
		assert.True(t, ledgerstate.CachedTransactionMetadata(transactions["B"].ID()).Consume(func(transactionMetadata *TransactionMetadata) {
			assert.Equal(t, branches["B"], transactionMetadata.BranchIDs())
		}))
		assert.True(t, ledgerstate.CachedTransactionMetadata(transactions["C"].ID()).Consume(func(transactionMetadata *TransactionMetadata) {
			assert.Equal(t, branches["C"], transactionMetadata.BranchIDs())
		}))
		assert.True(t, ledgerstate.CachedTransactionMetadata(transactions["D"].ID()).Consume(func(transactionMetadata *TransactionMetadata) {
			assert.Equal(t, branches["D"], transactionMetadata.BranchIDs())
		}))
	}

	// issue Transaction H
	{
		wallets["H"] = txvm.createWallets(1)[0]
		outputs["H"] = NewSigLockedSingleOutput(100, wallets["H"].address)

		transactionEssence := NewTransactionEssence(0, time.Now(), manaPledgeID, manaPledgeID,
			NewInputs(inputs["GENESIS_3"]),
			NewOutputs(outputs["H"]),
		)

		transactions["H"] = NewTransaction(transactionEssence, []UnlockBlock{
			NewSignatureUnlockBlock(
				NewED25519Signature(
					wallets["GENESIS_3"].keyPair.PublicKey,
					wallets["GENESIS_3"].keyPair.PrivateKey.Sign(transactionEssence.Bytes()),
				),
			),
		})

		addBranchAndRegister(branches, transactions, "H")

		outputs["H"].SetID(NewOutputID(transactions["H"].ID(), 0))
		inputs["H"] = NewUTXOInput(outputs["H"].ID())

		_, err := ledgerstate.BookTransaction(transactions["H"])
		require.NoError(t, err)

		assert.True(t, ledgerstate.CachedTransactionMetadata(transactions["A"].ID()).Consume(func(transactionMetadata *TransactionMetadata) {
			assert.Equal(t, branches["A"], transactionMetadata.BranchIDs())
		}))
		assert.True(t, ledgerstate.CachedTransactionMetadata(transactions["B"].ID()).Consume(func(transactionMetadata *TransactionMetadata) {
			assert.Equal(t, branches["B"], transactionMetadata.BranchIDs())
		}))
		assert.True(t, ledgerstate.CachedTransactionMetadata(transactions["C"].ID()).Consume(func(transactionMetadata *TransactionMetadata) {
			assert.Equal(t, branches["C"], transactionMetadata.BranchIDs())
		}))
		assert.True(t, ledgerstate.CachedTransactionMetadata(transactions["D"].ID()).Consume(func(transactionMetadata *TransactionMetadata) {
			assert.Equal(t, branches["D"], transactionMetadata.BranchIDs())
		}))
		assert.True(t, ledgerstate.CachedTransactionMetadata(transactions["H"].ID()).Consume(func(transactionMetadata *TransactionMetadata) {
			assert.Equal(t, branchdag.NewBranchIDs(branchdag.MasterBranchID), transactionMetadata.BranchIDs())
		}))
	}

	// issue Transaction I (conflicting with H)
	{
		wallets["I"] = txvm.createWallets(1)[0]
		outputs["I"] = NewSigLockedSingleOutput(100, wallets["I"].address)

		transactionEssence := NewTransactionEssence(0, time.Now(), manaPledgeID, manaPledgeID,
			NewInputs(inputs["GENESIS_3"]),
			NewOutputs(outputs["I"]),
		)

		transactions["I"] = NewTransaction(transactionEssence, []UnlockBlock{
			NewSignatureUnlockBlock(
				NewED25519Signature(
					wallets["GENESIS_3"].keyPair.PublicKey,
					wallets["GENESIS_3"].keyPair.PrivateKey.Sign(transactionEssence.Bytes()),
				),
			),
		})

		addBranchAndRegister(branches, transactions, "I")

		outputs["I"].SetID(NewOutputID(transactions["I"].ID(), 0))
		inputs["I"] = NewUTXOInput(outputs["I"].ID())

		_, err := ledgerstate.BookTransaction(transactions["I"])
		require.NoError(t, err)

		assert.True(t, ledgerstate.CachedTransactionMetadata(transactions["A"].ID()).Consume(func(transactionMetadata *TransactionMetadata) {
			assert.Equal(t, branches["A"], transactionMetadata.BranchIDs())
		}))
		assert.True(t, ledgerstate.CachedTransactionMetadata(transactions["B"].ID()).Consume(func(transactionMetadata *TransactionMetadata) {
			assert.Equal(t, branches["B"], transactionMetadata.BranchIDs())
		}))
		assert.True(t, ledgerstate.CachedTransactionMetadata(transactions["C"].ID()).Consume(func(transactionMetadata *TransactionMetadata) {
			assert.Equal(t, branches["C"], transactionMetadata.BranchIDs())
		}))
		assert.True(t, ledgerstate.CachedTransactionMetadata(transactions["D"].ID()).Consume(func(transactionMetadata *TransactionMetadata) {
			assert.Equal(t, branches["D"], transactionMetadata.BranchIDs())
		}))
		assert.True(t, ledgerstate.CachedTransactionMetadata(transactions["H"].ID()).Consume(func(transactionMetadata *TransactionMetadata) {
			assert.Equal(t, branches["H"], transactionMetadata.BranchIDs())
		}))
		assert.True(t, ledgerstate.CachedTransactionMetadata(transactions["I"].ID()).Consume(func(transactionMetadata *TransactionMetadata) {
			assert.Equal(t, branches["I"], transactionMetadata.BranchIDs())
		}))
	}
}

func setupScenarioMiddleLayer(t *testing.T, wallets map[string]txvm.wallet, outputs map[string]Output, ledgerstate *Ledger, inputs map[string]Input, manaPledgeID identity.ID, transactions map[string]*Transaction, branches map[string]branchdag.BranchIDs) {
	// issue Transaction E (combining Branch A and C)
	{
		wallets["E"] = txvm.createWallets(1)[0]
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

		branches["A+C"] = branches["A"].Clone().AddAll(branches["C"])

		outputs["E"].SetID(NewOutputID(transactions["E"].ID(), 0))
		inputs["E"] = NewUTXOInput(outputs["E"].ID())

		_, err := ledgerstate.BookTransaction(transactions["E"])
		require.NoError(t, err)
	}
}

func setupScenarioTopLayerGeneric(t *testing.T, wallets map[string]txvm.wallet, outputs map[string]Output, ledgerstate *Ledger, inputs map[string]Input, manaPledgeID identity.ID, transactions map[string]*Transaction, branches map[string]branchdag.BranchIDs, alias string) {
	// issue Transaction alias
	{
		wallets[alias] = txvm.createWallets(1)[0]
		outputs[alias] = NewSigLockedSingleOutput(200, wallets[alias].address)

		transactionEssence := NewTransactionEssence(0, time.Now(), manaPledgeID, manaPledgeID,
			NewInputs(inputs["E"]),
			NewOutputs(outputs[alias]),
		)

		transactions[alias] = NewTransaction(transactionEssence, []UnlockBlock{
			NewSignatureUnlockBlock(
				NewED25519Signature(
					wallets["E"].keyPair.PublicKey,
					wallets["E"].keyPair.PrivateKey.Sign(transactionEssence.Bytes()),
				),
			),
		})

		addBranchAndRegister(branches, transactions, alias)

		outputs[alias].SetID(NewOutputID(transactions[alias].ID(), 0))
		inputs[alias] = NewUTXOInput(outputs[alias].ID())

		_, err := ledgerstate.BookTransaction(transactions[alias])
		require.NoError(t, err)
	}
}

func setupScenarioTopLayer1(t *testing.T, wallets map[string]txvm.wallet, outputs map[string]Output, ledgerstate *Ledger, inputs map[string]Input, manaPledgeID identity.ID, transactions map[string]*Transaction, branches map[string]branchdag.BranchIDs) {
	setupScenarioTopLayerGeneric(t, wallets, outputs, ledgerstate, inputs, manaPledgeID, transactions, branches, "F")
}

func setupScenarioTopLayer2(t *testing.T, wallets map[string]txvm.wallet, outputs map[string]Output, ledgerstate *Ledger, inputs map[string]Input, manaPledgeID identity.ID, transactions map[string]*Transaction, branches map[string]branchdag.BranchIDs) {
	setupScenarioTopLayerGeneric(t, wallets, outputs, ledgerstate, inputs, manaPledgeID, transactions, branches, "G")
}

func setupScenarioTopTopLayer(t *testing.T, wallets map[string]txvm.wallet, outputs map[string]Output, ledgerstate *Ledger, inputs map[string]Input, manaPledgeID identity.ID, transactions map[string]*Transaction, branches map[string]branchdag.BranchIDs) {
	// issue Transaction L
	{
		wallets["L"] = txvm.createWallets(1)[0]
		outputs["L"] = NewSigLockedSingleOutput(200, wallets["L"].address)

		transactionEssence := NewTransactionEssence(0, time.Now(), manaPledgeID, manaPledgeID,
			NewInputs(inputs["G"], inputs["H"]),
			NewOutputs(outputs["L"]),
		)

		transactions["L"] = NewTransaction(transactionEssence, []UnlockBlock{
			NewSignatureUnlockBlock(
				NewED25519Signature(
					wallets["G"].keyPair.PublicKey,
					wallets["G"].keyPair.PrivateKey.Sign(transactionEssence.Bytes()),
				),
			),
			NewSignatureUnlockBlock(
				NewED25519Signature(
					wallets["H"].keyPair.PublicKey,
					wallets["H"].keyPair.PrivateKey.Sign(transactionEssence.Bytes()),
				),
			),
		})

		branches["G+H"] = branches["G"].Clone().AddAll(branches["H"])

		outputs["L"].SetID(NewOutputID(transactions["L"].ID(), 0))
		inputs["L"] = NewUTXOInput(outputs["L"].ID())

		_, err := ledgerstate.BookTransaction(transactions["L"])
		require.NoError(t, err)
	}
}

func setupScenarioTopTopTopLayer(t *testing.T, wallets map[string]txvm.wallet, outputs map[string]Output, ledgerstate *Ledger, inputs map[string]Input, manaPledgeID identity.ID, transactions map[string]*Transaction) {
	// issue Transaction L
	{
		wallets["M"] = txvm.createWallets(1)[0]
		outputs["M"] = NewSigLockedSingleOutput(200, wallets["M"].address)

		transactionEssence := NewTransactionEssence(0, time.Now(), manaPledgeID, manaPledgeID,
			NewInputs(inputs["L"]),
			NewOutputs(outputs["M"]),
		)

		transactions["M"] = NewTransaction(transactionEssence, []UnlockBlock{
			NewSignatureUnlockBlock(
				NewED25519Signature(
					wallets["L"].keyPair.PublicKey,
					wallets["L"].keyPair.PrivateKey.Sign(transactionEssence.Bytes()),
				),
			),
		})

		outputs["M"].SetID(NewOutputID(transactions["M"].ID(), 0))
		inputs["M"] = NewUTXOInput(outputs["M"].ID())

		_, err := ledgerstate.BookTransaction(transactions["M"])
		require.NoError(t, err)
	}
}

func addBranchAndRegister(branches map[string]branchdag.BranchIDs, transactions map[string]*Transaction, transactionAlias string) {
	branchID := branchdag2.NewBranchID(transactions[transactionAlias].ID())
	branches[transactionAlias] = branchdag.NewBranchIDs(branchID)
	branchdag2.RegisterBranchIDAlias(branchID, "Branch"+transactionAlias)
}

func getSingleBranch(branches map[string]branchdag.BranchIDs, alias string) branchdag.BranchID {
	if len(branches[alias]) != 1 {
		panic(fmt.Sprintf("Branches with alias %s are multiple branches, not a single one: %s", alias, branches[alias]))
	}

	for branchID := range branches[alias] {
		return branchID
	}

	return branchdag2.UndefinedBranchID
}
