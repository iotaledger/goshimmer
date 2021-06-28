package tangle

import (
	"strconv"
	"testing"
	"time"

	"github.com/iotaledger/hive.go/identity"
	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/markers"
)

func TestScenario_1(t *testing.T) {
	tangle := newTestTangle()
	defer tangle.Shutdown()
	tangle.Solidifier.Setup()
	tangle.Booker.Setup()

	wallets := make(map[string]wallet)
	walletsByAddress := make(map[ledgerstate.Address]wallet)
	w := createWallets(10)
	wallets["GENESIS"] = w[0]
	wallets["A"] = w[1]
	wallets["B"] = w[2]
	wallets["C"] = w[3]
	wallets["D"] = w[4]
	wallets["E"] = w[5]
	wallets["F"] = w[6]
	wallets["H"] = w[7]
	wallets["I"] = w[8]
	wallets["J"] = w[9]
	for _, wallet := range wallets {
		walletsByAddress[wallet.address] = wallet
	}

	genesisBalance := ledgerstate.NewColoredBalances(
		map[ledgerstate.Color]uint64{
			ledgerstate.ColorIOTA: 3,
		})

	genesisEssence := ledgerstate.NewTransactionEssence(
		0,
		time.Unix(DefaultGenesisTime, 0),
		identity.ID{},
		identity.ID{},
		ledgerstate.NewInputs(ledgerstate.NewUTXOInput(ledgerstate.NewOutputID(ledgerstate.GenesisTransactionID, 0))),
		ledgerstate.NewOutputs(ledgerstate.NewSigLockedColoredOutput(genesisBalance, wallets["GENESIS"].address)),
	)

	genesisTransaction := ledgerstate.NewTransaction(genesisEssence, ledgerstate.UnlockBlocks{ledgerstate.NewReferenceUnlockBlock(0)})

	snapshot := &ledgerstate.Snapshot{
		Transactions: map[ledgerstate.TransactionID]ledgerstate.Record{
			genesisTransaction.ID(): {
				Essence:        genesisEssence,
				UnlockBlocks:   ledgerstate.UnlockBlocks{ledgerstate.NewReferenceUnlockBlock(0)},
				UnspentOutputs: []bool{true},
			},
		},
	}

	tangle.LedgerState.LoadSnapshot(snapshot)

	messages := make(map[string]*Message)
	transactions := make(map[string]*ledgerstate.Transaction)
	branches := make(map[string]ledgerstate.BranchID)
	inputs := make(map[string]*ledgerstate.UTXOInput)
	outputs := make(map[string]*ledgerstate.SigLockedSingleOutput)
	outputsByID := make(map[ledgerstate.OutputID]ledgerstate.Output)

	branches["empty"] = ledgerstate.BranchID{}
	branches["green"] = ledgerstate.MasterBranchID
	branches["grey"] = ledgerstate.InvalidBranchID

	// Message 1
	inputs["GENESIS"] = ledgerstate.NewUTXOInput(ledgerstate.NewOutputID(genesisTransaction.ID(), 0))
	outputs["A"] = ledgerstate.NewSigLockedSingleOutput(1, wallets["A"].address)
	outputs["B"] = ledgerstate.NewSigLockedSingleOutput(1, wallets["B"].address)

	outputs["C"] = ledgerstate.NewSigLockedSingleOutput(1, wallets["C"].address)
	transactions["1"] = makeTransaction(ledgerstate.NewInputs(inputs["GENESIS"]), ledgerstate.NewOutputs(outputs["A"], outputs["B"], outputs["C"]), outputsByID, walletsByAddress, wallets["GENESIS"])
	messages["1"] = newTestParentsPayloadMessage(transactions["1"], []MessageID{EmptyMessageID}, []MessageID{})
	tangle.Storage.StoreMessage(messages["1"])

	err := tangle.Booker.BookMessage(messages["1"].ID())

	require.NoError(t, err)

	msgBranchID, err := messageBranchID(tangle, messages["1"].ID())
	require.NoError(t, err)
	assert.Equal(t, branches["green"], msgBranchID)

	txBranchID, err := transactionBranchID(tangle, transactions["1"].ID())
	require.NoError(t, err)
	assert.Equal(t, branches["green"], txBranchID)

	// Message 2
	inputs["B"] = ledgerstate.NewUTXOInput(ledgerstate.NewOutputID(transactions["1"].ID(), selectIndex(transactions["1"], wallets["B"])))
	outputsByID[inputs["B"].ReferencedOutputID()] = ledgerstate.NewOutputs(outputs["B"])[0]
	inputs["C"] = ledgerstate.NewUTXOInput(ledgerstate.NewOutputID(transactions["1"].ID(), selectIndex(transactions["1"], wallets["C"])))
	outputsByID[inputs["C"].ReferencedOutputID()] = ledgerstate.NewOutputs(outputs["C"])[0]
	outputs["E"] = ledgerstate.NewSigLockedSingleOutput(2, wallets["E"].address)
	transactions["2"] = makeTransaction(ledgerstate.NewInputs(inputs["B"], inputs["C"]), ledgerstate.NewOutputs(outputs["E"]), outputsByID, walletsByAddress)
	messages["2"] = newTestParentsPayloadMessage(transactions["2"], []MessageID{EmptyMessageID, messages["1"].ID()}, []MessageID{})
	tangle.Storage.StoreMessage(messages["2"])

	err = tangle.Booker.BookMessage(messages["2"].ID())
	require.NoError(t, err)

	msgBranchID, err = messageBranchID(tangle, messages["2"].ID())
	require.NoError(t, err)
	assert.Equal(t, branches["green"], msgBranchID)

	txBranchID, err = transactionBranchID(tangle, transactions["2"].ID())
	require.NoError(t, err)
	assert.Equal(t, branches["green"], txBranchID)

	// Message 3 (Reattachemnt of transaction 2)
	messages["3"] = newTestParentsPayloadMessage(transactions["2"], []MessageID{messages["1"].ID(), messages["2"].ID()}, []MessageID{})
	tangle.Storage.StoreMessage(messages["3"])

	err = tangle.Booker.BookMessage(messages["3"].ID())
	require.NoError(t, err)

	msgBranchID, err = messageBranchID(tangle, messages["3"].ID())
	require.NoError(t, err)
	assert.Equal(t, branches["green"], msgBranchID)

	txBranchID, err = transactionBranchID(tangle, transactions["2"].ID())
	require.NoError(t, err)
	assert.Equal(t, branches["green"], txBranchID)

	// Message 4
	inputs["A"] = ledgerstate.NewUTXOInput(ledgerstate.NewOutputID(transactions["1"].ID(), selectIndex(transactions["1"], wallets["A"])))
	outputsByID[inputs["A"].ReferencedOutputID()] = ledgerstate.NewOutputs(outputs["A"])[0]
	outputs["D"] = ledgerstate.NewSigLockedSingleOutput(1, wallets["D"].address)
	transactions["3"] = makeTransaction(ledgerstate.NewInputs(inputs["A"]), ledgerstate.NewOutputs(outputs["D"]), outputsByID, walletsByAddress)
	messages["4"] = newTestParentsPayloadMessage(transactions["3"], []MessageID{EmptyMessageID, messages["1"].ID()}, []MessageID{})
	tangle.Storage.StoreMessage(messages["4"])

	err = tangle.Booker.BookMessage(messages["4"].ID())
	require.NoError(t, err)

	msgBranchID, err = messageBranchID(tangle, messages["4"].ID())
	require.NoError(t, err)
	assert.Equal(t, branches["green"], msgBranchID)

	txBranchID, err = transactionBranchID(tangle, transactions["3"].ID())
	require.NoError(t, err)
	assert.Equal(t, branches["green"], txBranchID)

	// Message 5
	outputs["F"] = ledgerstate.NewSigLockedSingleOutput(1, wallets["F"].address)
	transactions["4"] = makeTransaction(ledgerstate.NewInputs(inputs["A"]), ledgerstate.NewOutputs(outputs["F"]), outputsByID, walletsByAddress)
	messages["5"] = newTestParentsPayloadMessage(transactions["4"], []MessageID{EmptyMessageID, messages["1"].ID()}, []MessageID{})
	tangle.Storage.StoreMessage(messages["5"])

	err = tangle.Booker.BookMessage(messages["5"].ID())
	require.NoError(t, err)

	branches["yellow"] = ledgerstate.NewBranchID(transactions["4"].ID())

	msgBranchID, err = messageBranchID(tangle, messages["5"].ID())
	require.NoError(t, err)
	assert.Equal(t, branches["yellow"], msgBranchID)

	txBranchID, err = transactionBranchID(tangle, transactions["4"].ID())
	require.NoError(t, err)
	assert.Equal(t, branches["yellow"], txBranchID)

	branches["red"] = ledgerstate.NewBranchID(transactions["3"].ID())

	msgBranchID, err = messageBranchID(tangle, messages["4"].ID())
	require.NoError(t, err)
	assert.Equal(t, branches["red"], msgBranchID)

	txBranchID, err = transactionBranchID(tangle, transactions["3"].ID())
	require.NoError(t, err)
	assert.Equal(t, branches["red"], txBranchID)

	// Message 6
	inputs["E"] = ledgerstate.NewUTXOInput(ledgerstate.NewOutputID(transactions["2"].ID(), 0))
	outputsByID[inputs["E"].ReferencedOutputID()] = ledgerstate.NewOutputs(outputs["E"])[0]
	inputs["F"] = ledgerstate.NewUTXOInput(ledgerstate.NewOutputID(transactions["4"].ID(), 0))
	outputsByID[inputs["F"].ReferencedOutputID()] = ledgerstate.NewOutputs(outputs["F"])[0]
	outputs["H"] = ledgerstate.NewSigLockedSingleOutput(3, wallets["H"].address)
	transactions["5"] = makeTransaction(ledgerstate.NewInputs(inputs["E"], inputs["F"]), ledgerstate.NewOutputs(outputs["H"]), outputsByID, walletsByAddress)
	messages["6"] = newTestParentsPayloadMessage(transactions["5"], []MessageID{messages["2"].ID(), messages["5"].ID()}, []MessageID{})
	tangle.Storage.StoreMessage(messages["6"])

	err = tangle.Booker.BookMessage(messages["6"].ID())
	require.NoError(t, err)

	msgBranchID, err = messageBranchID(tangle, messages["6"].ID())
	require.NoError(t, err)
	assert.Equal(t, branches["yellow"], msgBranchID)

	txBranchID, err = transactionBranchID(tangle, transactions["5"].ID())
	require.NoError(t, err)
	assert.Equal(t, branches["yellow"], txBranchID)

	// Message 7
	messages["7"] = newTestParentsPayloadMessage(transactions["2"], []MessageID{messages["4"].ID(), messages["5"].ID()}, []MessageID{})
	tangle.Storage.StoreMessage(messages["7"])

	err = tangle.Booker.BookMessage(messages["7"].ID())
	require.NoError(t, err)

	msgBranchID, err = messageBranchID(tangle, messages["7"].ID())
	require.NoError(t, err)
	assert.Equal(t, branches["grey"], msgBranchID)

	txBranchID, err = transactionBranchID(tangle, transactions["2"].ID())
	require.NoError(t, err)
	assert.Equal(t, branches["green"], txBranchID)

	// Message 8
	inputs["D"] = ledgerstate.NewUTXOInput(ledgerstate.NewOutputID(transactions["3"].ID(), 0))
	outputsByID[inputs["D"].ReferencedOutputID()] = ledgerstate.NewOutputs(outputs["D"])[0]
	outputs["I"] = ledgerstate.NewSigLockedSingleOutput(2, wallets["I"].address)
	transactions["6"] = makeTransaction(ledgerstate.NewInputs(inputs["F"], inputs["D"]), ledgerstate.NewOutputs(outputs["I"]), outputsByID, walletsByAddress)
	messages["8"] = newTestParentsPayloadMessage(transactions["6"], []MessageID{messages["4"].ID(), messages["5"].ID()}, []MessageID{})
	tangle.Storage.StoreMessage(messages["8"])

	err = tangle.Booker.BookMessage(messages["8"].ID())
	require.NoError(t, err)

	msgBranchID, err = messageBranchID(tangle, messages["8"].ID())
	require.NoError(t, err)
	assert.Equal(t, branches["grey"], msgBranchID)

	txBranchID, err = transactionBranchID(tangle, transactions["6"].ID())
	require.NoError(t, err)
	assert.Equal(t, branches["grey"], txBranchID)

	// Message 9
	inputs["H"] = ledgerstate.NewUTXOInput(ledgerstate.NewOutputID(transactions["5"].ID(), 0))
	outputsByID[inputs["H"].ReferencedOutputID()] = ledgerstate.NewOutputs(outputs["H"])[0]
	outputs["J"] = ledgerstate.NewSigLockedSingleOutput(3, wallets["J"].address)
	transactions["7"] = makeTransaction(ledgerstate.NewInputs(inputs["H"]), ledgerstate.NewOutputs(outputs["J"]), outputsByID, walletsByAddress)
	messages["9"] = newTestParentsPayloadMessage(transactions["7"], []MessageID{messages["4"].ID(), messages["6"].ID()}, []MessageID{})
	tangle.Storage.StoreMessage(messages["9"])

	err = tangle.Booker.BookMessage(messages["9"].ID())
	require.NoError(t, err)

	msgBranchID, err = messageBranchID(tangle, messages["9"].ID())
	require.NoError(t, err)
	assert.Equal(t, branches["grey"], msgBranchID)

	txBranchID, err = transactionBranchID(tangle, transactions["7"].ID())
	require.NoError(t, err)
	assert.Equal(t, branches["yellow"], txBranchID)
}

func TestScenario_2(t *testing.T) {
	tangle := newTestTangle()
	defer tangle.Shutdown()
	tangle.Solidifier.Setup()
	tangle.Booker.Setup()

	wallets := make(map[string]wallet)
	walletsByAddress := make(map[ledgerstate.Address]wallet)
	w := createWallets(11)
	wallets["GENESIS"] = w[0]
	wallets["A"] = w[1]
	wallets["B"] = w[2]
	wallets["C"] = w[3]
	wallets["D"] = w[4]
	wallets["E"] = w[5]
	wallets["F"] = w[6]
	wallets["H"] = w[7]
	wallets["I"] = w[8]
	wallets["J"] = w[9]
	wallets["L"] = w[10]
	for _, wallet := range wallets {
		walletsByAddress[wallet.address] = wallet
	}

	genesisBalance := ledgerstate.NewColoredBalances(
		map[ledgerstate.Color]uint64{
			ledgerstate.ColorIOTA: 3,
		})
	genesisEssence := ledgerstate.NewTransactionEssence(
		0,
		time.Unix(DefaultGenesisTime, 0),
		identity.ID{},
		identity.ID{},
		ledgerstate.NewInputs(ledgerstate.NewUTXOInput(ledgerstate.NewOutputID(ledgerstate.GenesisTransactionID, 0))),
		ledgerstate.NewOutputs(ledgerstate.NewSigLockedColoredOutput(genesisBalance, wallets["GENESIS"].address)),
	)

	genesisTransaction := ledgerstate.NewTransaction(genesisEssence, ledgerstate.UnlockBlocks{ledgerstate.NewReferenceUnlockBlock(0)})

	snapshot := &ledgerstate.Snapshot{
		Transactions: map[ledgerstate.TransactionID]ledgerstate.Record{
			genesisTransaction.ID(): {
				Essence:        genesisEssence,
				UnlockBlocks:   ledgerstate.UnlockBlocks{ledgerstate.NewReferenceUnlockBlock(0)},
				UnspentOutputs: []bool{true},
			},
		},
	}

	tangle.LedgerState.LoadSnapshot(snapshot)

	messages := make(map[string]*Message)
	transactions := make(map[string]*ledgerstate.Transaction)
	branches := make(map[string]ledgerstate.BranchID)
	inputs := make(map[string]*ledgerstate.UTXOInput)
	outputs := make(map[string]*ledgerstate.SigLockedSingleOutput)
	outputsByID := make(map[ledgerstate.OutputID]ledgerstate.Output)

	branches["empty"] = ledgerstate.BranchID{}
	branches["green"] = ledgerstate.MasterBranchID
	branches["grey"] = ledgerstate.InvalidBranchID

	// Message 1
	inputs["GENESIS"] = ledgerstate.NewUTXOInput(ledgerstate.NewOutputID(genesisTransaction.ID(), 0))
	outputs["A"] = ledgerstate.NewSigLockedSingleOutput(1, wallets["A"].address)
	outputs["B"] = ledgerstate.NewSigLockedSingleOutput(1, wallets["B"].address)

	outputs["C"] = ledgerstate.NewSigLockedSingleOutput(1, wallets["C"].address)
	transactions["1"] = makeTransaction(ledgerstate.NewInputs(inputs["GENESIS"]), ledgerstate.NewOutputs(outputs["A"], outputs["B"], outputs["C"]), outputsByID, walletsByAddress, wallets["GENESIS"])
	messages["1"] = newTestParentsPayloadMessage(transactions["1"], []MessageID{EmptyMessageID}, []MessageID{})
	tangle.Storage.StoreMessage(messages["1"])

	err := tangle.Booker.BookMessage(messages["1"].ID())

	require.NoError(t, err)

	msgBranchID, err := messageBranchID(tangle, messages["1"].ID())
	require.NoError(t, err)
	assert.Equal(t, branches["green"], msgBranchID)

	txBranchID, err := transactionBranchID(tangle, transactions["1"].ID())
	require.NoError(t, err)
	assert.Equal(t, branches["green"], txBranchID)

	// Message 2
	inputs["B"] = ledgerstate.NewUTXOInput(ledgerstate.NewOutputID(transactions["1"].ID(), selectIndex(transactions["1"], wallets["B"])))
	outputsByID[inputs["B"].ReferencedOutputID()] = ledgerstate.NewOutputs(outputs["B"])[0]
	inputs["C"] = ledgerstate.NewUTXOInput(ledgerstate.NewOutputID(transactions["1"].ID(), selectIndex(transactions["1"], wallets["C"])))
	outputsByID[inputs["C"].ReferencedOutputID()] = ledgerstate.NewOutputs(outputs["C"])[0]
	outputs["E"] = ledgerstate.NewSigLockedSingleOutput(2, wallets["E"].address)
	transactions["2"] = makeTransaction(ledgerstate.NewInputs(inputs["B"], inputs["C"]), ledgerstate.NewOutputs(outputs["E"]), outputsByID, walletsByAddress)
	messages["2"] = newTestParentsPayloadMessage(transactions["2"], []MessageID{EmptyMessageID, messages["1"].ID()}, []MessageID{})
	tangle.Storage.StoreMessage(messages["2"])

	err = tangle.Booker.BookMessage(messages["2"].ID())
	require.NoError(t, err)

	msgBranchID, err = messageBranchID(tangle, messages["2"].ID())
	require.NoError(t, err)
	assert.Equal(t, branches["green"], msgBranchID)

	txBranchID, err = transactionBranchID(tangle, transactions["2"].ID())
	require.NoError(t, err)
	assert.Equal(t, branches["green"], txBranchID)

	// Message 3 (Reattachemnt of transaction 2)
	messages["3"] = newTestParentsPayloadMessage(transactions["2"], []MessageID{messages["1"].ID(), messages["2"].ID()}, []MessageID{})
	tangle.Storage.StoreMessage(messages["3"])

	err = tangle.Booker.BookMessage(messages["3"].ID())
	require.NoError(t, err)

	msgBranchID, err = messageBranchID(tangle, messages["3"].ID())
	require.NoError(t, err)
	assert.Equal(t, branches["green"], msgBranchID)

	txBranchID, err = transactionBranchID(tangle, transactions["2"].ID())
	require.NoError(t, err)
	assert.Equal(t, branches["green"], txBranchID)

	// Message 4
	inputs["A"] = ledgerstate.NewUTXOInput(ledgerstate.NewOutputID(transactions["1"].ID(), selectIndex(transactions["1"], wallets["A"])))
	outputsByID[inputs["A"].ReferencedOutputID()] = ledgerstate.NewOutputs(outputs["A"])[0]
	outputs["D"] = ledgerstate.NewSigLockedSingleOutput(1, wallets["D"].address)
	transactions["3"] = makeTransaction(ledgerstate.NewInputs(inputs["A"]), ledgerstate.NewOutputs(outputs["D"]), outputsByID, walletsByAddress)
	messages["4"] = newTestParentsPayloadMessage(transactions["3"], []MessageID{EmptyMessageID, messages["1"].ID()}, []MessageID{})
	tangle.Storage.StoreMessage(messages["4"])

	err = tangle.Booker.BookMessage(messages["4"].ID())
	require.NoError(t, err)

	msgBranchID, err = messageBranchID(tangle, messages["4"].ID())
	require.NoError(t, err)
	assert.Equal(t, branches["green"], msgBranchID)

	txBranchID, err = transactionBranchID(tangle, transactions["3"].ID())
	require.NoError(t, err)
	assert.Equal(t, branches["green"], txBranchID)

	// Message 5
	outputs["F"] = ledgerstate.NewSigLockedSingleOutput(1, wallets["F"].address)
	transactions["4"] = makeTransaction(ledgerstate.NewInputs(inputs["A"]), ledgerstate.NewOutputs(outputs["F"]), outputsByID, walletsByAddress)
	messages["5"] = newTestParentsPayloadMessage(transactions["4"], []MessageID{messages["1"].ID(), messages["2"].ID()}, []MessageID{})
	tangle.Storage.StoreMessage(messages["5"])

	err = tangle.Booker.BookMessage(messages["5"].ID())
	require.NoError(t, err)

	branches["yellow"] = ledgerstate.NewBranchID(transactions["4"].ID())

	msgBranchID, err = messageBranchID(tangle, messages["5"].ID())
	require.NoError(t, err)
	assert.Equal(t, branches["yellow"], msgBranchID)

	txBranchID, err = transactionBranchID(tangle, transactions["4"].ID())
	require.NoError(t, err)
	assert.Equal(t, branches["yellow"], txBranchID)

	branches["red"] = ledgerstate.NewBranchID(transactions["3"].ID())

	msgBranchID, err = messageBranchID(tangle, messages["4"].ID())
	require.NoError(t, err)
	assert.Equal(t, branches["red"], msgBranchID)

	txBranchID, err = transactionBranchID(tangle, transactions["3"].ID())
	require.NoError(t, err)
	assert.Equal(t, branches["red"], txBranchID)

	// Message 6
	inputs["E"] = ledgerstate.NewUTXOInput(ledgerstate.NewOutputID(transactions["2"].ID(), 0))
	outputsByID[inputs["E"].ReferencedOutputID()] = ledgerstate.NewOutputs(outputs["E"])[0]
	inputs["F"] = ledgerstate.NewUTXOInput(ledgerstate.NewOutputID(transactions["4"].ID(), 0))
	outputsByID[inputs["F"].ReferencedOutputID()] = ledgerstate.NewOutputs(outputs["F"])[0]
	outputs["L"] = ledgerstate.NewSigLockedSingleOutput(3, wallets["L"].address)
	transactions["5"] = makeTransaction(ledgerstate.NewInputs(inputs["E"], inputs["F"]), ledgerstate.NewOutputs(outputs["L"]), outputsByID, walletsByAddress)
	messages["6"] = newTestParentsPayloadMessage(transactions["5"], []MessageID{messages["2"].ID(), messages["5"].ID()}, []MessageID{})
	tangle.Storage.StoreMessage(messages["6"])

	err = tangle.Booker.BookMessage(messages["6"].ID())
	require.NoError(t, err)

	msgBranchID, err = messageBranchID(tangle, messages["6"].ID())
	require.NoError(t, err)
	assert.Equal(t, branches["yellow"], msgBranchID)

	txBranchID, err = transactionBranchID(tangle, transactions["5"].ID())
	require.NoError(t, err)
	assert.Equal(t, branches["yellow"], txBranchID)

	// Message 7
	outputs["H"] = ledgerstate.NewSigLockedSingleOutput(1, wallets["H"].address)
	transactions["6"] = makeTransaction(ledgerstate.NewInputs(inputs["C"]), ledgerstate.NewOutputs(outputs["H"]), outputsByID, walletsByAddress)
	messages["7"] = newTestParentsPayloadMessage(transactions["6"], []MessageID{messages["1"].ID(), messages["4"].ID()}, []MessageID{})
	tangle.Storage.StoreMessage(messages["7"])

	err = tangle.Booker.BookMessage(messages["7"].ID())
	require.NoError(t, err)

	branches["orange"] = ledgerstate.NewBranchID(transactions["6"].ID())
	branches["purple"] = ledgerstate.NewBranchID(transactions["2"].ID())
	branches["red+orange"] = ledgerstate.NewAggregatedBranch(ledgerstate.NewBranchIDs(branches["red"], branches["orange"])).ID()
	branches["yellow+purple"] = ledgerstate.NewAggregatedBranch(ledgerstate.NewBranchIDs(branches["yellow"], branches["purple"])).ID()

	msgBranchID, err = messageBranchID(tangle, messages["7"].ID())
	require.NoError(t, err)
	assert.Equal(t, branches["red+orange"], msgBranchID)

	txBranchID, err = transactionBranchID(tangle, transactions["6"].ID())
	require.NoError(t, err)
	assert.Equal(t, branches["orange"], txBranchID)

	msgBranchID, err = messageBranchID(tangle, messages["2"].ID())
	require.NoError(t, err)
	assert.Equal(t, branches["purple"], msgBranchID)

	txBranchID, err = transactionBranchID(tangle, transactions["2"].ID())
	require.NoError(t, err)
	assert.Equal(t, branches["purple"], txBranchID)

	msgBranchID, err = messageBranchID(tangle, messages["3"].ID())
	require.NoError(t, err)
	assert.Equal(t, branches["purple"], msgBranchID)

	msgBranchID, err = messageBranchID(tangle, messages["5"].ID())
	require.NoError(t, err)
	assert.Equal(t, branches["yellow+purple"], msgBranchID)

	txBranchID, err = transactionBranchID(tangle, transactions["4"].ID())
	require.NoError(t, err)
	assert.Equal(t, branches["yellow"], txBranchID)

	msgBranchID, err = messageBranchID(tangle, messages["6"].ID())
	require.NoError(t, err)
	assert.Equal(t, branches["yellow+purple"], msgBranchID)

	txBranchID, err = transactionBranchID(tangle, transactions["5"].ID())
	require.NoError(t, err)
	assert.Equal(t, branches["yellow+purple"], txBranchID)

	// Message 8
	inputs["H"] = ledgerstate.NewUTXOInput(ledgerstate.NewOutputID(transactions["6"].ID(), 0))
	outputsByID[inputs["H"].ReferencedOutputID()] = ledgerstate.NewOutputs(outputs["H"])[0]
	inputs["D"] = ledgerstate.NewUTXOInput(ledgerstate.NewOutputID(transactions["3"].ID(), 0))
	outputsByID[inputs["D"].ReferencedOutputID()] = ledgerstate.NewOutputs(outputs["D"])[0]
	outputs["I"] = ledgerstate.NewSigLockedSingleOutput(2, wallets["I"].address)
	transactions["7"] = makeTransaction(ledgerstate.NewInputs(inputs["D"], inputs["H"]), ledgerstate.NewOutputs(outputs["I"]), outputsByID, walletsByAddress)
	messages["8"] = newTestParentsPayloadMessage(transactions["7"], []MessageID{messages["4"].ID(), messages["7"].ID()}, []MessageID{})
	tangle.Storage.StoreMessage(messages["8"])

	err = tangle.Booker.BookMessage(messages["8"].ID())
	require.NoError(t, err)

	msgBranchID, err = messageBranchID(tangle, messages["8"].ID())
	require.NoError(t, err)
	assert.Equal(t, branches["red+orange"], msgBranchID)

	txBranchID, err = transactionBranchID(tangle, transactions["7"].ID())
	require.NoError(t, err)
	assert.Equal(t, branches["red+orange"], txBranchID)

	// Message 9
	outputs["J"] = ledgerstate.NewSigLockedSingleOutput(1, wallets["J"].address)
	transactions["8"] = makeTransaction(ledgerstate.NewInputs(inputs["B"]), ledgerstate.NewOutputs(outputs["J"]), outputsByID, walletsByAddress)
	messages["9"] = newTestParentsPayloadMessage(transactions["8"], []MessageID{messages["4"].ID(), messages["7"].ID()}, []MessageID{})
	tangle.Storage.StoreMessage(messages["9"])

	err = tangle.Booker.BookMessage(messages["9"].ID())
	require.NoError(t, err)

	branches["blue"] = ledgerstate.NewBranchID(transactions["8"].ID())
	branches["red+orange+blue"] = ledgerstate.NewAggregatedBranch(ledgerstate.NewBranchIDs(branches["red"], branches["orange"], branches["blue"])).ID()

	msgBranchID, err = messageBranchID(tangle, messages["9"].ID())
	require.NoError(t, err)
	assert.Equal(t, branches["red+orange+blue"], msgBranchID)

	txBranchID, err = transactionBranchID(tangle, transactions["8"].ID())
	require.NoError(t, err)
	assert.Equal(t, branches["blue"], txBranchID)

	msgBranchID, err = messageBranchID(tangle, messages["2"].ID())
	require.NoError(t, err)
	assert.Equal(t, branches["purple"], msgBranchID)

	txBranchID, err = transactionBranchID(tangle, transactions["2"].ID())
	require.NoError(t, err)
	assert.Equal(t, branches["purple"], txBranchID)

	////////// Markers //////////////
	{
		structureDetails := make(map[MessageID]*markers.StructureDetails)
		structureDetails[messages["1"].ID()] = &markers.StructureDetails{
			Rank:          1,
			SequenceID:    1,
			IsPastMarker:  true,
			PastMarkers:   markers.NewMarkers(markers.NewMarker(1, 1)),
			FutureMarkers: markers.NewMarkers(markers.NewMarker(1, 2), markers.NewMarker(3, 2)),
		}
		structureDetails[messages["2"].ID()] = &markers.StructureDetails{
			Rank:          2,
			SequenceID:    1,
			IsPastMarker:  true,
			PastMarkers:   markers.NewMarkers(markers.NewMarker(1, 2)),
			FutureMarkers: markers.NewMarkers(markers.NewMarker(1, 3), markers.NewMarker(2, 3)),
		}
		structureDetails[messages["3"].ID()] = &markers.StructureDetails{
			Rank:          3,
			SequenceID:    1,
			IsPastMarker:  true,
			PastMarkers:   markers.NewMarkers(markers.NewMarker(1, 3)),
			FutureMarkers: markers.NewMarkers(),
		}
		structureDetails[messages["4"].ID()] = &markers.StructureDetails{
			Rank:          2,
			SequenceID:    1,
			PastMarkerGap: 1,
			IsPastMarker:  false,
			PastMarkers:   markers.NewMarkers(markers.NewMarker(1, 1)),
			FutureMarkers: markers.NewMarkers(markers.NewMarker(3, 2)),
		}
		structureDetails[messages["5"].ID()] = &markers.StructureDetails{
			Rank:          3,
			SequenceID:    2,
			IsPastMarker:  true,
			PastMarkers:   markers.NewMarkers(markers.NewMarker(2, 3)),
			FutureMarkers: markers.NewMarkers(markers.NewMarker(2, 4)),
		}
		structureDetails[messages["6"].ID()] = &markers.StructureDetails{
			Rank:          4,
			SequenceID:    2,
			IsPastMarker:  true,
			PastMarkers:   markers.NewMarkers(markers.NewMarker(2, 4)),
			FutureMarkers: markers.NewMarkers(),
		}
		structureDetails[messages["7"].ID()] = &markers.StructureDetails{
			Rank:          3,
			SequenceID:    3,
			IsPastMarker:  true,
			PastMarkers:   markers.NewMarkers(markers.NewMarker(3, 2)),
			FutureMarkers: markers.NewMarkers(markers.NewMarker(3, 3), markers.NewMarker(4, 3)),
		}
		structureDetails[messages["8"].ID()] = &markers.StructureDetails{
			Rank:          4,
			SequenceID:    3,
			IsPastMarker:  true,
			PastMarkers:   markers.NewMarkers(markers.NewMarker(3, 3)),
			FutureMarkers: markers.NewMarkers(),
		}
		structureDetails[messages["9"].ID()] = &markers.StructureDetails{
			Rank:          4,
			SequenceID:    4,
			IsPastMarker:  true,
			PastMarkers:   markers.NewMarkers(markers.NewMarker(4, 3)),
			FutureMarkers: markers.NewMarkers(),
		}

		for alias, message := range messages {
			tangle.Storage.MessageMetadata(message.ID()).Consume(func(metadata *MessageMetadata) {
				assert.Equal(t, structureDetails[message.ID()], metadata.StructureDetails(), "StructureDetails of Message %s are not correct", alias)
			})
		}
	}
	/////////////////////////////////
}

func TestScenario_3(t *testing.T) {
	tangle := newTestTangle()
	defer tangle.Shutdown()
	tangle.Solidifier.Setup()
	tangle.Booker.Setup()

	wallets := make(map[string]wallet)
	walletsByAddress := make(map[ledgerstate.Address]wallet)
	w := createWallets(11)
	wallets["GENESIS"] = w[0]
	wallets["A"] = w[1]
	wallets["B"] = w[2]
	wallets["C"] = w[3]
	wallets["D"] = w[4]
	wallets["E"] = w[5]
	wallets["F"] = w[6]
	wallets["H"] = w[7]
	wallets["I"] = w[8]
	wallets["J"] = w[9]
	wallets["L"] = w[10]
	for _, wallet := range wallets {
		walletsByAddress[wallet.address] = wallet
	}

	genesisBalance := ledgerstate.NewColoredBalances(
		map[ledgerstate.Color]uint64{
			ledgerstate.ColorIOTA: 3,
		})
	genesisEssence := ledgerstate.NewTransactionEssence(
		0,
		time.Unix(DefaultGenesisTime, 0),
		identity.ID{},
		identity.ID{},
		ledgerstate.NewInputs(ledgerstate.NewUTXOInput(ledgerstate.NewOutputID(ledgerstate.GenesisTransactionID, 0))),
		ledgerstate.NewOutputs(ledgerstate.NewSigLockedColoredOutput(genesisBalance, wallets["GENESIS"].address)),
	)

	genesisTransaction := ledgerstate.NewTransaction(genesisEssence, ledgerstate.UnlockBlocks{ledgerstate.NewReferenceUnlockBlock(0)})

	snapshot := &ledgerstate.Snapshot{
		Transactions: map[ledgerstate.TransactionID]ledgerstate.Record{
			genesisTransaction.ID(): {
				Essence:        genesisEssence,
				UnlockBlocks:   ledgerstate.UnlockBlocks{ledgerstate.NewReferenceUnlockBlock(0)},
				UnspentOutputs: []bool{true},
			},
		},
	}

	tangle.LedgerState.LoadSnapshot(snapshot)

	messages := make(map[string]*Message)
	transactions := make(map[string]*ledgerstate.Transaction)
	branches := make(map[string]ledgerstate.BranchID)
	inputs := make(map[string]*ledgerstate.UTXOInput)
	outputs := make(map[string]*ledgerstate.SigLockedSingleOutput)
	outputsByID := make(map[ledgerstate.OutputID]ledgerstate.Output)

	branches["empty"] = ledgerstate.BranchID{}
	branches["green"] = ledgerstate.MasterBranchID
	branches["grey"] = ledgerstate.InvalidBranchID

	// Message 1
	inputs["GENESIS"] = ledgerstate.NewUTXOInput(ledgerstate.NewOutputID(genesisTransaction.ID(), 0))
	outputs["A"] = ledgerstate.NewSigLockedSingleOutput(1, wallets["A"].address)
	outputs["B"] = ledgerstate.NewSigLockedSingleOutput(1, wallets["B"].address)

	outputs["C"] = ledgerstate.NewSigLockedSingleOutput(1, wallets["C"].address)
	transactions["1"] = makeTransaction(ledgerstate.NewInputs(inputs["GENESIS"]), ledgerstate.NewOutputs(outputs["A"], outputs["B"], outputs["C"]), outputsByID, walletsByAddress, wallets["GENESIS"])
	messages["1"] = newTestParentsPayloadMessage(transactions["1"], []MessageID{EmptyMessageID}, []MessageID{})
	tangle.Storage.StoreMessage(messages["1"])

	err := tangle.Booker.BookMessage(messages["1"].ID())

	require.NoError(t, err)

	msgBranchID, err := messageBranchID(tangle, messages["1"].ID())
	require.NoError(t, err)
	assert.Equal(t, branches["green"], msgBranchID)

	txBranchID, err := transactionBranchID(tangle, transactions["1"].ID())
	require.NoError(t, err)
	assert.Equal(t, branches["green"], txBranchID)

	// Message 2
	inputs["B"] = ledgerstate.NewUTXOInput(ledgerstate.NewOutputID(transactions["1"].ID(), selectIndex(transactions["1"], wallets["B"])))
	outputsByID[inputs["B"].ReferencedOutputID()] = ledgerstate.NewOutputs(outputs["B"])[0]
	inputs["C"] = ledgerstate.NewUTXOInput(ledgerstate.NewOutputID(transactions["1"].ID(), selectIndex(transactions["1"], wallets["C"])))
	outputsByID[inputs["C"].ReferencedOutputID()] = ledgerstate.NewOutputs(outputs["C"])[0]
	outputs["E"] = ledgerstate.NewSigLockedSingleOutput(2, wallets["E"].address)
	transactions["2"] = makeTransaction(ledgerstate.NewInputs(inputs["B"], inputs["C"]), ledgerstate.NewOutputs(outputs["E"]), outputsByID, walletsByAddress)
	messages["2"] = newTestParentsPayloadMessage(transactions["2"], []MessageID{EmptyMessageID, messages["1"].ID()}, []MessageID{})
	tangle.Storage.StoreMessage(messages["2"])

	err = tangle.Booker.BookMessage(messages["2"].ID())
	require.NoError(t, err)

	msgBranchID, err = messageBranchID(tangle, messages["2"].ID())
	require.NoError(t, err)
	assert.Equal(t, branches["green"], msgBranchID)

	txBranchID, err = transactionBranchID(tangle, transactions["2"].ID())
	require.NoError(t, err)
	assert.Equal(t, branches["green"], txBranchID)

	// Message 3 (Reattachment of transaction 2)
	messages["3"] = newTestParentsPayloadMessage(transactions["2"], []MessageID{messages["1"].ID(), messages["2"].ID()}, []MessageID{})
	tangle.Storage.StoreMessage(messages["3"])

	err = tangle.Booker.BookMessage(messages["3"].ID())
	require.NoError(t, err)

	msgBranchID, err = messageBranchID(tangle, messages["3"].ID())
	require.NoError(t, err)
	assert.Equal(t, branches["green"], msgBranchID)

	txBranchID, err = transactionBranchID(tangle, transactions["2"].ID())
	require.NoError(t, err)
	assert.Equal(t, branches["green"], txBranchID)

	// Message 4
	inputs["A"] = ledgerstate.NewUTXOInput(ledgerstate.NewOutputID(transactions["1"].ID(), selectIndex(transactions["1"], wallets["A"])))
	outputsByID[inputs["A"].ReferencedOutputID()] = ledgerstate.NewOutputs(outputs["A"])[0]
	outputs["D"] = ledgerstate.NewSigLockedSingleOutput(1, wallets["D"].address)
	transactions["3"] = makeTransaction(ledgerstate.NewInputs(inputs["A"]), ledgerstate.NewOutputs(outputs["D"]), outputsByID, walletsByAddress)
	messages["4"] = newTestParentsPayloadMessage(transactions["3"], []MessageID{EmptyMessageID, messages["1"].ID()}, []MessageID{})
	tangle.Storage.StoreMessage(messages["4"])

	err = tangle.Booker.BookMessage(messages["4"].ID())
	require.NoError(t, err)

	msgBranchID, err = messageBranchID(tangle, messages["4"].ID())
	require.NoError(t, err)
	assert.Equal(t, branches["green"], msgBranchID)

	txBranchID, err = transactionBranchID(tangle, transactions["3"].ID())
	require.NoError(t, err)
	assert.Equal(t, branches["green"], txBranchID)

	// Message 5
	outputs["F"] = ledgerstate.NewSigLockedSingleOutput(1, wallets["F"].address)
	transactions["4"] = makeTransaction(ledgerstate.NewInputs(inputs["A"]), ledgerstate.NewOutputs(outputs["F"]), outputsByID, walletsByAddress)
	messages["5"] = newTestParentsPayloadMessage(transactions["4"], []MessageID{messages["1"].ID()}, []MessageID{messages["2"].ID()})
	tangle.Storage.StoreMessage(messages["5"])

	err = tangle.Booker.BookMessage(messages["5"].ID())
	require.NoError(t, err)

	branches["yellow"] = ledgerstate.NewBranchID(transactions["4"].ID())

	msgBranchID, err = messageBranchID(tangle, messages["5"].ID())
	require.NoError(t, err)
	assert.Equal(t, branches["yellow"], msgBranchID)

	txBranchID, err = transactionBranchID(tangle, transactions["4"].ID())
	require.NoError(t, err)
	assert.Equal(t, branches["yellow"], txBranchID)

	branches["red"] = ledgerstate.NewBranchID(transactions["3"].ID())

	msgBranchID, err = messageBranchID(tangle, messages["4"].ID())
	require.NoError(t, err)
	assert.Equal(t, branches["red"], msgBranchID)

	txBranchID, err = transactionBranchID(tangle, transactions["3"].ID())
	require.NoError(t, err)
	assert.Equal(t, branches["red"], txBranchID)

	// Message 6
	inputs["E"] = ledgerstate.NewUTXOInput(ledgerstate.NewOutputID(transactions["2"].ID(), 0))
	outputsByID[inputs["E"].ReferencedOutputID()] = ledgerstate.NewOutputs(outputs["E"])[0]
	inputs["F"] = ledgerstate.NewUTXOInput(ledgerstate.NewOutputID(transactions["4"].ID(), 0))
	outputsByID[inputs["F"].ReferencedOutputID()] = ledgerstate.NewOutputs(outputs["F"])[0]
	outputs["L"] = ledgerstate.NewSigLockedSingleOutput(3, wallets["L"].address)
	transactions["5"] = makeTransaction(ledgerstate.NewInputs(inputs["E"], inputs["F"]), ledgerstate.NewOutputs(outputs["L"]), outputsByID, walletsByAddress)
	messages["6"] = newTestParentsPayloadMessage(transactions["5"], []MessageID{messages["2"].ID(), messages["5"].ID()}, []MessageID{})
	tangle.Storage.StoreMessage(messages["6"])

	err = tangle.Booker.BookMessage(messages["6"].ID())
	require.NoError(t, err)

	msgBranchID, err = messageBranchID(tangle, messages["6"].ID())
	require.NoError(t, err)
	assert.Equal(t, branches["yellow"], msgBranchID)

	txBranchID, err = transactionBranchID(tangle, transactions["5"].ID())
	require.NoError(t, err)
	assert.Equal(t, branches["yellow"], txBranchID)

	// Message 7
	outputs["H"] = ledgerstate.NewSigLockedSingleOutput(1, wallets["H"].address)
	transactions["6"] = makeTransaction(ledgerstate.NewInputs(inputs["C"]), ledgerstate.NewOutputs(outputs["H"]), outputsByID, walletsByAddress)
	messages["7"] = newTestParentsPayloadMessage(transactions["6"], []MessageID{messages["1"].ID(), messages["4"].ID()}, []MessageID{})
	tangle.Storage.StoreMessage(messages["7"])

	err = tangle.Booker.BookMessage(messages["7"].ID())
	require.NoError(t, err)

	branches["orange"] = ledgerstate.NewBranchID(transactions["6"].ID())
	branches["purple"] = ledgerstate.NewBranchID(transactions["2"].ID())
	branches["red+orange"] = ledgerstate.NewAggregatedBranch(ledgerstate.NewBranchIDs(branches["red"], branches["orange"])).ID()
	branches["yellow+purple"] = ledgerstate.NewAggregatedBranch(ledgerstate.NewBranchIDs(branches["yellow"], branches["purple"])).ID()

	msgBranchID, err = messageBranchID(tangle, messages["7"].ID())
	require.NoError(t, err)
	assert.Equal(t, branches["red+orange"], msgBranchID)

	txBranchID, err = transactionBranchID(tangle, transactions["6"].ID())
	require.NoError(t, err)
	assert.Equal(t, branches["orange"], txBranchID)

	msgBranchID, err = messageBranchID(tangle, messages["2"].ID())
	require.NoError(t, err)
	assert.Equal(t, branches["purple"], msgBranchID)

	txBranchID, err = transactionBranchID(tangle, transactions["2"].ID())
	require.NoError(t, err)
	assert.Equal(t, branches["purple"], txBranchID)

	msgBranchID, err = messageBranchID(tangle, messages["3"].ID())
	require.NoError(t, err)
	assert.Equal(t, branches["purple"], msgBranchID)

	msgBranchID, err = messageBranchID(tangle, messages["5"].ID())
	require.NoError(t, err)
	assert.Equal(t, branches["yellow"], msgBranchID)

	txBranchID, err = transactionBranchID(tangle, transactions["4"].ID())
	require.NoError(t, err)
	assert.Equal(t, branches["yellow"], txBranchID)

	msgBranchID, err = messageBranchID(tangle, messages["6"].ID())
	require.NoError(t, err)
	assert.Equal(t, branches["yellow+purple"], msgBranchID)

	txBranchID, err = transactionBranchID(tangle, transactions["5"].ID())
	require.NoError(t, err)
	assert.Equal(t, branches["yellow+purple"], txBranchID)

	// Message 8
	inputs["H"] = ledgerstate.NewUTXOInput(ledgerstate.NewOutputID(transactions["6"].ID(), 0))
	outputsByID[inputs["H"].ReferencedOutputID()] = ledgerstate.NewOutputs(outputs["H"])[0]
	inputs["D"] = ledgerstate.NewUTXOInput(ledgerstate.NewOutputID(transactions["3"].ID(), 0))
	outputsByID[inputs["D"].ReferencedOutputID()] = ledgerstate.NewOutputs(outputs["D"])[0]
	outputs["I"] = ledgerstate.NewSigLockedSingleOutput(2, wallets["I"].address)
	transactions["7"] = makeTransaction(ledgerstate.NewInputs(inputs["D"], inputs["H"]), ledgerstate.NewOutputs(outputs["I"]), outputsByID, walletsByAddress)
	messages["8"] = newTestParentsPayloadMessage(transactions["7"], []MessageID{messages["4"].ID(), messages["7"].ID()}, []MessageID{})
	tangle.Storage.StoreMessage(messages["8"])

	err = tangle.Booker.BookMessage(messages["8"].ID())
	require.NoError(t, err)

	msgBranchID, err = messageBranchID(tangle, messages["8"].ID())
	require.NoError(t, err)
	assert.Equal(t, branches["red+orange"], msgBranchID)

	txBranchID, err = transactionBranchID(tangle, transactions["7"].ID())
	require.NoError(t, err)
	assert.Equal(t, branches["red+orange"], txBranchID)

	// Message 9
	outputs["J"] = ledgerstate.NewSigLockedSingleOutput(1, wallets["J"].address)
	transactions["8"] = makeTransaction(ledgerstate.NewInputs(inputs["B"]), ledgerstate.NewOutputs(outputs["J"]), outputsByID, walletsByAddress)
	messages["9"] = newTestParentsPayloadMessage(transactions["8"], []MessageID{messages["4"].ID(), messages["7"].ID()}, []MessageID{})
	tangle.Storage.StoreMessage(messages["9"])

	err = tangle.Booker.BookMessage(messages["9"].ID())
	require.NoError(t, err)

	branches["blue"] = ledgerstate.NewBranchID(transactions["8"].ID())
	branches["red+orange+blue"] = ledgerstate.NewAggregatedBranch(ledgerstate.NewBranchIDs(branches["red"], branches["orange"], branches["blue"])).ID()

	msgBranchID, err = messageBranchID(tangle, messages["9"].ID())
	require.NoError(t, err)
	assert.Equal(t, branches["red+orange+blue"], msgBranchID)

	txBranchID, err = transactionBranchID(tangle, transactions["8"].ID())
	require.NoError(t, err)
	assert.Equal(t, branches["blue"], txBranchID)

	msgBranchID, err = messageBranchID(tangle, messages["2"].ID())
	require.NoError(t, err)
	assert.Equal(t, branches["purple"], msgBranchID)

	txBranchID, err = transactionBranchID(tangle, transactions["2"].ID())
	require.NoError(t, err)
	assert.Equal(t, branches["purple"], txBranchID)
}

func TestBookerNewAutomaticSequence(t *testing.T) {
	tangle := newTestTangle()
	defer tangle.Shutdown()

	testFramework := NewMessageTestFramework(tangle)

	tangle.Setup()

	testFramework.CreateMessage("Message1", WithStrongParents("Genesis"))
	testFramework.CreateMessage("Message2", WithStrongParents("Message1"))
	testFramework.IssueMessages("Message1", "Message2").WaitMessagesBooked()

	messageAliases := []string{"Message1"}
	for i := 0; i < 3020; i++ {
		parentMessageAlias := messageAliases[i]
		currentMessageAlias := "Message" + strconv.Itoa(i+3)
		messageAliases = append(messageAliases, currentMessageAlias)
		testFramework.CreateMessage(currentMessageAlias, WithStrongParents(parentMessageAlias))
		testFramework.IssueMessages(currentMessageAlias)
		testFramework.WaitMessagesBooked()
	}

	assert.True(t, tangle.Storage.MessageMetadata(testFramework.Message("Message3009").ID()).Consume(func(messageMetadata *MessageMetadata) {
		assert.Equal(t, &markers.StructureDetails{
			Rank:          3008,
			PastMarkerGap: 0,
			IsPastMarker:  true,
			SequenceID:    2,
			PastMarkers:   markers.NewMarkers(markers.NewMarker(2, 9)),
			FutureMarkers: markers.NewMarkers(markers.NewMarker(2, 10)),
		}, messageMetadata.StructureDetails())
	}))
}

func TestBookerMarkerMappings(t *testing.T) {
	tangle := newTestTangle()
	defer tangle.Shutdown()

	testFramework := NewMessageTestFramework(
		tangle,
		WithGenesisOutput("A", 500),
		WithGenesisOutput("B", 500),
		WithGenesisOutput("C", 500),
	)

	tangle.Setup()

	branchIDs := make(map[string]ledgerstate.BranchID)

	// ISSUE Message1
	{
		testFramework.CreateMessage("Message1", WithStrongParents("Genesis"), WithInputs("A"), WithOutput("G", 500))
		testFramework.IssueMessages("Message1").WaitMessagesBooked()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1": markers.NewMarkers(markers.NewMarker(1, 1)),
		})
		checkMessageMetadataBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1": ledgerstate.UndefinedBranchID,
		})
		checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1": ledgerstate.MasterBranchID,
		})
	}

	// ISSUE Message2
	{
		testFramework.CreateMessage("Message2", WithStrongParents("Genesis"), WithInputs("A", "B"), WithOutput("E", 1000))
		testFramework.IssueMessages("Message2").WaitMessagesBooked()

		branchIDs["A"] = ledgerstate.NewBranchID(testFramework.TransactionID("Message1"))
		branchIDs["B"] = ledgerstate.NewBranchID(testFramework.TransactionID("Message2"))

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1": markers.NewMarkers(markers.NewMarker(1, 1)),
			"Message2": markers.NewMarkers(markers.NewMarker(2, 1)),
		})
		checkMessageMetadataBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1": ledgerstate.UndefinedBranchID,
			"Message2": ledgerstate.UndefinedBranchID,
		})
		checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1": branchIDs["A"],
			"Message2": branchIDs["B"],
		})
	}

	// ISSUE Message3
	{
		testFramework.CreateMessage("Message3", WithStrongParents("Genesis"), WithInputs("B"), WithOutput("F", 500))
		testFramework.IssueMessages("Message3").WaitMessagesBooked()

		branchIDs["C"] = ledgerstate.NewBranchID(testFramework.TransactionID("Message3"))

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1": markers.NewMarkers(markers.NewMarker(1, 1)),
			"Message2": markers.NewMarkers(markers.NewMarker(2, 1)),
			"Message3": markers.NewMarkers(markers.NewMarker(3, 1)),
		})
		checkMessageMetadataBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1": ledgerstate.UndefinedBranchID,
			"Message2": ledgerstate.UndefinedBranchID,
			"Message3": ledgerstate.UndefinedBranchID,
		})
		checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1": branchIDs["A"],
			"Message2": branchIDs["B"],
			"Message3": branchIDs["C"],
		})
	}

	// ISSUE Message4
	{
		testFramework.CreateMessage("Message4", WithStrongParents("Genesis"))
		testFramework.IssueMessages("Message4").WaitMessagesBooked()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1": markers.NewMarkers(markers.NewMarker(1, 1)),
			"Message2": markers.NewMarkers(markers.NewMarker(2, 1)),
			"Message3": markers.NewMarkers(markers.NewMarker(3, 1)),
			"Message4": markers.NewMarkers(markers.NewMarker(4, 1)),
		})
		checkMessageMetadataBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1": ledgerstate.UndefinedBranchID,
			"Message2": ledgerstate.UndefinedBranchID,
			"Message3": ledgerstate.UndefinedBranchID,
			"Message4": ledgerstate.UndefinedBranchID,
		})
		checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1": branchIDs["A"],
			"Message2": branchIDs["B"],
			"Message3": branchIDs["C"],
			"Message4": ledgerstate.MasterBranchID,
		})
	}

	// ISSUE Message5
	{
		testFramework.CreateMessage("Message5", WithStrongParents("Message4"), WithInputs("C"), WithOutput("D", 500))
		testFramework.IssueMessages("Message5").WaitMessagesBooked()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1": markers.NewMarkers(markers.NewMarker(1, 1)),
			"Message2": markers.NewMarkers(markers.NewMarker(2, 1)),
			"Message3": markers.NewMarkers(markers.NewMarker(3, 1)),
			"Message4": markers.NewMarkers(markers.NewMarker(4, 1)),
			"Message5": markers.NewMarkers(markers.NewMarker(4, 2)),
		})
		checkMessageMetadataBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1": ledgerstate.UndefinedBranchID,
			"Message2": ledgerstate.UndefinedBranchID,
			"Message3": ledgerstate.UndefinedBranchID,
			"Message4": ledgerstate.UndefinedBranchID,
			"Message5": ledgerstate.UndefinedBranchID,
		})
		checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1": branchIDs["A"],
			"Message2": branchIDs["B"],
			"Message3": branchIDs["C"],
			"Message4": ledgerstate.MasterBranchID,
			"Message5": ledgerstate.MasterBranchID,
		})
	}

	// ISSUE Message6
	{
		testFramework.CreateMessage("Message6", WithStrongParents("Message1"), WithInputs("G"), WithOutput("I", 500))
		testFramework.IssueMessages("Message6").WaitMessagesBooked()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1": markers.NewMarkers(markers.NewMarker(1, 1)),
			"Message2": markers.NewMarkers(markers.NewMarker(2, 1)),
			"Message3": markers.NewMarkers(markers.NewMarker(3, 1)),
			"Message4": markers.NewMarkers(markers.NewMarker(4, 1)),
			"Message5": markers.NewMarkers(markers.NewMarker(4, 2)),
			"Message6": markers.NewMarkers(markers.NewMarker(1, 2)),
		})
		checkMessageMetadataBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1": ledgerstate.UndefinedBranchID,
			"Message2": ledgerstate.UndefinedBranchID,
			"Message3": ledgerstate.UndefinedBranchID,
			"Message4": ledgerstate.UndefinedBranchID,
			"Message5": ledgerstate.UndefinedBranchID,
			"Message6": ledgerstate.UndefinedBranchID,
		})
		checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1": branchIDs["A"],
			"Message2": branchIDs["B"],
			"Message3": branchIDs["C"],
			"Message4": ledgerstate.MasterBranchID,
			"Message5": ledgerstate.MasterBranchID,
			"Message6": branchIDs["A"],
		})
	}

	// ISSUE Message7
	{
		testFramework.CreateMessage("Message7", WithStrongParents("Message3"))
		testFramework.IssueMessages("Message7").WaitMessagesBooked()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1": markers.NewMarkers(markers.NewMarker(1, 1)),
			"Message2": markers.NewMarkers(markers.NewMarker(2, 1)),
			"Message3": markers.NewMarkers(markers.NewMarker(3, 1)),
			"Message4": markers.NewMarkers(markers.NewMarker(4, 1)),
			"Message5": markers.NewMarkers(markers.NewMarker(4, 2)),
			"Message6": markers.NewMarkers(markers.NewMarker(1, 2)),
			"Message7": markers.NewMarkers(markers.NewMarker(3, 2)),
		})
		checkMessageMetadataBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1": ledgerstate.UndefinedBranchID,
			"Message2": ledgerstate.UndefinedBranchID,
			"Message3": ledgerstate.UndefinedBranchID,
			"Message4": ledgerstate.UndefinedBranchID,
			"Message5": ledgerstate.UndefinedBranchID,
			"Message6": ledgerstate.UndefinedBranchID,
			"Message7": ledgerstate.UndefinedBranchID,
		})
		checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1": branchIDs["A"],
			"Message2": branchIDs["B"],
			"Message3": branchIDs["C"],
			"Message4": ledgerstate.MasterBranchID,
			"Message5": ledgerstate.MasterBranchID,
			"Message6": branchIDs["A"],
			"Message7": branchIDs["C"],
		})
	}

	// ISSUE Message8
	{
		testFramework.CreateMessage("Message8", WithStrongParents("Message1", "Message6"))
		testFramework.IssueMessages("Message8").WaitMessagesBooked()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1": markers.NewMarkers(markers.NewMarker(1, 1)),
			"Message2": markers.NewMarkers(markers.NewMarker(2, 1)),
			"Message3": markers.NewMarkers(markers.NewMarker(3, 1)),
			"Message4": markers.NewMarkers(markers.NewMarker(4, 1)),
			"Message5": markers.NewMarkers(markers.NewMarker(4, 2)),
			"Message6": markers.NewMarkers(markers.NewMarker(1, 2)),
			"Message7": markers.NewMarkers(markers.NewMarker(3, 2)),
			"Message8": markers.NewMarkers(markers.NewMarker(1, 3)),
		})
		checkMessageMetadataBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1": ledgerstate.UndefinedBranchID,
			"Message2": ledgerstate.UndefinedBranchID,
			"Message3": ledgerstate.UndefinedBranchID,
			"Message4": ledgerstate.UndefinedBranchID,
			"Message5": ledgerstate.UndefinedBranchID,
			"Message6": ledgerstate.UndefinedBranchID,
			"Message7": ledgerstate.UndefinedBranchID,
			"Message8": ledgerstate.UndefinedBranchID,
		})
		checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1": branchIDs["A"],
			"Message2": branchIDs["B"],
			"Message3": branchIDs["C"],
			"Message4": ledgerstate.MasterBranchID,
			"Message5": ledgerstate.MasterBranchID,
			"Message6": branchIDs["A"],
			"Message7": branchIDs["C"],
			"Message8": branchIDs["A"],
		})
	}

	// ISSUE Message9
	{
		testFramework.CreateMessage("Message9", WithStrongParents("Message7"))
		testFramework.IssueMessages("Message9").WaitMessagesBooked()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1": markers.NewMarkers(markers.NewMarker(1, 1)),
			"Message2": markers.NewMarkers(markers.NewMarker(2, 1)),
			"Message3": markers.NewMarkers(markers.NewMarker(3, 1)),
			"Message4": markers.NewMarkers(markers.NewMarker(4, 1)),
			"Message5": markers.NewMarkers(markers.NewMarker(4, 2)),
			"Message6": markers.NewMarkers(markers.NewMarker(1, 2)),
			"Message7": markers.NewMarkers(markers.NewMarker(3, 2)),
			"Message8": markers.NewMarkers(markers.NewMarker(1, 3)),
			"Message9": markers.NewMarkers(markers.NewMarker(3, 3)),
		})
		checkMessageMetadataBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1": ledgerstate.UndefinedBranchID,
			"Message2": ledgerstate.UndefinedBranchID,
			"Message3": ledgerstate.UndefinedBranchID,
			"Message4": ledgerstate.UndefinedBranchID,
			"Message5": ledgerstate.UndefinedBranchID,
			"Message6": ledgerstate.UndefinedBranchID,
			"Message7": ledgerstate.UndefinedBranchID,
			"Message8": ledgerstate.UndefinedBranchID,
			"Message9": ledgerstate.UndefinedBranchID,
		})
		checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1": branchIDs["A"],
			"Message2": branchIDs["B"],
			"Message3": branchIDs["C"],
			"Message4": ledgerstate.MasterBranchID,
			"Message5": ledgerstate.MasterBranchID,
			"Message6": branchIDs["A"],
			"Message7": branchIDs["C"],
			"Message8": branchIDs["A"],
			"Message9": branchIDs["C"],
		})
	}

	// ISSUE Message10
	{
		testFramework.CreateMessage("Message10", WithStrongParents("Message2"), WithInputs("C"), WithOutput("H", 500))
		testFramework.IssueMessages("Message10").WaitMessagesBooked()

		branchIDs["D"] = ledgerstate.NewBranchID(testFramework.TransactionID("Message5"))
		branchIDs["E"] = ledgerstate.NewBranchID(testFramework.TransactionID("Message10"))
		branchIDs["B+E"] = aggregatedBranchID(branchIDs["B"], branchIDs["E"])

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1":  markers.NewMarkers(markers.NewMarker(1, 1)),
			"Message2":  markers.NewMarkers(markers.NewMarker(2, 1)),
			"Message3":  markers.NewMarkers(markers.NewMarker(3, 1)),
			"Message4":  markers.NewMarkers(markers.NewMarker(4, 1)),
			"Message5":  markers.NewMarkers(markers.NewMarker(4, 2)),
			"Message6":  markers.NewMarkers(markers.NewMarker(1, 2)),
			"Message7":  markers.NewMarkers(markers.NewMarker(3, 2)),
			"Message8":  markers.NewMarkers(markers.NewMarker(1, 3)),
			"Message9":  markers.NewMarkers(markers.NewMarker(3, 3)),
			"Message10": markers.NewMarkers(markers.NewMarker(5, 2)),
		})
		checkMessageMetadataBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1":  ledgerstate.UndefinedBranchID,
			"Message2":  ledgerstate.UndefinedBranchID,
			"Message3":  ledgerstate.UndefinedBranchID,
			"Message4":  ledgerstate.UndefinedBranchID,
			"Message5":  ledgerstate.UndefinedBranchID,
			"Message6":  ledgerstate.UndefinedBranchID,
			"Message7":  ledgerstate.UndefinedBranchID,
			"Message8":  ledgerstate.UndefinedBranchID,
			"Message9":  ledgerstate.UndefinedBranchID,
			"Message10": ledgerstate.UndefinedBranchID,
		})
		checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1":  branchIDs["A"],
			"Message2":  branchIDs["B"],
			"Message3":  branchIDs["C"],
			"Message4":  ledgerstate.MasterBranchID,
			"Message5":  branchIDs["D"],
			"Message6":  branchIDs["A"],
			"Message7":  branchIDs["C"],
			"Message8":  branchIDs["A"],
			"Message9":  branchIDs["C"],
			"Message10": branchIDs["B+E"],
		})
	}

	// ISSUE Message11
	{
		testFramework.CreateMessage("Message11", WithStrongParents("Message8", "Message9"))
		testFramework.IssueMessages("Message11").WaitMessagesBooked()

		branchIDs["A+C"] = aggregatedBranchID(branchIDs["A"], branchIDs["C"])

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1":  markers.NewMarkers(markers.NewMarker(1, 1)),
			"Message2":  markers.NewMarkers(markers.NewMarker(2, 1)),
			"Message3":  markers.NewMarkers(markers.NewMarker(3, 1)),
			"Message4":  markers.NewMarkers(markers.NewMarker(4, 1)),
			"Message5":  markers.NewMarkers(markers.NewMarker(4, 2)),
			"Message6":  markers.NewMarkers(markers.NewMarker(1, 2)),
			"Message7":  markers.NewMarkers(markers.NewMarker(3, 2)),
			"Message8":  markers.NewMarkers(markers.NewMarker(1, 3)),
			"Message9":  markers.NewMarkers(markers.NewMarker(3, 3)),
			"Message10": markers.NewMarkers(markers.NewMarker(5, 2)),
			"Message11": markers.NewMarkers(markers.NewMarker(6, 4)),
		})
		checkMessageMetadataBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1":  ledgerstate.UndefinedBranchID,
			"Message2":  ledgerstate.UndefinedBranchID,
			"Message3":  ledgerstate.UndefinedBranchID,
			"Message4":  ledgerstate.UndefinedBranchID,
			"Message5":  ledgerstate.UndefinedBranchID,
			"Message6":  ledgerstate.UndefinedBranchID,
			"Message7":  ledgerstate.UndefinedBranchID,
			"Message8":  ledgerstate.UndefinedBranchID,
			"Message9":  ledgerstate.UndefinedBranchID,
			"Message10": ledgerstate.UndefinedBranchID,
			"Message11": ledgerstate.UndefinedBranchID,
		})
		checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1":  branchIDs["A"],
			"Message2":  branchIDs["B"],
			"Message3":  branchIDs["C"],
			"Message4":  ledgerstate.MasterBranchID,
			"Message5":  branchIDs["D"],
			"Message6":  branchIDs["A"],
			"Message7":  branchIDs["C"],
			"Message8":  branchIDs["A"],
			"Message9":  branchIDs["C"],
			"Message10": branchIDs["B+E"],
			"Message11": branchIDs["A+C"],
		})
	}

	// ISSUE Message12
	{
		testFramework.CreateMessage("Message12", WithStrongParents("Message8", "Message9"))
		testFramework.IssueMessages("Message12").WaitMessagesBooked()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1":  markers.NewMarkers(markers.NewMarker(1, 1)),
			"Message2":  markers.NewMarkers(markers.NewMarker(2, 1)),
			"Message3":  markers.NewMarkers(markers.NewMarker(3, 1)),
			"Message4":  markers.NewMarkers(markers.NewMarker(4, 1)),
			"Message5":  markers.NewMarkers(markers.NewMarker(4, 2)),
			"Message6":  markers.NewMarkers(markers.NewMarker(1, 2)),
			"Message7":  markers.NewMarkers(markers.NewMarker(3, 2)),
			"Message8":  markers.NewMarkers(markers.NewMarker(1, 3)),
			"Message9":  markers.NewMarkers(markers.NewMarker(3, 3)),
			"Message10": markers.NewMarkers(markers.NewMarker(5, 2)),
			"Message11": markers.NewMarkers(markers.NewMarker(6, 4)),
			"Message12": markers.NewMarkers(markers.NewMarker(1, 3), markers.NewMarker(3, 3)),
		})
		checkMessageMetadataBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1":  ledgerstate.UndefinedBranchID,
			"Message2":  ledgerstate.UndefinedBranchID,
			"Message3":  ledgerstate.UndefinedBranchID,
			"Message4":  ledgerstate.UndefinedBranchID,
			"Message5":  ledgerstate.UndefinedBranchID,
			"Message6":  ledgerstate.UndefinedBranchID,
			"Message7":  ledgerstate.UndefinedBranchID,
			"Message8":  ledgerstate.UndefinedBranchID,
			"Message9":  ledgerstate.UndefinedBranchID,
			"Message10": ledgerstate.UndefinedBranchID,
			"Message11": ledgerstate.UndefinedBranchID,
			"Message12": branchIDs["A+C"],
		})
		checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1":  branchIDs["A"],
			"Message2":  branchIDs["B"],
			"Message3":  branchIDs["C"],
			"Message4":  ledgerstate.MasterBranchID,
			"Message5":  branchIDs["D"],
			"Message6":  branchIDs["A"],
			"Message7":  branchIDs["C"],
			"Message8":  branchIDs["A"],
			"Message9":  branchIDs["C"],
			"Message10": branchIDs["B+E"],
			"Message11": branchIDs["A+C"],
			"Message12": branchIDs["A+C"],
		})
		checkIndividuallyMappedMessages(t, testFramework, map[ledgerstate.BranchID][]string{
			branchIDs["A+C"]: {"Message12"},
		})
	}

	// ISSUE Message13
	{
		testFramework.CreateMessage("Message13", WithStrongParents("Message6", "Message7"), WithWeakParents("Message10"))
		testFramework.IssueMessages("Message13").WaitMessagesBooked()

		branchIDs["A+C+E"] = aggregatedBranchID(branchIDs["A"], branchIDs["C"], branchIDs["E"])

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1":  markers.NewMarkers(markers.NewMarker(1, 1)),
			"Message2":  markers.NewMarkers(markers.NewMarker(2, 1)),
			"Message3":  markers.NewMarkers(markers.NewMarker(3, 1)),
			"Message4":  markers.NewMarkers(markers.NewMarker(4, 1)),
			"Message5":  markers.NewMarkers(markers.NewMarker(4, 2)),
			"Message6":  markers.NewMarkers(markers.NewMarker(1, 2)),
			"Message7":  markers.NewMarkers(markers.NewMarker(3, 2)),
			"Message8":  markers.NewMarkers(markers.NewMarker(1, 3)),
			"Message9":  markers.NewMarkers(markers.NewMarker(3, 3)),
			"Message10": markers.NewMarkers(markers.NewMarker(5, 2)),
			"Message11": markers.NewMarkers(markers.NewMarker(6, 4)),
			"Message12": markers.NewMarkers(markers.NewMarker(1, 3), markers.NewMarker(3, 3)),
			"Message13": markers.NewMarkers(markers.NewMarker(7, 3)),
		})
		checkMessageMetadataBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1":  ledgerstate.UndefinedBranchID,
			"Message2":  ledgerstate.UndefinedBranchID,
			"Message3":  ledgerstate.UndefinedBranchID,
			"Message4":  ledgerstate.UndefinedBranchID,
			"Message5":  ledgerstate.UndefinedBranchID,
			"Message6":  ledgerstate.UndefinedBranchID,
			"Message7":  ledgerstate.UndefinedBranchID,
			"Message8":  ledgerstate.UndefinedBranchID,
			"Message9":  ledgerstate.UndefinedBranchID,
			"Message10": ledgerstate.UndefinedBranchID,
			"Message11": ledgerstate.UndefinedBranchID,
			"Message12": branchIDs["A+C"],
			"Message13": ledgerstate.UndefinedBranchID,
		})
		checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1":  branchIDs["A"],
			"Message2":  branchIDs["B"],
			"Message3":  branchIDs["C"],
			"Message4":  ledgerstate.MasterBranchID,
			"Message5":  branchIDs["D"],
			"Message6":  branchIDs["A"],
			"Message7":  branchIDs["C"],
			"Message8":  branchIDs["A"],
			"Message9":  branchIDs["C"],
			"Message10": branchIDs["B+E"],
			"Message11": branchIDs["A+C"],
			"Message12": branchIDs["A+C"],
			"Message13": branchIDs["A+C+E"],
		})
		checkIndividuallyMappedMessages(t, testFramework, map[ledgerstate.BranchID][]string{
			branchIDs["A+C"]: {"Message12"},
		})
	}

	// ISSUE Message14
	{
		testFramework.CreateMessage("Message14", WithStrongParents("Message12"), WithInputs("I"), WithOutput("J", 500))
		testFramework.IssueMessages("Message14").WaitMessagesBooked()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1":  markers.NewMarkers(markers.NewMarker(1, 1)),
			"Message2":  markers.NewMarkers(markers.NewMarker(2, 1)),
			"Message3":  markers.NewMarkers(markers.NewMarker(3, 1)),
			"Message4":  markers.NewMarkers(markers.NewMarker(4, 1)),
			"Message5":  markers.NewMarkers(markers.NewMarker(4, 2)),
			"Message6":  markers.NewMarkers(markers.NewMarker(1, 2)),
			"Message7":  markers.NewMarkers(markers.NewMarker(3, 2)),
			"Message8":  markers.NewMarkers(markers.NewMarker(1, 3)),
			"Message9":  markers.NewMarkers(markers.NewMarker(3, 3)),
			"Message10": markers.NewMarkers(markers.NewMarker(5, 2)),
			"Message11": markers.NewMarkers(markers.NewMarker(6, 4)),
			"Message12": markers.NewMarkers(markers.NewMarker(1, 3), markers.NewMarker(3, 3)),
			"Message13": markers.NewMarkers(markers.NewMarker(7, 3)),
			"Message14": markers.NewMarkers(markers.NewMarker(1, 3), markers.NewMarker(3, 3)),
		})
		checkMessageMetadataBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1":  ledgerstate.UndefinedBranchID,
			"Message2":  ledgerstate.UndefinedBranchID,
			"Message3":  ledgerstate.UndefinedBranchID,
			"Message4":  ledgerstate.UndefinedBranchID,
			"Message5":  ledgerstate.UndefinedBranchID,
			"Message6":  ledgerstate.UndefinedBranchID,
			"Message7":  ledgerstate.UndefinedBranchID,
			"Message8":  ledgerstate.UndefinedBranchID,
			"Message9":  ledgerstate.UndefinedBranchID,
			"Message10": ledgerstate.UndefinedBranchID,
			"Message11": ledgerstate.UndefinedBranchID,
			"Message12": branchIDs["A+C"],
			"Message13": ledgerstate.UndefinedBranchID,
			"Message14": branchIDs["A+C"],
		})
		checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1":  branchIDs["A"],
			"Message2":  branchIDs["B"],
			"Message3":  branchIDs["C"],
			"Message4":  ledgerstate.MasterBranchID,
			"Message5":  branchIDs["D"],
			"Message6":  branchIDs["A"],
			"Message7":  branchIDs["C"],
			"Message8":  branchIDs["A"],
			"Message9":  branchIDs["C"],
			"Message10": branchIDs["B+E"],
			"Message11": branchIDs["A+C"],
			"Message12": branchIDs["A+C"],
			"Message13": branchIDs["A+C+E"],
			"Message14": branchIDs["A+C"],
		})
		checkIndividuallyMappedMessages(t, testFramework, map[ledgerstate.BranchID][]string{
			branchIDs["A+C"]: {"Message12", "Message14"},
		})
	}

	// ISSUE Message15
	{
		testFramework.CreateMessage("Message15", WithStrongParents("Message11", "Message14"))
		testFramework.PreventNewMarkers(true).IssueMessages("Message15").WaitMessagesBooked().PreventNewMarkers(false)

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1":  markers.NewMarkers(markers.NewMarker(1, 1)),
			"Message2":  markers.NewMarkers(markers.NewMarker(2, 1)),
			"Message3":  markers.NewMarkers(markers.NewMarker(3, 1)),
			"Message4":  markers.NewMarkers(markers.NewMarker(4, 1)),
			"Message5":  markers.NewMarkers(markers.NewMarker(4, 2)),
			"Message6":  markers.NewMarkers(markers.NewMarker(1, 2)),
			"Message7":  markers.NewMarkers(markers.NewMarker(3, 2)),
			"Message8":  markers.NewMarkers(markers.NewMarker(1, 3)),
			"Message9":  markers.NewMarkers(markers.NewMarker(3, 3)),
			"Message10": markers.NewMarkers(markers.NewMarker(5, 2)),
			"Message11": markers.NewMarkers(markers.NewMarker(6, 4)),
			"Message12": markers.NewMarkers(markers.NewMarker(1, 3), markers.NewMarker(3, 3)),
			"Message13": markers.NewMarkers(markers.NewMarker(7, 3)),
			"Message14": markers.NewMarkers(markers.NewMarker(1, 3), markers.NewMarker(3, 3)),
			"Message15": markers.NewMarkers(markers.NewMarker(6, 4)),
		})
		checkMessageMetadataBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1":  ledgerstate.UndefinedBranchID,
			"Message2":  ledgerstate.UndefinedBranchID,
			"Message3":  ledgerstate.UndefinedBranchID,
			"Message4":  ledgerstate.UndefinedBranchID,
			"Message5":  ledgerstate.UndefinedBranchID,
			"Message6":  ledgerstate.UndefinedBranchID,
			"Message7":  ledgerstate.UndefinedBranchID,
			"Message8":  ledgerstate.UndefinedBranchID,
			"Message9":  ledgerstate.UndefinedBranchID,
			"Message10": ledgerstate.UndefinedBranchID,
			"Message11": ledgerstate.UndefinedBranchID,
			"Message12": branchIDs["A+C"],
			"Message13": ledgerstate.UndefinedBranchID,
			"Message14": branchIDs["A+C"],
			"Message15": ledgerstate.UndefinedBranchID,
		})
		checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1":  branchIDs["A"],
			"Message2":  branchIDs["B"],
			"Message3":  branchIDs["C"],
			"Message4":  ledgerstate.MasterBranchID,
			"Message5":  branchIDs["D"],
			"Message6":  branchIDs["A"],
			"Message7":  branchIDs["C"],
			"Message8":  branchIDs["A"],
			"Message9":  branchIDs["C"],
			"Message10": branchIDs["B+E"],
			"Message11": branchIDs["A+C"],
			"Message12": branchIDs["A+C"],
			"Message13": branchIDs["A+C+E"],
			"Message14": branchIDs["A+C"],
			"Message15": branchIDs["A+C"],
		})
		checkIndividuallyMappedMessages(t, testFramework, map[ledgerstate.BranchID][]string{
			branchIDs["A+C"]: {"Message12", "Message14"},
		})
	}

	// ISSUE Message16
	{
		testFramework.CreateMessage("Message16", WithStrongParents("Message15"))
		testFramework.IssueMessages("Message16").WaitMessagesBooked()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1":  markers.NewMarkers(markers.NewMarker(1, 1)),
			"Message2":  markers.NewMarkers(markers.NewMarker(2, 1)),
			"Message3":  markers.NewMarkers(markers.NewMarker(3, 1)),
			"Message4":  markers.NewMarkers(markers.NewMarker(4, 1)),
			"Message5":  markers.NewMarkers(markers.NewMarker(4, 2)),
			"Message6":  markers.NewMarkers(markers.NewMarker(1, 2)),
			"Message7":  markers.NewMarkers(markers.NewMarker(3, 2)),
			"Message8":  markers.NewMarkers(markers.NewMarker(1, 3)),
			"Message9":  markers.NewMarkers(markers.NewMarker(3, 3)),
			"Message10": markers.NewMarkers(markers.NewMarker(5, 2)),
			"Message11": markers.NewMarkers(markers.NewMarker(6, 4)),
			"Message12": markers.NewMarkers(markers.NewMarker(1, 3), markers.NewMarker(3, 3)),
			"Message13": markers.NewMarkers(markers.NewMarker(7, 3)),
			"Message14": markers.NewMarkers(markers.NewMarker(1, 3), markers.NewMarker(3, 3)),
			"Message15": markers.NewMarkers(markers.NewMarker(6, 4)),
			"Message16": markers.NewMarkers(markers.NewMarker(6, 5)),
		})
		checkMessageMetadataBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1":  ledgerstate.UndefinedBranchID,
			"Message2":  ledgerstate.UndefinedBranchID,
			"Message3":  ledgerstate.UndefinedBranchID,
			"Message4":  ledgerstate.UndefinedBranchID,
			"Message5":  ledgerstate.UndefinedBranchID,
			"Message6":  ledgerstate.UndefinedBranchID,
			"Message7":  ledgerstate.UndefinedBranchID,
			"Message8":  ledgerstate.UndefinedBranchID,
			"Message9":  ledgerstate.UndefinedBranchID,
			"Message10": ledgerstate.UndefinedBranchID,
			"Message11": ledgerstate.UndefinedBranchID,
			"Message12": branchIDs["A+C"],
			"Message13": ledgerstate.UndefinedBranchID,
			"Message14": branchIDs["A+C"],
			"Message15": ledgerstate.UndefinedBranchID,
			"Message16": ledgerstate.UndefinedBranchID,
		})
		checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1":  branchIDs["A"],
			"Message2":  branchIDs["B"],
			"Message3":  branchIDs["C"],
			"Message4":  ledgerstate.MasterBranchID,
			"Message5":  branchIDs["D"],
			"Message6":  branchIDs["A"],
			"Message7":  branchIDs["C"],
			"Message8":  branchIDs["A"],
			"Message9":  branchIDs["C"],
			"Message10": branchIDs["B+E"],
			"Message11": branchIDs["A+C"],
			"Message12": branchIDs["A+C"],
			"Message13": branchIDs["A+C+E"],
			"Message14": branchIDs["A+C"],
			"Message15": branchIDs["A+C"],
			"Message16": branchIDs["A+C"],
		})
		checkIndividuallyMappedMessages(t, testFramework, map[ledgerstate.BranchID][]string{
			branchIDs["A+C"]: {"Message12", "Message14"},
		})
	}

	// ISSUE Message17
	{
		testFramework.CreateMessage("Message17", WithStrongParents("Message1"), WithInputs("G"), WithOutput("K", 500))

		branchIDs["F"] = ledgerstate.NewBranchID(testFramework.TransactionID("Message6"))
		branchIDs["G"] = ledgerstate.NewBranchID(testFramework.TransactionID("Message17"))
		branchIDs["F+C"] = aggregatedBranchID(branchIDs["F"], branchIDs["C"])
		branchIDs["F+C+E"] = aggregatedBranchID(branchIDs["F"], branchIDs["C"], branchIDs["E"])

		for alias, branchID := range branchIDs {
			ledgerstate.RegisterBranchIDAlias(branchID, alias)
		}

		testFramework.IssueMessages("Message17").WaitMessagesBooked()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1":  markers.NewMarkers(markers.NewMarker(1, 1)),
			"Message2":  markers.NewMarkers(markers.NewMarker(2, 1)),
			"Message3":  markers.NewMarkers(markers.NewMarker(3, 1)),
			"Message4":  markers.NewMarkers(markers.NewMarker(4, 1)),
			"Message5":  markers.NewMarkers(markers.NewMarker(4, 2)),
			"Message6":  markers.NewMarkers(markers.NewMarker(1, 2)),
			"Message7":  markers.NewMarkers(markers.NewMarker(3, 2)),
			"Message8":  markers.NewMarkers(markers.NewMarker(1, 3)),
			"Message9":  markers.NewMarkers(markers.NewMarker(3, 3)),
			"Message10": markers.NewMarkers(markers.NewMarker(5, 2)),
			"Message11": markers.NewMarkers(markers.NewMarker(6, 4)),
			"Message12": markers.NewMarkers(markers.NewMarker(1, 3), markers.NewMarker(3, 3)),
			"Message13": markers.NewMarkers(markers.NewMarker(7, 3)),
			"Message14": markers.NewMarkers(markers.NewMarker(1, 3), markers.NewMarker(3, 3)),
			"Message15": markers.NewMarkers(markers.NewMarker(6, 4)),
			"Message16": markers.NewMarkers(markers.NewMarker(6, 5)),
			"Message17": markers.NewMarkers(markers.NewMarker(8, 2)),
		})
		checkMessageMetadataBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1":  ledgerstate.UndefinedBranchID,
			"Message2":  ledgerstate.UndefinedBranchID,
			"Message3":  ledgerstate.UndefinedBranchID,
			"Message4":  ledgerstate.UndefinedBranchID,
			"Message5":  ledgerstate.UndefinedBranchID,
			"Message6":  ledgerstate.UndefinedBranchID,
			"Message7":  ledgerstate.UndefinedBranchID,
			"Message8":  ledgerstate.UndefinedBranchID,
			"Message9":  ledgerstate.UndefinedBranchID,
			"Message10": ledgerstate.UndefinedBranchID,
			"Message11": ledgerstate.UndefinedBranchID,
			"Message12": branchIDs["F+C"],
			"Message13": ledgerstate.UndefinedBranchID,
			"Message14": branchIDs["F+C"],
			"Message15": ledgerstate.UndefinedBranchID,
			"Message16": ledgerstate.UndefinedBranchID,
			"Message17": ledgerstate.UndefinedBranchID,
		})
		checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1":  branchIDs["A"],
			"Message2":  branchIDs["B"],
			"Message3":  branchIDs["C"],
			"Message4":  ledgerstate.MasterBranchID,
			"Message5":  branchIDs["D"],
			"Message6":  branchIDs["F"],
			"Message7":  branchIDs["C"],
			"Message8":  branchIDs["F"],
			"Message9":  branchIDs["C"],
			"Message10": branchIDs["B+E"],
			"Message11": branchIDs["F+C"],
			"Message12": branchIDs["F+C"],
			"Message13": branchIDs["F+C+E"],
			"Message14": branchIDs["F+C"],
			"Message15": branchIDs["F+C"],
			"Message16": branchIDs["F+C"],
			"Message17": branchIDs["G"],
		})
		checkIndividuallyMappedMessages(t, testFramework, map[ledgerstate.BranchID][]string{
			branchIDs["F+C"]: {"Message12", "Message14"},
		})
	}

	// ISSUE Message18
	{
		testFramework.CreateMessage("Message18", WithStrongParents("Message12"), WithInputs("I"), WithOutput("L", 500))

		branchIDs["H"] = ledgerstate.NewBranchID(testFramework.TransactionID("Message14"))
		branchIDs["H+C"] = aggregatedBranchID(branchIDs["H"], branchIDs["C"])
		branchIDs["I"] = ledgerstate.NewBranchID(testFramework.TransactionID("Message18"))
		branchIDs["I+C"] = aggregatedBranchID(branchIDs["I"], branchIDs["C"])

		for alias, branchID := range branchIDs {
			ledgerstate.RegisterBranchIDAlias(branchID, alias)
		}

		testFramework.IssueMessages("Message18").WaitMessagesBooked()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1":  markers.NewMarkers(markers.NewMarker(1, 1)),
			"Message2":  markers.NewMarkers(markers.NewMarker(2, 1)),
			"Message3":  markers.NewMarkers(markers.NewMarker(3, 1)),
			"Message4":  markers.NewMarkers(markers.NewMarker(4, 1)),
			"Message5":  markers.NewMarkers(markers.NewMarker(4, 2)),
			"Message6":  markers.NewMarkers(markers.NewMarker(1, 2)),
			"Message7":  markers.NewMarkers(markers.NewMarker(3, 2)),
			"Message8":  markers.NewMarkers(markers.NewMarker(1, 3)),
			"Message9":  markers.NewMarkers(markers.NewMarker(3, 3)),
			"Message10": markers.NewMarkers(markers.NewMarker(5, 2)),
			"Message11": markers.NewMarkers(markers.NewMarker(6, 4)),
			"Message12": markers.NewMarkers(markers.NewMarker(1, 3), markers.NewMarker(3, 3)),
			"Message13": markers.NewMarkers(markers.NewMarker(7, 3)),
			"Message14": markers.NewMarkers(markers.NewMarker(1, 3), markers.NewMarker(3, 3)),
			"Message15": markers.NewMarkers(markers.NewMarker(6, 4)),
			"Message16": markers.NewMarkers(markers.NewMarker(6, 5)),
			"Message17": markers.NewMarkers(markers.NewMarker(8, 2)),
			"Message18": markers.NewMarkers(markers.NewMarker(9, 4)),
		})
		checkMessageMetadataBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1":  ledgerstate.UndefinedBranchID,
			"Message2":  ledgerstate.UndefinedBranchID,
			"Message3":  ledgerstate.UndefinedBranchID,
			"Message4":  ledgerstate.UndefinedBranchID,
			"Message5":  ledgerstate.UndefinedBranchID,
			"Message6":  ledgerstate.UndefinedBranchID,
			"Message7":  ledgerstate.UndefinedBranchID,
			"Message8":  ledgerstate.UndefinedBranchID,
			"Message9":  ledgerstate.UndefinedBranchID,
			"Message10": ledgerstate.UndefinedBranchID,
			"Message11": ledgerstate.UndefinedBranchID,
			"Message12": branchIDs["F+C"],
			"Message13": ledgerstate.UndefinedBranchID,
			"Message14": branchIDs["H+C"],
			"Message15": branchIDs["H+C"],
			"Message16": ledgerstate.UndefinedBranchID,
			"Message17": ledgerstate.UndefinedBranchID,
			"Message18": ledgerstate.UndefinedBranchID,
		})
		checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1":  branchIDs["A"],
			"Message2":  branchIDs["B"],
			"Message3":  branchIDs["C"],
			"Message4":  ledgerstate.MasterBranchID,
			"Message5":  branchIDs["D"],
			"Message6":  branchIDs["F"],
			"Message7":  branchIDs["C"],
			"Message8":  branchIDs["F"],
			"Message9":  branchIDs["C"],
			"Message10": branchIDs["B+E"],
			"Message11": branchIDs["F+C"],
			"Message12": branchIDs["F+C"],
			"Message13": branchIDs["F+C+E"],
			"Message14": branchIDs["H+C"],
			"Message15": branchIDs["H+C"],
			"Message16": branchIDs["H+C"],
			"Message17": branchIDs["G"],
			"Message18": branchIDs["I+C"],
		})
		checkIndividuallyMappedMessages(t, testFramework, map[ledgerstate.BranchID][]string{
			branchIDs["F+C"]: {"Message12"},
			branchIDs["H+C"]: {"Message14", "Message15"},
		})
	}

	// ISSUE Message19
	{
		testFramework.CreateMessage("Message19", WithStrongParents("Message8"))
		testFramework.IssueMessages("Message19").WaitMessagesBooked()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1":  markers.NewMarkers(markers.NewMarker(1, 1)),
			"Message2":  markers.NewMarkers(markers.NewMarker(2, 1)),
			"Message3":  markers.NewMarkers(markers.NewMarker(3, 1)),
			"Message4":  markers.NewMarkers(markers.NewMarker(4, 1)),
			"Message5":  markers.NewMarkers(markers.NewMarker(4, 2)),
			"Message6":  markers.NewMarkers(markers.NewMarker(1, 2)),
			"Message7":  markers.NewMarkers(markers.NewMarker(3, 2)),
			"Message8":  markers.NewMarkers(markers.NewMarker(1, 3)),
			"Message9":  markers.NewMarkers(markers.NewMarker(3, 3)),
			"Message10": markers.NewMarkers(markers.NewMarker(5, 2)),
			"Message11": markers.NewMarkers(markers.NewMarker(6, 4)),
			"Message12": markers.NewMarkers(markers.NewMarker(1, 3), markers.NewMarker(3, 3)),
			"Message13": markers.NewMarkers(markers.NewMarker(7, 3)),
			"Message14": markers.NewMarkers(markers.NewMarker(1, 3), markers.NewMarker(3, 3)),
			"Message15": markers.NewMarkers(markers.NewMarker(6, 4)),
			"Message16": markers.NewMarkers(markers.NewMarker(6, 5)),
			"Message17": markers.NewMarkers(markers.NewMarker(8, 2)),
			"Message18": markers.NewMarkers(markers.NewMarker(9, 4)),
			"Message19": markers.NewMarkers(markers.NewMarker(1, 4)),
		})
		checkMessageMetadataBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1":  ledgerstate.UndefinedBranchID,
			"Message2":  ledgerstate.UndefinedBranchID,
			"Message3":  ledgerstate.UndefinedBranchID,
			"Message4":  ledgerstate.UndefinedBranchID,
			"Message5":  ledgerstate.UndefinedBranchID,
			"Message6":  ledgerstate.UndefinedBranchID,
			"Message7":  ledgerstate.UndefinedBranchID,
			"Message8":  ledgerstate.UndefinedBranchID,
			"Message9":  ledgerstate.UndefinedBranchID,
			"Message10": ledgerstate.UndefinedBranchID,
			"Message11": ledgerstate.UndefinedBranchID,
			"Message12": branchIDs["F+C"],
			"Message13": ledgerstate.UndefinedBranchID,
			"Message14": branchIDs["H+C"],
			"Message15": branchIDs["H+C"],
			"Message16": ledgerstate.UndefinedBranchID,
			"Message17": ledgerstate.UndefinedBranchID,
			"Message18": ledgerstate.UndefinedBranchID,
			"Message19": ledgerstate.UndefinedBranchID,
		})
		checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1":  branchIDs["A"],
			"Message2":  branchIDs["B"],
			"Message3":  branchIDs["C"],
			"Message4":  ledgerstate.MasterBranchID,
			"Message5":  branchIDs["D"],
			"Message6":  branchIDs["F"],
			"Message7":  branchIDs["C"],
			"Message8":  branchIDs["F"],
			"Message9":  branchIDs["C"],
			"Message10": branchIDs["B+E"],
			"Message11": branchIDs["F+C"],
			"Message12": branchIDs["F+C"],
			"Message13": branchIDs["F+C+E"],
			"Message14": branchIDs["H+C"],
			"Message15": branchIDs["H+C"],
			"Message16": branchIDs["H+C"],
			"Message17": branchIDs["G"],
			"Message18": branchIDs["I+C"],
			"Message19": branchIDs["F"],
		})
		checkIndividuallyMappedMessages(t, testFramework, map[ledgerstate.BranchID][]string{
			branchIDs["F+C"]: {"Message12"},
			branchIDs["H+C"]: {"Message14", "Message15"},
		})
	}

	// ISSUE Message20
	{
		testFramework.CreateMessage("Message20", WithStrongParents("Message16", "Message19"))
		testFramework.PreventNewMarkers(true).IssueMessages("Message20").WaitMessagesBooked().PreventNewMarkers(false)

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1":  markers.NewMarkers(markers.NewMarker(1, 1)),
			"Message2":  markers.NewMarkers(markers.NewMarker(2, 1)),
			"Message3":  markers.NewMarkers(markers.NewMarker(3, 1)),
			"Message4":  markers.NewMarkers(markers.NewMarker(4, 1)),
			"Message5":  markers.NewMarkers(markers.NewMarker(4, 2)),
			"Message6":  markers.NewMarkers(markers.NewMarker(1, 2)),
			"Message7":  markers.NewMarkers(markers.NewMarker(3, 2)),
			"Message8":  markers.NewMarkers(markers.NewMarker(1, 3)),
			"Message9":  markers.NewMarkers(markers.NewMarker(3, 3)),
			"Message10": markers.NewMarkers(markers.NewMarker(5, 2)),
			"Message11": markers.NewMarkers(markers.NewMarker(6, 4)),
			"Message12": markers.NewMarkers(markers.NewMarker(1, 3), markers.NewMarker(3, 3)),
			"Message13": markers.NewMarkers(markers.NewMarker(7, 3)),
			"Message14": markers.NewMarkers(markers.NewMarker(1, 3), markers.NewMarker(3, 3)),
			"Message15": markers.NewMarkers(markers.NewMarker(6, 4)),
			"Message16": markers.NewMarkers(markers.NewMarker(6, 5)),
			"Message17": markers.NewMarkers(markers.NewMarker(8, 2)),
			"Message18": markers.NewMarkers(markers.NewMarker(9, 4)),
			"Message19": markers.NewMarkers(markers.NewMarker(1, 4)),
			"Message20": markers.NewMarkers(markers.NewMarker(1, 4), markers.NewMarker(6, 5)),
		})
		checkMessageMetadataBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1":  ledgerstate.UndefinedBranchID,
			"Message2":  ledgerstate.UndefinedBranchID,
			"Message3":  ledgerstate.UndefinedBranchID,
			"Message4":  ledgerstate.UndefinedBranchID,
			"Message5":  ledgerstate.UndefinedBranchID,
			"Message6":  ledgerstate.UndefinedBranchID,
			"Message7":  ledgerstate.UndefinedBranchID,
			"Message8":  ledgerstate.UndefinedBranchID,
			"Message9":  ledgerstate.UndefinedBranchID,
			"Message10": ledgerstate.UndefinedBranchID,
			"Message11": ledgerstate.UndefinedBranchID,
			"Message12": branchIDs["F+C"],
			"Message13": ledgerstate.UndefinedBranchID,
			"Message14": branchIDs["H+C"],
			"Message15": branchIDs["H+C"],
			"Message16": ledgerstate.UndefinedBranchID,
			"Message17": ledgerstate.UndefinedBranchID,
			"Message18": ledgerstate.UndefinedBranchID,
			"Message19": ledgerstate.UndefinedBranchID,
			"Message20": branchIDs["H+C"],
		})
		checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1":  branchIDs["A"],
			"Message2":  branchIDs["B"],
			"Message3":  branchIDs["C"],
			"Message4":  ledgerstate.MasterBranchID,
			"Message5":  branchIDs["D"],
			"Message6":  branchIDs["F"],
			"Message7":  branchIDs["C"],
			"Message8":  branchIDs["F"],
			"Message9":  branchIDs["C"],
			"Message10": branchIDs["B+E"],
			"Message11": branchIDs["F+C"],
			"Message12": branchIDs["F+C"],
			"Message13": branchIDs["F+C+E"],
			"Message14": branchIDs["H+C"],
			"Message15": branchIDs["H+C"],
			"Message16": branchIDs["H+C"],
			"Message17": branchIDs["G"],
			"Message18": branchIDs["I+C"],
			"Message19": branchIDs["F"],
			"Message20": branchIDs["H+C"],
		})
		checkIndividuallyMappedMessages(t, testFramework, map[ledgerstate.BranchID][]string{
			branchIDs["F+C"]: {"Message12"},
			branchIDs["H+C"]: {"Message14", "Message15", "Message20"},
		})
	}

	// ISSUE Message21
	{
		testFramework.CreateMessage("Message21", WithStrongParents("Message4"))
		testFramework.IssueMessages("Message21").WaitMessagesBooked()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1":  markers.NewMarkers(markers.NewMarker(1, 1)),
			"Message2":  markers.NewMarkers(markers.NewMarker(2, 1)),
			"Message3":  markers.NewMarkers(markers.NewMarker(3, 1)),
			"Message4":  markers.NewMarkers(markers.NewMarker(4, 1)),
			"Message5":  markers.NewMarkers(markers.NewMarker(4, 2)),
			"Message6":  markers.NewMarkers(markers.NewMarker(1, 2)),
			"Message7":  markers.NewMarkers(markers.NewMarker(3, 2)),
			"Message8":  markers.NewMarkers(markers.NewMarker(1, 3)),
			"Message9":  markers.NewMarkers(markers.NewMarker(3, 3)),
			"Message10": markers.NewMarkers(markers.NewMarker(5, 2)),
			"Message11": markers.NewMarkers(markers.NewMarker(6, 4)),
			"Message12": markers.NewMarkers(markers.NewMarker(1, 3), markers.NewMarker(3, 3)),
			"Message13": markers.NewMarkers(markers.NewMarker(7, 3)),
			"Message14": markers.NewMarkers(markers.NewMarker(1, 3), markers.NewMarker(3, 3)),
			"Message15": markers.NewMarkers(markers.NewMarker(6, 4)),
			"Message16": markers.NewMarkers(markers.NewMarker(6, 5)),
			"Message17": markers.NewMarkers(markers.NewMarker(8, 2)),
			"Message18": markers.NewMarkers(markers.NewMarker(9, 4)),
			"Message19": markers.NewMarkers(markers.NewMarker(1, 4)),
			"Message20": markers.NewMarkers(markers.NewMarker(1, 4), markers.NewMarker(6, 5)),
			"Message21": markers.NewMarkers(markers.NewMarker(10, 2)),
		})
		checkMessageMetadataBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1":  ledgerstate.UndefinedBranchID,
			"Message2":  ledgerstate.UndefinedBranchID,
			"Message3":  ledgerstate.UndefinedBranchID,
			"Message4":  ledgerstate.UndefinedBranchID,
			"Message5":  ledgerstate.UndefinedBranchID,
			"Message6":  ledgerstate.UndefinedBranchID,
			"Message7":  ledgerstate.UndefinedBranchID,
			"Message8":  ledgerstate.UndefinedBranchID,
			"Message9":  ledgerstate.UndefinedBranchID,
			"Message10": ledgerstate.UndefinedBranchID,
			"Message11": ledgerstate.UndefinedBranchID,
			"Message12": branchIDs["F+C"],
			"Message13": ledgerstate.UndefinedBranchID,
			"Message14": branchIDs["H+C"],
			"Message15": branchIDs["H+C"],
			"Message16": ledgerstate.UndefinedBranchID,
			"Message17": ledgerstate.UndefinedBranchID,
			"Message18": ledgerstate.UndefinedBranchID,
			"Message19": ledgerstate.UndefinedBranchID,
			"Message20": branchIDs["H+C"],
			"Message21": ledgerstate.UndefinedBranchID,
		})
		checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1":  branchIDs["A"],
			"Message2":  branchIDs["B"],
			"Message3":  branchIDs["C"],
			"Message4":  ledgerstate.MasterBranchID,
			"Message5":  branchIDs["D"],
			"Message6":  branchIDs["F"],
			"Message7":  branchIDs["C"],
			"Message8":  branchIDs["F"],
			"Message9":  branchIDs["C"],
			"Message10": branchIDs["B+E"],
			"Message11": branchIDs["F+C"],
			"Message12": branchIDs["F+C"],
			"Message13": branchIDs["F+C+E"],
			"Message14": branchIDs["H+C"],
			"Message15": branchIDs["H+C"],
			"Message16": branchIDs["H+C"],
			"Message17": branchIDs["G"],
			"Message18": branchIDs["I+C"],
			"Message19": branchIDs["F"],
			"Message20": branchIDs["H+C"],
			"Message21": ledgerstate.MasterBranchID,
		})
		checkIndividuallyMappedMessages(t, testFramework, map[ledgerstate.BranchID][]string{
			branchIDs["F+C"]: {"Message12"},
			branchIDs["H+C"]: {"Message14", "Message15", "Message20"},
		})
	}

	// ISSUE Message22
	{
		testFramework.CreateMessage("Message22", WithStrongParents("Message4"))
		testFramework.IssueMessages("Message22").WaitMessagesBooked()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1":  markers.NewMarkers(markers.NewMarker(1, 1)),
			"Message2":  markers.NewMarkers(markers.NewMarker(2, 1)),
			"Message3":  markers.NewMarkers(markers.NewMarker(3, 1)),
			"Message4":  markers.NewMarkers(markers.NewMarker(4, 1)),
			"Message5":  markers.NewMarkers(markers.NewMarker(4, 2)),
			"Message6":  markers.NewMarkers(markers.NewMarker(1, 2)),
			"Message7":  markers.NewMarkers(markers.NewMarker(3, 2)),
			"Message8":  markers.NewMarkers(markers.NewMarker(1, 3)),
			"Message9":  markers.NewMarkers(markers.NewMarker(3, 3)),
			"Message10": markers.NewMarkers(markers.NewMarker(5, 2)),
			"Message11": markers.NewMarkers(markers.NewMarker(6, 4)),
			"Message12": markers.NewMarkers(markers.NewMarker(1, 3), markers.NewMarker(3, 3)),
			"Message13": markers.NewMarkers(markers.NewMarker(7, 3)),
			"Message14": markers.NewMarkers(markers.NewMarker(1, 3), markers.NewMarker(3, 3)),
			"Message15": markers.NewMarkers(markers.NewMarker(6, 4)),
			"Message16": markers.NewMarkers(markers.NewMarker(6, 5)),
			"Message17": markers.NewMarkers(markers.NewMarker(8, 2)),
			"Message18": markers.NewMarkers(markers.NewMarker(9, 4)),
			"Message19": markers.NewMarkers(markers.NewMarker(1, 4)),
			"Message20": markers.NewMarkers(markers.NewMarker(1, 4), markers.NewMarker(6, 5)),
			"Message21": markers.NewMarkers(markers.NewMarker(10, 2)),
			"Message22": markers.NewMarkers(markers.NewMarker(4, 1)),
		})
		checkMessageMetadataBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1":  ledgerstate.UndefinedBranchID,
			"Message2":  ledgerstate.UndefinedBranchID,
			"Message3":  ledgerstate.UndefinedBranchID,
			"Message4":  ledgerstate.UndefinedBranchID,
			"Message5":  ledgerstate.UndefinedBranchID,
			"Message6":  ledgerstate.UndefinedBranchID,
			"Message7":  ledgerstate.UndefinedBranchID,
			"Message8":  ledgerstate.UndefinedBranchID,
			"Message9":  ledgerstate.UndefinedBranchID,
			"Message10": ledgerstate.UndefinedBranchID,
			"Message11": ledgerstate.UndefinedBranchID,
			"Message12": branchIDs["F+C"],
			"Message13": ledgerstate.UndefinedBranchID,
			"Message14": branchIDs["H+C"],
			"Message15": branchIDs["H+C"],
			"Message16": ledgerstate.UndefinedBranchID,
			"Message17": ledgerstate.UndefinedBranchID,
			"Message18": ledgerstate.UndefinedBranchID,
			"Message19": ledgerstate.UndefinedBranchID,
			"Message20": branchIDs["H+C"],
			"Message21": ledgerstate.UndefinedBranchID,
			"Message22": ledgerstate.UndefinedBranchID,
		})
		checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1":  branchIDs["A"],
			"Message2":  branchIDs["B"],
			"Message3":  branchIDs["C"],
			"Message4":  ledgerstate.MasterBranchID,
			"Message5":  branchIDs["D"],
			"Message6":  branchIDs["F"],
			"Message7":  branchIDs["C"],
			"Message8":  branchIDs["F"],
			"Message9":  branchIDs["C"],
			"Message10": branchIDs["B+E"],
			"Message11": branchIDs["F+C"],
			"Message12": branchIDs["F+C"],
			"Message13": branchIDs["F+C+E"],
			"Message14": branchIDs["H+C"],
			"Message15": branchIDs["H+C"],
			"Message16": branchIDs["H+C"],
			"Message17": branchIDs["G"],
			"Message18": branchIDs["I+C"],
			"Message19": branchIDs["F"],
			"Message20": branchIDs["H+C"],
			"Message21": ledgerstate.MasterBranchID,
			"Message22": ledgerstate.MasterBranchID,
		})
		checkIndividuallyMappedMessages(t, testFramework, map[ledgerstate.BranchID][]string{
			branchIDs["F+C"]: {"Message12"},
			branchIDs["H+C"]: {"Message14", "Message15", "Message20"},
		})
	}

	// ISSUE Message23
	{
		testFramework.CreateMessage("Message23", WithStrongParents("Message21", "Message22"))
		testFramework.IssueMessages("Message23").WaitMessagesBooked()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1":  markers.NewMarkers(markers.NewMarker(1, 1)),
			"Message2":  markers.NewMarkers(markers.NewMarker(2, 1)),
			"Message3":  markers.NewMarkers(markers.NewMarker(3, 1)),
			"Message4":  markers.NewMarkers(markers.NewMarker(4, 1)),
			"Message5":  markers.NewMarkers(markers.NewMarker(4, 2)),
			"Message6":  markers.NewMarkers(markers.NewMarker(1, 2)),
			"Message7":  markers.NewMarkers(markers.NewMarker(3, 2)),
			"Message8":  markers.NewMarkers(markers.NewMarker(1, 3)),
			"Message9":  markers.NewMarkers(markers.NewMarker(3, 3)),
			"Message10": markers.NewMarkers(markers.NewMarker(5, 2)),
			"Message11": markers.NewMarkers(markers.NewMarker(6, 4)),
			"Message12": markers.NewMarkers(markers.NewMarker(1, 3), markers.NewMarker(3, 3)),
			"Message13": markers.NewMarkers(markers.NewMarker(7, 3)),
			"Message14": markers.NewMarkers(markers.NewMarker(1, 3), markers.NewMarker(3, 3)),
			"Message15": markers.NewMarkers(markers.NewMarker(6, 4)),
			"Message16": markers.NewMarkers(markers.NewMarker(6, 5)),
			"Message17": markers.NewMarkers(markers.NewMarker(8, 2)),
			"Message18": markers.NewMarkers(markers.NewMarker(9, 4)),
			"Message19": markers.NewMarkers(markers.NewMarker(1, 4)),
			"Message20": markers.NewMarkers(markers.NewMarker(1, 4), markers.NewMarker(6, 5)),
			"Message21": markers.NewMarkers(markers.NewMarker(10, 2)),
			"Message22": markers.NewMarkers(markers.NewMarker(4, 1)),
			"Message23": markers.NewMarkers(markers.NewMarker(10, 3)),
		})
		checkMessageMetadataBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1":  ledgerstate.UndefinedBranchID,
			"Message2":  ledgerstate.UndefinedBranchID,
			"Message3":  ledgerstate.UndefinedBranchID,
			"Message4":  ledgerstate.UndefinedBranchID,
			"Message5":  ledgerstate.UndefinedBranchID,
			"Message6":  ledgerstate.UndefinedBranchID,
			"Message7":  ledgerstate.UndefinedBranchID,
			"Message8":  ledgerstate.UndefinedBranchID,
			"Message9":  ledgerstate.UndefinedBranchID,
			"Message10": ledgerstate.UndefinedBranchID,
			"Message11": ledgerstate.UndefinedBranchID,
			"Message12": branchIDs["F+C"],
			"Message13": ledgerstate.UndefinedBranchID,
			"Message14": branchIDs["H+C"],
			"Message15": branchIDs["H+C"],
			"Message16": ledgerstate.UndefinedBranchID,
			"Message17": ledgerstate.UndefinedBranchID,
			"Message18": ledgerstate.UndefinedBranchID,
			"Message19": ledgerstate.UndefinedBranchID,
			"Message20": branchIDs["H+C"],
			"Message21": ledgerstate.UndefinedBranchID,
			"Message22": ledgerstate.UndefinedBranchID,
			"Message23": ledgerstate.UndefinedBranchID,
		})
		checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1":  branchIDs["A"],
			"Message2":  branchIDs["B"],
			"Message3":  branchIDs["C"],
			"Message4":  ledgerstate.MasterBranchID,
			"Message5":  branchIDs["D"],
			"Message6":  branchIDs["F"],
			"Message7":  branchIDs["C"],
			"Message8":  branchIDs["F"],
			"Message9":  branchIDs["C"],
			"Message10": branchIDs["B+E"],
			"Message11": branchIDs["F+C"],
			"Message12": branchIDs["F+C"],
			"Message13": branchIDs["F+C+E"],
			"Message14": branchIDs["H+C"],
			"Message15": branchIDs["H+C"],
			"Message16": branchIDs["H+C"],
			"Message17": branchIDs["G"],
			"Message18": branchIDs["I+C"],
			"Message19": branchIDs["F"],
			"Message20": branchIDs["H+C"],
			"Message21": ledgerstate.MasterBranchID,
			"Message22": ledgerstate.MasterBranchID,
			"Message23": ledgerstate.MasterBranchID,
		})
		checkIndividuallyMappedMessages(t, testFramework, map[ledgerstate.BranchID][]string{
			branchIDs["F+C"]: {"Message12"},
			branchIDs["H+C"]: {"Message14", "Message15", "Message20"},
		})
	}

	// ISSUE Message24
	{
		testFramework.CreateMessage("Message24", WithStrongParents("Message23", "Message5"))
		testFramework.IssueMessages("Message24").WaitMessagesBooked()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1":  markers.NewMarkers(markers.NewMarker(1, 1)),
			"Message2":  markers.NewMarkers(markers.NewMarker(2, 1)),
			"Message3":  markers.NewMarkers(markers.NewMarker(3, 1)),
			"Message4":  markers.NewMarkers(markers.NewMarker(4, 1)),
			"Message5":  markers.NewMarkers(markers.NewMarker(4, 2)),
			"Message6":  markers.NewMarkers(markers.NewMarker(1, 2)),
			"Message7":  markers.NewMarkers(markers.NewMarker(3, 2)),
			"Message8":  markers.NewMarkers(markers.NewMarker(1, 3)),
			"Message9":  markers.NewMarkers(markers.NewMarker(3, 3)),
			"Message10": markers.NewMarkers(markers.NewMarker(5, 2)),
			"Message11": markers.NewMarkers(markers.NewMarker(6, 4)),
			"Message12": markers.NewMarkers(markers.NewMarker(1, 3), markers.NewMarker(3, 3)),
			"Message13": markers.NewMarkers(markers.NewMarker(7, 3)),
			"Message14": markers.NewMarkers(markers.NewMarker(1, 3), markers.NewMarker(3, 3)),
			"Message15": markers.NewMarkers(markers.NewMarker(6, 4)),
			"Message16": markers.NewMarkers(markers.NewMarker(6, 5)),
			"Message17": markers.NewMarkers(markers.NewMarker(8, 2)),
			"Message18": markers.NewMarkers(markers.NewMarker(9, 4)),
			"Message19": markers.NewMarkers(markers.NewMarker(1, 4)),
			"Message20": markers.NewMarkers(markers.NewMarker(1, 4), markers.NewMarker(6, 5)),
			"Message21": markers.NewMarkers(markers.NewMarker(10, 2)),
			"Message22": markers.NewMarkers(markers.NewMarker(4, 1)),
			"Message23": markers.NewMarkers(markers.NewMarker(10, 3)),
			"Message24": markers.NewMarkers(markers.NewMarker(4, 4)),
		})
		checkMessageMetadataBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1":  ledgerstate.UndefinedBranchID,
			"Message2":  ledgerstate.UndefinedBranchID,
			"Message3":  ledgerstate.UndefinedBranchID,
			"Message4":  ledgerstate.UndefinedBranchID,
			"Message5":  ledgerstate.UndefinedBranchID,
			"Message6":  ledgerstate.UndefinedBranchID,
			"Message7":  ledgerstate.UndefinedBranchID,
			"Message8":  ledgerstate.UndefinedBranchID,
			"Message9":  ledgerstate.UndefinedBranchID,
			"Message10": ledgerstate.UndefinedBranchID,
			"Message11": ledgerstate.UndefinedBranchID,
			"Message12": branchIDs["F+C"],
			"Message13": ledgerstate.UndefinedBranchID,
			"Message14": branchIDs["H+C"],
			"Message15": branchIDs["H+C"],
			"Message16": ledgerstate.UndefinedBranchID,
			"Message17": ledgerstate.UndefinedBranchID,
			"Message18": ledgerstate.UndefinedBranchID,
			"Message19": ledgerstate.UndefinedBranchID,
			"Message20": branchIDs["H+C"],
			"Message21": ledgerstate.UndefinedBranchID,
			"Message22": ledgerstate.UndefinedBranchID,
			"Message23": ledgerstate.UndefinedBranchID,
			"Message24": ledgerstate.UndefinedBranchID,
		})
		checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1":  branchIDs["A"],
			"Message2":  branchIDs["B"],
			"Message3":  branchIDs["C"],
			"Message4":  ledgerstate.MasterBranchID,
			"Message5":  branchIDs["D"],
			"Message6":  branchIDs["F"],
			"Message7":  branchIDs["C"],
			"Message8":  branchIDs["F"],
			"Message9":  branchIDs["C"],
			"Message10": branchIDs["B+E"],
			"Message11": branchIDs["F+C"],
			"Message12": branchIDs["F+C"],
			"Message13": branchIDs["F+C+E"],
			"Message14": branchIDs["H+C"],
			"Message15": branchIDs["H+C"],
			"Message16": branchIDs["H+C"],
			"Message17": branchIDs["G"],
			"Message18": branchIDs["I+C"],
			"Message19": branchIDs["F"],
			"Message20": branchIDs["H+C"],
			"Message21": ledgerstate.MasterBranchID,
			"Message22": ledgerstate.MasterBranchID,
			"Message23": ledgerstate.MasterBranchID,
			"Message24": branchIDs["D"],
		})
		checkIndividuallyMappedMessages(t, testFramework, map[ledgerstate.BranchID][]string{
			branchIDs["F+C"]: {"Message12"},
			branchIDs["H+C"]: {"Message14", "Message15", "Message20"},
		})
	}
}

func aggregatedBranchID(branchIDs ...ledgerstate.BranchID) ledgerstate.BranchID {
	return ledgerstate.NewAggregatedBranch(ledgerstate.NewBranchIDs(branchIDs...)).ID()
}

func checkIndividuallyMappedMessages(t *testing.T, testFramework *MessageTestFramework, expectedIndividuallyMappedMessages map[ledgerstate.BranchID][]string) {
	expectedMappings := 0

	for branchID, expectedMessageAliases := range expectedIndividuallyMappedMessages {
		for _, alias := range expectedMessageAliases {
			expectedMappings++
			assert.True(t, testFramework.tangle.Storage.IndividuallyMappedMessage(branchID, testFramework.Message(alias).ID()).Consume(func(individuallyMappedMessage *IndividuallyMappedMessage) {}))
		}
	}

	testFramework.tangle.Storage.individuallyMappedMessageStorage.ForEach(func(key []byte, cachedObject objectstorage.CachedObject) bool {
		defer cachedObject.Release()

		expectedMappings--

		return true
	})

	assert.Zero(t, expectedMappings)
}

func checkMarkers(t *testing.T, testFramework *MessageTestFramework, expectedMarkers map[string]*markers.Markers) {
	for messageID, expectedMarkersOfMessage := range expectedMarkers {
		assert.True(t, testFramework.tangle.Storage.MessageMetadata(testFramework.Message(messageID).ID()).Consume(func(messageMetadata *MessageMetadata) {
			assert.True(t, expectedMarkersOfMessage.Equals(messageMetadata.StructureDetails().PastMarkers), "Markers of %s are wrong.\n"+
				"Expected: %+v\nActual: %+v", messageID, expectedMarkersOfMessage, messageMetadata.StructureDetails().PastMarkers)
		}))

		// if we have only a single marker - check if the marker is mapped to this message (or its inherited past marker)
		if expectedMarkersOfMessage.Size() == 1 {
			currentMarker := expectedMarkersOfMessage.Marker()

			mappedMessageIDOfMarker := testFramework.tangle.Booker.MarkersManager.MessageID(currentMarker)
			currentMessageID := testFramework.Message(messageID).ID()

			if mappedMessageIDOfMarker == currentMessageID {
				continue
			}

			assert.True(t, testFramework.tangle.Storage.MessageMetadata(mappedMessageIDOfMarker).Consume(func(messageMetadata *MessageMetadata) {
				assert.True(t, messageMetadata.StructureDetails().IsPastMarker && *messageMetadata.StructureDetails().PastMarkers.Marker() == *currentMarker, "%s was mapped to wrong %s", currentMarker, messageMetadata.ID())
			}), "failed to load Message with %s", mappedMessageIDOfMarker)
		}
	}
}

func checkBranchIDs(t *testing.T, testFramework *MessageTestFramework, expectedBranchIDs map[string]ledgerstate.BranchID) {
	for messageID, expectedBranchID := range expectedBranchIDs {
		retrievedBranchID, err := testFramework.tangle.Booker.MessageBranchID(testFramework.Message(messageID).ID())
		assert.NoError(t, err)

		assert.Equal(t, expectedBranchID, retrievedBranchID, "BranchID of %s should be %s but is %s", messageID, expectedBranchID, retrievedBranchID)
	}
}

func checkMessageMetadataBranchIDs(t *testing.T, testFramework *MessageTestFramework, expectedBranchIDs map[string]ledgerstate.BranchID) {
	for messageID, expectedBranchID := range expectedBranchIDs {
		assert.True(t, testFramework.tangle.Storage.MessageMetadata(testFramework.Message(messageID).ID()).Consume(func(messageMetadata *MessageMetadata) {
			assert.Equal(t, expectedBranchID, messageMetadata.BranchID(), "BranchID of %s should be %s but is %s in the Metadata", messageID, expectedBranchID, messageMetadata.BranchID())
		}))
	}
}
