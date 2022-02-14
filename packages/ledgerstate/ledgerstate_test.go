package ledgerstate

import (
	"bytes"
	"testing"
	"time"

	"github.com/iotaledger/hive.go/identity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLedgerstate_SetBranchConfirmed(t *testing.T) {
	ledgerstate := setupDependencies(t)
	defer ledgerstate.Shutdown()

	manaPledgeID := identity.GenerateIdentity().ID()
	wallets := make(map[string]wallet)
	outputs := make(map[string]Output)
	inputs := make(map[string]Input)
	transactions := make(map[string]*Transaction)
	branches := make(map[string]BranchID)

	setupScenarioBottomLayer(t, wallets, outputs, ledgerstate, inputs, manaPledgeID, transactions, branches)

	// Mark A as Confirmed
	{
		require.True(t, ledgerstate.BranchDAG.SetBranchConfirmed(branches["A"]))

		assertBranchID(t, ledgerstate, transactions["A"], branches["A"])
		assertBranchID(t, ledgerstate, transactions["B"], branches["B"])
		assertBranchID(t, ledgerstate, transactions["C"], branches["C"])
		assertBranchID(t, ledgerstate, transactions["D"], branches["D"])
		assertBranchID(t, ledgerstate, transactions["H"], branches["H"])
		assertBranchID(t, ledgerstate, transactions["I"], branches["I"])

		assert.Equal(t, Confirmed, ledgerstate.BranchDAG.InclusionState(branches["A"]))
		assert.Equal(t, Pending, ledgerstate.BranchDAG.InclusionState(branches["C"]))
	}

	setupScenarioMiddleLayer(t, wallets, outputs, ledgerstate, inputs, manaPledgeID, transactions, branches)

	// When creating the middle layer the new transaction E should be booked only under its Pending parent C
	{
		assertBranchID(t, ledgerstate, transactions["A"], branches["A"])
		assertBranchID(t, ledgerstate, transactions["B"], branches["B"])
		assertBranchID(t, ledgerstate, transactions["C"], branches["C"])
		assertBranchID(t, ledgerstate, transactions["D"], branches["D"])
		assertBranchID(t, ledgerstate, transactions["H"], branches["H"])
		assertBranchID(t, ledgerstate, transactions["I"], branches["I"])
		assertBranchID(t, ledgerstate, transactions["E"], branches["C"])
	}

	setupScenarioTopLayer1(t, wallets, outputs, ledgerstate, inputs, manaPledgeID, transactions, branches)

	// When creating the first transaction of top layer it should be booked under the Pending parent C
	{
		assertBranchID(t, ledgerstate, transactions["A"], branches["A"])
		assertBranchID(t, ledgerstate, transactions["B"], branches["B"])
		assertBranchID(t, ledgerstate, transactions["C"], branches["C"])
		assertBranchID(t, ledgerstate, transactions["D"], branches["D"])
		assertBranchID(t, ledgerstate, transactions["H"], branches["H"])
		assertBranchID(t, ledgerstate, transactions["I"], branches["I"])
		assertBranchID(t, ledgerstate, transactions["E"], branches["C"])

		// Branches F & G are spawned by the fork of G
		assertBranchID(t, ledgerstate, transactions["F"], branches["C"])
	}

	setupScenarioTopLayer2(t, wallets, outputs, ledgerstate, inputs, manaPledgeID, transactions, branches)

	// When creating the conflicting TX of the top layer branches F & G are spawned by the fork of G
	{
		assertBranchID(t, ledgerstate, transactions["A"], branches["A"])
		assertBranchID(t, ledgerstate, transactions["B"], branches["B"])
		assertBranchID(t, ledgerstate, transactions["C"], branches["C"])
		assertBranchID(t, ledgerstate, transactions["D"], branches["D"])
		assertBranchID(t, ledgerstate, transactions["H"], branches["H"])
		assertBranchID(t, ledgerstate, transactions["I"], branches["I"])
		assertBranchID(t, ledgerstate, transactions["E"], branches["C"])

		// Branches F & G are spawned by the fork of G
		assertBranchID(t, ledgerstate, transactions["F"], branches["F"])
		assertBranchID(t, ledgerstate, transactions["G"], branches["G"])

		ledgerstate.BranchDAG.Branch(branches["F"]).Consume(func(branch Branch) {
			assert.Equal(t, NewBranchIDs(branches["C"]), branch.Parents())
		})

		ledgerstate.BranchDAG.Branch(branches["G"]).Consume(func(branch Branch) {
			assert.Equal(t, NewBranchIDs(branches["C"]), branch.Parents())
		})
	}

	require.True(t, ledgerstate.BranchDAG.SetBranchConfirmed(branches["D"]))

	setupScenarioTopTopLayer(t, wallets, outputs, ledgerstate, inputs, manaPledgeID, transactions, branches)

	// TX L aggregates a child (G) of a Rejected branch (C) and a pending branch H, resulting in C+H
	{
		assertBranchID(t, ledgerstate, transactions["A"], branches["A"])
		assertBranchID(t, ledgerstate, transactions["B"], branches["B"])
		assertBranchID(t, ledgerstate, transactions["C"], branches["C"])
		assertBranchID(t, ledgerstate, transactions["D"], branches["D"])
		assertBranchID(t, ledgerstate, transactions["H"], branches["H"])
		assertBranchID(t, ledgerstate, transactions["I"], branches["I"])
		assertBranchID(t, ledgerstate, transactions["E"], branches["C"])
		assertBranchID(t, ledgerstate, transactions["F"], branches["F"])
		assertBranchID(t, ledgerstate, transactions["G"], branches["G"])
		assertBranchID(t, ledgerstate, transactions["L"], branches["G+H"])

		assert.Equal(t, Confirmed, ledgerstate.BranchDAG.InclusionState(branches["D"]))
		assert.Equal(t, Rejected, ledgerstate.BranchDAG.InclusionState(branches["C"]))
		assert.Equal(t, Rejected, ledgerstate.BranchDAG.InclusionState(branches["F"]))
		assert.Equal(t, Rejected, ledgerstate.BranchDAG.InclusionState(branches["G"]))
		assert.Equal(t, Rejected, ledgerstate.BranchDAG.InclusionState(branches["G+H"]))
		assert.Equal(t, Pending, ledgerstate.BranchDAG.InclusionState(branches["H"]))
		assert.Equal(t, Pending, ledgerstate.BranchDAG.InclusionState(branches["I"]))

		ledgerstate.BranchDAG.Branch(branches["G+H"]).Consume(func(branch Branch) {
			assert.Equal(t, NewBranchIDs(branches["G"], branches["H"]), branch.Parents())
		})
	}

	require.True(t, ledgerstate.BranchDAG.SetBranchConfirmed(branches["H"]))

	setupScenarioTopTopTopLayer(t, wallets, outputs, ledgerstate, inputs, manaPledgeID, transactions)

	// The new TX M should be now booked under G, as the aggregated branch G+H got H confirmed, transforming it into
	// a ConflictBranch again: just G.
	{
		assertBranchID(t, ledgerstate, transactions["A"], branches["A"])
		assertBranchID(t, ledgerstate, transactions["B"], branches["B"])
		assertBranchID(t, ledgerstate, transactions["C"], branches["C"])
		assertBranchID(t, ledgerstate, transactions["D"], branches["D"])
		assertBranchID(t, ledgerstate, transactions["H"], branches["H"])
		assertBranchID(t, ledgerstate, transactions["I"], branches["I"])
		assertBranchID(t, ledgerstate, transactions["E"], branches["C"])
		assertBranchID(t, ledgerstate, transactions["F"], branches["F"])
		assertBranchID(t, ledgerstate, transactions["G"], branches["G"])
		assertBranchID(t, ledgerstate, transactions["L"], branches["G+H"])
		assertBranchID(t, ledgerstate, transactions["M"], branches["G"])

		assert.Equal(t, Confirmed, ledgerstate.BranchDAG.InclusionState(branches["D"]))
		assert.Equal(t, Rejected, ledgerstate.BranchDAG.InclusionState(branches["C"]))
		assert.Equal(t, Rejected, ledgerstate.BranchDAG.InclusionState(branches["F"]))
		assert.Equal(t, Rejected, ledgerstate.BranchDAG.InclusionState(branches["G"]))
		assert.Equal(t, Rejected, ledgerstate.BranchDAG.InclusionState(branches["G+H"]))
		assert.Equal(t, Confirmed, ledgerstate.BranchDAG.InclusionState(branches["H"]))
		assert.Equal(t, Rejected, ledgerstate.BranchDAG.InclusionState(branches["I"]))
	}
}

func assertBranchID(t *testing.T, ledgerstate *Ledgerstate, transaction *Transaction, branchID BranchID) {
	assert.True(t, ledgerstate.CachedTransactionMetadata(transaction.ID()).Consume(func(transactionMetadata *TransactionMetadata) {
		assert.Equal(t, branchID, transactionMetadata.BranchID(), transactionMetadata.String(), branchID.String())
	}))

	for _, output := range transaction.Essence().Outputs() {
		assert.True(t, ledgerstate.CachedOutputMetadata(output.ID()).Consume(func(outputMetadata *OutputMetadata) {
			assert.Equal(t, branchID, outputMetadata.BranchID(), outputMetadata.String(), branchID.String())
		}))
	}
}

func setupScenarioBottomLayer(t *testing.T, wallets map[string]wallet, outputs map[string]Output, ledgerstate *Ledgerstate, inputs map[string]Input, manaPledgeID identity.ID, transactions map[string]*Transaction, branches map[string]BranchID) {
	// create genesis outputs
	{
		wallets["GENESIS_1"] = createWallets(1)[0]
		outputs["GENESIS_1"] = generateOutput(ledgerstate, wallets["GENESIS_1"].address, 0)
		inputs["GENESIS_1"] = NewUTXOInput(outputs["GENESIS_1"].ID())

		wallets["GENESIS_2"] = createWallets(1)[0]
		outputs["GENESIS_2"] = generateOutput(ledgerstate, wallets["GENESIS_2"].address, 1)
		inputs["GENESIS_2"] = NewUTXOInput(outputs["GENESIS_2"].ID())

		wallets["GENESIS_3"] = createWallets(1)[0]
		outputs["GENESIS_3"] = generateOutput(ledgerstate, wallets["GENESIS_3"].address, 2)
		inputs["GENESIS_3"] = NewUTXOInput(outputs["GENESIS_3"].ID())
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

	// issue Transaction H
	{
		wallets["H"] = createWallets(1)[0]
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

		branches["H"] = NewBranchID(transactions["H"].ID())
		RegisterBranchIDAlias(branches["H"], "BranchH")

		outputs["H"].SetID(NewOutputID(transactions["H"].ID(), 0))
		inputs["H"] = NewUTXOInput(outputs["H"].ID())

		_, err := ledgerstate.BookTransaction(transactions["H"])
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
		assert.True(t, ledgerstate.CachedTransactionMetadata(transactions["H"].ID()).Consume(func(transactionMetadata *TransactionMetadata) {
			assert.Equal(t, MasterBranchID, transactionMetadata.BranchID())
		}))
	}

	// issue Transaction I (conflicting with H)
	{
		wallets["I"] = createWallets(1)[0]
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

		branches["I"] = NewBranchID(transactions["I"].ID())
		RegisterBranchIDAlias(branches["I"], "BranchI")

		outputs["I"].SetID(NewOutputID(transactions["I"].ID(), 0))
		inputs["I"] = NewUTXOInput(outputs["I"].ID())

		_, err := ledgerstate.BookTransaction(transactions["I"])
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
		assert.True(t, ledgerstate.CachedTransactionMetadata(transactions["H"].ID()).Consume(func(transactionMetadata *TransactionMetadata) {
			assert.Equal(t, branches["H"], transactionMetadata.BranchID())
		}))
		assert.True(t, ledgerstate.CachedTransactionMetadata(transactions["I"].ID()).Consume(func(transactionMetadata *TransactionMetadata) {
			assert.Equal(t, branches["I"], transactionMetadata.BranchID())
		}))
	}
}

func setupScenarioMiddleLayer(t *testing.T, wallets map[string]wallet, outputs map[string]Output, ledgerstate *Ledgerstate, inputs map[string]Input, manaPledgeID identity.ID, transactions map[string]*Transaction, branches map[string]BranchID) {
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
	}
}

func setupScenarioTopLayerGeneric(t *testing.T, wallets map[string]wallet, outputs map[string]Output, ledgerstate *Ledgerstate, inputs map[string]Input, manaPledgeID identity.ID, transactions map[string]*Transaction, branches map[string]BranchID, alias string) {
	// issue Transaction alias
	{
		wallets[alias] = createWallets(1)[0]
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

		branches[alias] = NewBranchID(transactions[alias].ID())
		RegisterBranchIDAlias(branches[alias], "Branch"+alias)

		outputs[alias].SetID(NewOutputID(transactions[alias].ID(), 0))
		inputs[alias] = NewUTXOInput(outputs[alias].ID())

		_, err := ledgerstate.BookTransaction(transactions[alias])
		require.NoError(t, err)
	}
}

func setupScenarioTopLayer1(t *testing.T, wallets map[string]wallet, outputs map[string]Output, ledgerstate *Ledgerstate, inputs map[string]Input, manaPledgeID identity.ID, transactions map[string]*Transaction, branches map[string]BranchID) {
	setupScenarioTopLayerGeneric(t, wallets, outputs, ledgerstate, inputs, manaPledgeID, transactions, branches, "F")
}

func setupScenarioTopLayer2(t *testing.T, wallets map[string]wallet, outputs map[string]Output, ledgerstate *Ledgerstate, inputs map[string]Input, manaPledgeID identity.ID, transactions map[string]*Transaction, branches map[string]BranchID) {
	setupScenarioTopLayerGeneric(t, wallets, outputs, ledgerstate, inputs, manaPledgeID, transactions, branches, "G")
}

func setupScenarioTopTopLayer(t *testing.T, wallets map[string]wallet, outputs map[string]Output, ledgerstate *Ledgerstate, inputs map[string]Input, manaPledgeID identity.ID, transactions map[string]*Transaction, branches map[string]BranchID) {
	// issue Transaction L
	{
		wallets["L"] = createWallets(1)[0]
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

		branches["G+H"] = NewAggregatedBranch(NewBranchIDs(
			branches["G"],
			branches["H"],
		)).ID()
		RegisterBranchIDAlias(branches["G+H"], "BranchG+H")

		outputs["L"].SetID(NewOutputID(transactions["L"].ID(), 0))
		inputs["L"] = NewUTXOInput(outputs["L"].ID())

		_, err := ledgerstate.BookTransaction(transactions["L"])
		require.NoError(t, err)
	}
}

func setupScenarioTopTopTopLayer(t *testing.T, wallets map[string]wallet, outputs map[string]Output, ledgerstate *Ledgerstate, inputs map[string]Input, manaPledgeID identity.ID, transactions map[string]*Transaction) {
	// issue Transaction L
	{
		wallets["M"] = createWallets(1)[0]
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
