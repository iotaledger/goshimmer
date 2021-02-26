package tangle

import (
	"testing"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/markers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScenario_2(t *testing.T) {
	tangle := New(WithoutOpinionFormer(true))
	defer tangle.Shutdown()
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
	snapshot := map[ledgerstate.TransactionID]map[ledgerstate.Address]*ledgerstate.ColoredBalances{
		ledgerstate.GenesisTransactionID: {
			wallets["GENESIS"].address: genesisBalance},
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
	inputs["GENESIS"] = ledgerstate.NewUTXOInput(ledgerstate.NewOutputID(ledgerstate.GenesisTransactionID, 0))
	outputs["A"] = ledgerstate.NewSigLockedSingleOutput(1, wallets["A"].address)
	outputs["B"] = ledgerstate.NewSigLockedSingleOutput(1, wallets["B"].address)

	outputs["C"] = ledgerstate.NewSigLockedSingleOutput(1, wallets["C"].address)
	transactions["1"] = makeTransaction(ledgerstate.NewInputs(inputs["GENESIS"]), ledgerstate.NewOutputs(outputs["A"], outputs["B"], outputs["C"]), outputsByID, walletsByAddress, wallets["GENESIS"])
	messages["1"] = newTestParentsPayloadMessage(transactions["1"], []MessageID{EmptyMessageID}, []MessageID{})
	tangle.Storage.StoreMessage(messages["1"])

	err := tangle.Booker.Book(messages["1"].ID())

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

	err = tangle.Booker.Book(messages["2"].ID())
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

	err = tangle.Booker.Book(messages["3"].ID())
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

	err = tangle.Booker.Book(messages["4"].ID())
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

	err = tangle.Booker.Book(messages["5"].ID())
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

	err = tangle.Booker.Book(messages["6"].ID())
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

	err = tangle.Booker.Book(messages["7"].ID())
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

	err = tangle.Booker.Book(messages["8"].ID())
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

	err = tangle.Booker.Book(messages["9"].ID())
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
			IsPastMarker:  true,
			PastMarkers:   markers.NewMarkers(markers.NewMarker(1, 1)),
			FutureMarkers: markers.NewMarkers(markers.NewMarker(1, 2), markers.NewMarker(3, 2)),
		}
		structureDetails[messages["2"].ID()] = &markers.StructureDetails{
			Rank:          2,
			IsPastMarker:  true,
			PastMarkers:   markers.NewMarkers(markers.NewMarker(1, 2)),
			FutureMarkers: markers.NewMarkers(markers.NewMarker(1, 3), markers.NewMarker(2, 3)),
		}
		structureDetails[messages["3"].ID()] = &markers.StructureDetails{
			Rank:          3,
			IsPastMarker:  true,
			PastMarkers:   markers.NewMarkers(markers.NewMarker(1, 3)),
			FutureMarkers: markers.NewMarkers(),
		}
		structureDetails[messages["4"].ID()] = &markers.StructureDetails{
			Rank:          2,
			IsPastMarker:  false,
			PastMarkers:   markers.NewMarkers(markers.NewMarker(1, 1)),
			FutureMarkers: markers.NewMarkers(markers.NewMarker(3, 2)),
		}
		structureDetails[messages["5"].ID()] = &markers.StructureDetails{
			Rank:          3,
			IsPastMarker:  true,
			PastMarkers:   markers.NewMarkers(markers.NewMarker(2, 3)),
			FutureMarkers: markers.NewMarkers(markers.NewMarker(2, 4)),
		}
		structureDetails[messages["6"].ID()] = &markers.StructureDetails{
			Rank:          4,
			IsPastMarker:  true,
			PastMarkers:   markers.NewMarkers(markers.NewMarker(2, 4)),
			FutureMarkers: markers.NewMarkers(),
		}
		structureDetails[messages["7"].ID()] = &markers.StructureDetails{
			Rank:          3,
			IsPastMarker:  true,
			PastMarkers:   markers.NewMarkers(markers.NewMarker(3, 2)),
			FutureMarkers: markers.NewMarkers(markers.NewMarker(3, 3), markers.NewMarker(4, 3)),
		}
		structureDetails[messages["8"].ID()] = &markers.StructureDetails{
			Rank:          4,
			IsPastMarker:  true,
			PastMarkers:   markers.NewMarkers(markers.NewMarker(3, 3)),
			FutureMarkers: markers.NewMarkers(),
		}
		structureDetails[messages["9"].ID()] = &markers.StructureDetails{
			Rank:          4,
			IsPastMarker:  true,
			PastMarkers:   markers.NewMarkers(markers.NewMarker(4, 3)),
			FutureMarkers: markers.NewMarkers(),
		}

		for _, message := range messages {
			tangle.Storage.MessageMetadata(message.ID()).Consume(func(metadata *MessageMetadata) {
				assert.Equal(t, structureDetails[message.ID()], metadata.StructureDetails())
			})
		}
	}
	/////////////////////////////////
}
