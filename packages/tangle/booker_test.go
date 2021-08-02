//nolint:dupl
package tangle

import (
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
	tangle := NewTestTangle()
	defer tangle.Shutdown()

	testFramework := NewMessageTestFramework(
		tangle,
		WithGenesisOutput("G", 3),
	)

	tangle.Setup()

	testFramework.CreateMessage("Message1", WithStrongParents("Genesis"), WithInputs("G"), WithOutput("A", 1), WithOutput("B", 1), WithOutput("C", 1))
	testFramework.CreateMessage("Message2", WithStrongParents("Genesis", "Message1"), WithInputs("B", "C"), WithOutput("E", 2))
	testFramework.CreateMessage("Message3", WithStrongParents("Message1", "Message2"), WithReattachment("Message2"))
	testFramework.CreateMessage("Message4", WithStrongParents("Genesis", "Message1"), WithInputs("A"), WithOutput("D", 1))
	testFramework.CreateMessage("Message5", WithStrongParents("Message1", "Message2"), WithInputs("A"), WithOutput("F", 1))
	testFramework.CreateMessage("Message6", WithStrongParents("Message2", "Message5"), WithInputs("E", "F"), WithOutput("H", 3))
	testFramework.CreateMessage("Message7", WithStrongParents("Message4", "Message5"), WithReattachment("Message2"))
	testFramework.CreateMessage("Message8", WithStrongParents("Message4", "Message5"), WithInputs("F", "D"), WithOutput("I", 2))
	testFramework.CreateMessage("Message9", WithStrongParents("Message4", "Message6"), WithInputs("H"), WithOutput("J", 1))

	testFramework.RegisterBranchID("red", "Message4")
	testFramework.RegisterBranchID("yellow", "Message5")

	testFramework.IssueMessages("Message1", "Message2", "Message3", "Message4", "Message5", "Message6").WaitMessagesBooked()
	testFramework.IssueMessages("Message7", "Message9").WaitMessagesBooked()
	testFramework.IssueMessages("Message8").WaitMessagesBooked()

	for _, messageAlias := range []string{"Message7", "Message8", "Message9"} {
		assert.Truef(t, testFramework.MessageMetadata(messageAlias).invalid, "%s not invalid", messageAlias)
	}

	checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
		"Message1": ledgerstate.MasterBranchID,
		"Message3": ledgerstate.MasterBranchID,
		"Message2": ledgerstate.MasterBranchID,
		"Message4": testFramework.BranchID("red"),
		"Message5": testFramework.BranchID("yellow"),
	})
}

func TestScenario_2(t *testing.T) {
	tangle := NewTestTangle()
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
	messages["1"] = newTestParentsPayloadMessage(transactions["1"], []MessageID{EmptyMessageID}, []MessageID{}, nil, nil)
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
	messages["2"] = newTestParentsPayloadMessage(transactions["2"], []MessageID{EmptyMessageID, messages["1"].ID()}, []MessageID{}, nil, nil)
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
	messages["3"] = newTestParentsPayloadMessage(transactions["2"], []MessageID{messages["1"].ID(), messages["2"].ID()}, []MessageID{}, nil, nil)
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
	messages["4"] = newTestParentsPayloadMessage(transactions["3"], []MessageID{EmptyMessageID, messages["1"].ID()}, []MessageID{}, nil, nil)
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
	messages["5"] = newTestParentsPayloadMessage(transactions["4"], []MessageID{messages["1"].ID(), messages["2"].ID()}, []MessageID{}, nil, nil)
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
	messages["6"] = newTestParentsPayloadMessage(transactions["5"], []MessageID{messages["2"].ID(), messages["5"].ID()}, []MessageID{}, nil, nil)
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
	messages["7"] = newTestParentsPayloadMessage(transactions["6"], []MessageID{messages["1"].ID(), messages["4"].ID()}, []MessageID{}, nil, nil)
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
	messages["8"] = newTestParentsPayloadMessage(transactions["7"], []MessageID{messages["4"].ID(), messages["7"].ID()}, []MessageID{}, nil, nil)
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
	messages["9"] = newTestParentsPayloadMessage(transactions["8"], []MessageID{messages["4"].ID(), messages["7"].ID()}, []MessageID{}, nil, nil)
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
	tangle := NewTestTangle()
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
	messages["1"] = newTestParentsPayloadMessage(transactions["1"], []MessageID{EmptyMessageID}, []MessageID{}, nil, nil)
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
	messages["2"] = newTestParentsPayloadMessage(transactions["2"], []MessageID{EmptyMessageID, messages["1"].ID()}, []MessageID{}, nil, nil)
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
	messages["3"] = newTestParentsPayloadMessage(transactions["2"], []MessageID{messages["1"].ID(), messages["2"].ID()}, []MessageID{}, nil, nil)
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
	messages["4"] = newTestParentsPayloadMessage(transactions["3"], []MessageID{EmptyMessageID, messages["1"].ID()}, []MessageID{}, nil, nil)
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
	messages["5"] = newTestParentsPayloadMessage(transactions["4"], []MessageID{messages["1"].ID()}, []MessageID{messages["2"].ID()}, nil, nil)
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
	messages["6"] = newTestParentsPayloadMessage(transactions["5"], []MessageID{messages["2"].ID(), messages["5"].ID()}, []MessageID{}, nil, nil)
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
	messages["7"] = newTestParentsPayloadMessage(transactions["6"], []MessageID{messages["1"].ID(), messages["4"].ID()}, []MessageID{}, nil, nil)
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
	messages["8"] = newTestParentsPayloadMessage(transactions["7"], []MessageID{messages["4"].ID(), messages["7"].ID()}, []MessageID{}, nil, nil)
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
	messages["9"] = newTestParentsPayloadMessage(transactions["8"], []MessageID{messages["4"].ID(), messages["7"].ID()}, []MessageID{}, nil, nil)
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

// func TestBookerNewAutomaticSequence(t *testing.T) {
// 	tangle := NewTestTangle()
// 	defer tangle.Shutdown()

// 	testFramework := NewMessageTestFramework(tangle)

// 	tangle.Setup()

// 	testFramework.CreateMessage("Message1", WithStrongParents("Genesis"))
// 	testFramework.CreateMessage("Message2", WithStrongParents("Message1"))
// 	testFramework.IssueMessages("Message1", "Message2").WaitMessagesBooked()

// 	messageAliases := []string{"Message1"}
// 	for i := 0; i < 3020; i++ {
// 		parentMessageAlias := messageAliases[i]
// 		currentMessageAlias := "Message" + strconv.Itoa(i+3)
// 		messageAliases = append(messageAliases, currentMessageAlias)
// 		testFramework.CreateMessage(currentMessageAlias, WithStrongParents(parentMessageAlias))
// 		testFramework.IssueMessages(currentMessageAlias)
// 		testFramework.WaitMessagesBooked()
// 	}

// 	assert.True(t, tangle.Storage.MessageMetadata(testFramework.Message("Message3009").ID()).Consume(func(messageMetadata *MessageMetadata) {
// 		assert.Equal(t, &markers.StructureDetails{
// 			Rank:          3008,
// 			PastMarkerGap: 0,
// 			IsPastMarker:  true,
// 			SequenceID:    2,
// 			PastMarkers:   markers.NewMarkers(markers.NewMarker(2, 9)),
// 			FutureMarkers: markers.NewMarkers(markers.NewMarker(2, 10)),
// 		}, messageMetadata.StructureDetails())
// 	}))
// }
// branchIDAliases contains a list of aliases registered for a set of MessageIDs.

// Please refer to packages/tangle/images/TestBookerMarkerMappings.html for a diagram of this test
func TestBookerMarkerMappings(t *testing.T) {
	tangle := NewTestTangle()
	defer tangle.Shutdown()

	testFramework := NewMessageTestFramework(
		tangle,
		WithGenesisOutput("A", 500),
		WithGenesisOutput("B", 500),
		WithGenesisOutput("C", 500),
		WithGenesisOutput("L", 500),
	)

	tangle.Setup()

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

		testFramework.RegisterBranchID("A", "Message1")
		testFramework.RegisterBranchID("B", "Message2")

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1": markers.NewMarkers(markers.NewMarker(1, 1)),
			"Message2": markers.NewMarkers(markers.NewMarker(2, 1)),
		})
		checkMessageMetadataBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1": ledgerstate.UndefinedBranchID,
			"Message2": ledgerstate.UndefinedBranchID,
		})
		checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1": testFramework.BranchID("A"),
			"Message2": testFramework.BranchID("B"),
		})
	}

	// ISSUE Message3
	{
		testFramework.CreateMessage("Message3", WithStrongParents("Genesis"), WithInputs("B"), WithOutput("F", 500))
		testFramework.IssueMessages("Message3").WaitMessagesBooked()

		testFramework.RegisterBranchID("C", "Message3")

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
			"Message1": testFramework.BranchID("A"),
			"Message2": testFramework.BranchID("B"),
			"Message3": testFramework.BranchID("C"),
		})
	}

	// ISSUE Message4
	{
		testFramework.CreateMessage("Message4", WithStrongParents("Genesis"), WithInputs("L"), WithOutput("K", 500))
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
			"Message1": testFramework.BranchID("A"),
			"Message2": testFramework.BranchID("B"),
			"Message3": testFramework.BranchID("C"),
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
			"Message1": testFramework.BranchID("A"),
			"Message2": testFramework.BranchID("B"),
			"Message3": testFramework.BranchID("C"),
			"Message4": ledgerstate.MasterBranchID,
			"Message5": ledgerstate.MasterBranchID,
		})
	}

	// ISSUE Message6
	{
		testFramework.CreateMessage("Message6", WithStrongParents("Message1", "Message2"), WithLikeParents("Message2"))
		testFramework.IssueMessages("Message6").WaitMessagesBooked()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1": markers.NewMarkers(markers.NewMarker(1, 1)),
			"Message2": markers.NewMarkers(markers.NewMarker(2, 1)),
			"Message3": markers.NewMarkers(markers.NewMarker(3, 1)),
			"Message4": markers.NewMarkers(markers.NewMarker(4, 1)),
			"Message5": markers.NewMarkers(markers.NewMarker(4, 2)),
			"Message6": markers.NewMarkers(markers.NewMarker(2, 2)),
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
			"Message1": testFramework.BranchID("A"),
			"Message2": testFramework.BranchID("B"),
			"Message3": testFramework.BranchID("C"),
			"Message4": ledgerstate.MasterBranchID,
			"Message5": ledgerstate.MasterBranchID,
			"Message6": testFramework.BranchID("B"),
		})
	}

	// ISSUE Message7
	{
		testFramework.CreateMessage("Message7", WithStrongParents("Message6", "Message5"))
		testFramework.IssueMessages("Message7").WaitMessagesBooked()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1": markers.NewMarkers(markers.NewMarker(1, 1)),
			"Message2": markers.NewMarkers(markers.NewMarker(2, 1)),
			"Message3": markers.NewMarkers(markers.NewMarker(3, 1)),
			"Message4": markers.NewMarkers(markers.NewMarker(4, 1)),
			"Message5": markers.NewMarkers(markers.NewMarker(4, 2)),
			"Message6": markers.NewMarkers(markers.NewMarker(2, 2)),
			"Message7": markers.NewMarkers(markers.NewMarker(2, 3)),
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
			"Message1": testFramework.BranchID("A"),
			"Message2": testFramework.BranchID("B"),
			"Message3": testFramework.BranchID("C"),
			"Message4": ledgerstate.MasterBranchID,
			"Message5": ledgerstate.MasterBranchID,
			"Message6": testFramework.BranchID("B"),
			"Message7": testFramework.BranchID("B"),
		})
	}

	// ISSUE Message8
	{
		testFramework.CreateMessage("Message8", WithStrongParents("Message5", "Message7", "Message3"), WithLikeParents("Message1", "Message3"))
		testFramework.IssueMessages("Message8").WaitMessagesBooked()

		testFramework.RegisterBranchID("A+C", "Message1", "Message3")

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1": markers.NewMarkers(markers.NewMarker(1, 1)),
			"Message2": markers.NewMarkers(markers.NewMarker(2, 1)),
			"Message3": markers.NewMarkers(markers.NewMarker(3, 1)),
			"Message4": markers.NewMarkers(markers.NewMarker(4, 1)),
			"Message5": markers.NewMarkers(markers.NewMarker(4, 2)),
			"Message6": markers.NewMarkers(markers.NewMarker(2, 2)),
			"Message7": markers.NewMarkers(markers.NewMarker(2, 3)),
			"Message8": markers.NewMarkers(markers.NewMarker(5, 4)),
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
			"Message1": testFramework.BranchID("A"),
			"Message2": testFramework.BranchID("B"),
			"Message3": testFramework.BranchID("C"),
			"Message4": ledgerstate.MasterBranchID,
			"Message5": ledgerstate.MasterBranchID,
			"Message6": testFramework.BranchID("B"),
			"Message7": testFramework.BranchID("B"),
			"Message8": testFramework.BranchID("A+C"),
		})
	}

	// ISSUE Message9
	{
		testFramework.CreateMessage("Message9", WithStrongParents("Message1", "Message7", "Message3"), WithLikeParents("Message1", "Message3"), WithInputs("F"), WithOutput("N", 500))
		testFramework.IssueMessages("Message9").WaitMessagesBooked()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1": markers.NewMarkers(markers.NewMarker(1, 1)),
			"Message2": markers.NewMarkers(markers.NewMarker(2, 1)),
			"Message3": markers.NewMarkers(markers.NewMarker(3, 1)),
			"Message4": markers.NewMarkers(markers.NewMarker(4, 1)),
			"Message5": markers.NewMarkers(markers.NewMarker(4, 2)),
			"Message6": markers.NewMarkers(markers.NewMarker(2, 2)),
			"Message7": markers.NewMarkers(markers.NewMarker(2, 3)),
			"Message8": markers.NewMarkers(markers.NewMarker(5, 4)),
			"Message9": markers.NewMarkers(markers.NewMarker(2, 3), markers.NewMarker(3, 1)),
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
			"Message9": testFramework.BranchID("A+C"),
		})
		checkIndividuallyMappedMessages(t, testFramework, map[ledgerstate.BranchID][]string{
			testFramework.BranchID("A+C"): {"Message9"},
		})
		checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1": testFramework.BranchID("A"),
			"Message2": testFramework.BranchID("B"),
			"Message3": testFramework.BranchID("C"),
			"Message4": ledgerstate.MasterBranchID,
			"Message5": ledgerstate.MasterBranchID,
			"Message6": testFramework.BranchID("B"),
			"Message7": testFramework.BranchID("B"),
			"Message8": testFramework.BranchID("A+C"),
			"Message9": testFramework.BranchID("A+C"),
		})
	}

	// ISSUE Message10
	{
		testFramework.CreateMessage("Message10", WithStrongParents("Message9"), WithLikeParents("Message2"))
		testFramework.IssueMessages("Message10").WaitMessagesBooked()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1":  markers.NewMarkers(markers.NewMarker(1, 1)),
			"Message2":  markers.NewMarkers(markers.NewMarker(2, 1)),
			"Message3":  markers.NewMarkers(markers.NewMarker(3, 1)),
			"Message4":  markers.NewMarkers(markers.NewMarker(4, 1)),
			"Message5":  markers.NewMarkers(markers.NewMarker(4, 2)),
			"Message6":  markers.NewMarkers(markers.NewMarker(2, 2)),
			"Message7":  markers.NewMarkers(markers.NewMarker(2, 3)),
			"Message8":  markers.NewMarkers(markers.NewMarker(5, 4)),
			"Message9":  markers.NewMarkers(markers.NewMarker(2, 3), markers.NewMarker(3, 1)),
			"Message10": markers.NewMarkers(markers.NewMarker(2, 4)),
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
			"Message9":  testFramework.BranchID("A+C"),
			"Message10": ledgerstate.UndefinedBranchID,
		})
		checkIndividuallyMappedMessages(t, testFramework, map[ledgerstate.BranchID][]string{
			testFramework.BranchID("A+C"): {"Message9"},
		})
		checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1":  testFramework.BranchID("A"),
			"Message2":  testFramework.BranchID("B"),
			"Message3":  testFramework.BranchID("C"),
			"Message4":  ledgerstate.MasterBranchID,
			"Message5":  ledgerstate.MasterBranchID,
			"Message6":  testFramework.BranchID("B"),
			"Message7":  testFramework.BranchID("B"),
			"Message8":  testFramework.BranchID("A+C"),
			"Message9":  testFramework.BranchID("A+C"),
			"Message10": testFramework.BranchID("B"),
		})
	}

	// ISSUE Message11
	{
		testFramework.CreateMessage("Message11", WithStrongParents("Message8", "Message9"))
		testFramework.CreateMessage("Message11.5", WithStrongParents("Message9"))
		testFramework.IssueMessages("Message11", "Message11.5").WaitMessagesBooked()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1":    markers.NewMarkers(markers.NewMarker(1, 1)),
			"Message2":    markers.NewMarkers(markers.NewMarker(2, 1)),
			"Message3":    markers.NewMarkers(markers.NewMarker(3, 1)),
			"Message4":    markers.NewMarkers(markers.NewMarker(4, 1)),
			"Message5":    markers.NewMarkers(markers.NewMarker(4, 2)),
			"Message6":    markers.NewMarkers(markers.NewMarker(2, 2)),
			"Message7":    markers.NewMarkers(markers.NewMarker(2, 3)),
			"Message8":    markers.NewMarkers(markers.NewMarker(5, 4)),
			"Message9":    markers.NewMarkers(markers.NewMarker(2, 3), markers.NewMarker(3, 1)),
			"Message10":   markers.NewMarkers(markers.NewMarker(2, 4)),
			"Message11":   markers.NewMarkers(markers.NewMarker(5, 5)),
			"Message11.5": markers.NewMarkers(markers.NewMarker(2, 3), markers.NewMarker(3, 1)),
		})
		checkMessageMetadataBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1":    ledgerstate.UndefinedBranchID,
			"Message2":    ledgerstate.UndefinedBranchID,
			"Message3":    ledgerstate.UndefinedBranchID,
			"Message4":    ledgerstate.UndefinedBranchID,
			"Message5":    ledgerstate.UndefinedBranchID,
			"Message6":    ledgerstate.UndefinedBranchID,
			"Message7":    ledgerstate.UndefinedBranchID,
			"Message8":    ledgerstate.UndefinedBranchID,
			"Message9":    testFramework.BranchID("A+C"),
			"Message10":   ledgerstate.UndefinedBranchID,
			"Message11":   ledgerstate.UndefinedBranchID,
			"Message11.5": testFramework.BranchID("A+C"),
		})
		checkIndividuallyMappedMessages(t, testFramework, map[ledgerstate.BranchID][]string{
			testFramework.BranchID("A+C"): {"Message9", "Message11.5"},
		})
		checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1":    testFramework.BranchID("A"),
			"Message2":    testFramework.BranchID("B"),
			"Message3":    testFramework.BranchID("C"),
			"Message4":    ledgerstate.MasterBranchID,
			"Message5":    ledgerstate.MasterBranchID,
			"Message6":    testFramework.BranchID("B"),
			"Message7":    testFramework.BranchID("B"),
			"Message8":    testFramework.BranchID("A+C"),
			"Message9":    testFramework.BranchID("A+C"),
			"Message10":   testFramework.BranchID("B"),
			"Message11":   testFramework.BranchID("A+C"),
			"Message11.5": testFramework.BranchID("A+C"),
		})
	}

	// ISSUE Message12
	{
		testFramework.CreateMessage("Message12", WithStrongParents("Message1"), WithInputs("C"), WithOutput("H", 500))
		testFramework.IssueMessages("Message12").WaitMessagesBooked()

		testFramework.RegisterBranchID("D", "Message5")
		testFramework.RegisterBranchID("E", "Message12")
		testFramework.RegisterBranchID("A+E", "Message1", "Message12")
		testFramework.RegisterBranchID("B+D", "Message2", "Message5")
		testFramework.RegisterBranchID("A+C+D", "Message1", "Message3", "Message5")

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1":    markers.NewMarkers(markers.NewMarker(1, 1)),
			"Message2":    markers.NewMarkers(markers.NewMarker(2, 1)),
			"Message3":    markers.NewMarkers(markers.NewMarker(3, 1)),
			"Message4":    markers.NewMarkers(markers.NewMarker(4, 1)),
			"Message5":    markers.NewMarkers(markers.NewMarker(4, 2)),
			"Message6":    markers.NewMarkers(markers.NewMarker(2, 2)),
			"Message7":    markers.NewMarkers(markers.NewMarker(2, 3)),
			"Message8":    markers.NewMarkers(markers.NewMarker(5, 4)),
			"Message9":    markers.NewMarkers(markers.NewMarker(2, 3), markers.NewMarker(3, 1)),
			"Message10":   markers.NewMarkers(markers.NewMarker(2, 4)),
			"Message11":   markers.NewMarkers(markers.NewMarker(5, 5)),
			"Message11.5": markers.NewMarkers(markers.NewMarker(2, 3), markers.NewMarker(3, 1)),
			"Message12":   markers.NewMarkers(markers.NewMarker(6, 2)),
		})
		checkMessageMetadataBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1":    ledgerstate.UndefinedBranchID,
			"Message2":    ledgerstate.UndefinedBranchID,
			"Message3":    ledgerstate.UndefinedBranchID,
			"Message4":    ledgerstate.UndefinedBranchID,
			"Message5":    ledgerstate.UndefinedBranchID,
			"Message6":    ledgerstate.UndefinedBranchID,
			"Message7":    ledgerstate.UndefinedBranchID,
			"Message8":    ledgerstate.UndefinedBranchID,
			"Message9":    testFramework.BranchID("A+C+D"),
			"Message10":   ledgerstate.UndefinedBranchID,
			"Message11":   ledgerstate.UndefinedBranchID,
			"Message11.5": testFramework.BranchID("A+C+D"),
			"Message12":   ledgerstate.UndefinedBranchID,
		})
		checkIndividuallyMappedMessages(t, testFramework, map[ledgerstate.BranchID][]string{
			testFramework.BranchID("A+C+D"): {"Message9", "Message11.5"},
		})
		checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1":    testFramework.BranchID("A"),
			"Message2":    testFramework.BranchID("B"),
			"Message3":    testFramework.BranchID("C"),
			"Message4":    ledgerstate.MasterBranchID,
			"Message5":    testFramework.BranchID("D"),
			"Message6":    testFramework.BranchID("B"),
			"Message7":    testFramework.BranchID("B+D"),
			"Message8":    testFramework.BranchID("A+C+D"),
			"Message9":    testFramework.BranchID("A+C+D"),
			"Message10":   testFramework.BranchID("B+D"),
			"Message11":   testFramework.BranchID("A+C+D"),
			"Message11.5": testFramework.BranchID("A+C+D"),
			"Message12":   testFramework.BranchID("A+E"),
		})
	}

	// ISSUE Message13
	{
		testFramework.CreateMessage("Message13", WithStrongParents("Message9"), WithLikeParents("Message2", "Message12"))
		testFramework.IssueMessages("Message13").WaitMessagesBooked()

		testFramework.RegisterBranchID("B+E", "Message2", "Message12")

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1":    markers.NewMarkers(markers.NewMarker(1, 1)),
			"Message2":    markers.NewMarkers(markers.NewMarker(2, 1)),
			"Message3":    markers.NewMarkers(markers.NewMarker(3, 1)),
			"Message4":    markers.NewMarkers(markers.NewMarker(4, 1)),
			"Message5":    markers.NewMarkers(markers.NewMarker(4, 2)),
			"Message6":    markers.NewMarkers(markers.NewMarker(2, 2)),
			"Message7":    markers.NewMarkers(markers.NewMarker(2, 3)),
			"Message8":    markers.NewMarkers(markers.NewMarker(5, 4)),
			"Message9":    markers.NewMarkers(markers.NewMarker(2, 3), markers.NewMarker(3, 1)),
			"Message10":   markers.NewMarkers(markers.NewMarker(2, 4)),
			"Message11":   markers.NewMarkers(markers.NewMarker(5, 5)),
			"Message11.5": markers.NewMarkers(markers.NewMarker(2, 3), markers.NewMarker(3, 1)),
			"Message12":   markers.NewMarkers(markers.NewMarker(6, 2)),
			"Message13":   markers.NewMarkers(markers.NewMarker(7, 4)),
		})
		checkMessageMetadataBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1":    ledgerstate.UndefinedBranchID,
			"Message2":    ledgerstate.UndefinedBranchID,
			"Message3":    ledgerstate.UndefinedBranchID,
			"Message4":    ledgerstate.UndefinedBranchID,
			"Message5":    ledgerstate.UndefinedBranchID,
			"Message6":    ledgerstate.UndefinedBranchID,
			"Message7":    ledgerstate.UndefinedBranchID,
			"Message8":    ledgerstate.UndefinedBranchID,
			"Message9":    testFramework.BranchID("A+C+D"),
			"Message10":   ledgerstate.UndefinedBranchID,
			"Message11":   ledgerstate.UndefinedBranchID,
			"Message11.5": testFramework.BranchID("A+C+D"),
			"Message12":   ledgerstate.UndefinedBranchID,
			"Message13":   ledgerstate.UndefinedBranchID,
		})
		checkIndividuallyMappedMessages(t, testFramework, map[ledgerstate.BranchID][]string{
			testFramework.BranchID("A+C+D"): {"Message9", "Message11.5"},
		})
		checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1":    testFramework.BranchID("A"),
			"Message2":    testFramework.BranchID("B"),
			"Message3":    testFramework.BranchID("C"),
			"Message4":    ledgerstate.MasterBranchID,
			"Message5":    testFramework.BranchID("D"),
			"Message6":    testFramework.BranchID("B"),
			"Message7":    testFramework.BranchID("B+D"),
			"Message8":    testFramework.BranchID("A+C+D"),
			"Message9":    testFramework.BranchID("A+C+D"),
			"Message10":   testFramework.BranchID("B+D"),
			"Message11":   testFramework.BranchID("A+C+D"),
			"Message11.5": testFramework.BranchID("A+C+D"),
			"Message12":   testFramework.BranchID("A+E"),
			"Message13":   testFramework.BranchID("B+E"),
		})
	}

	// ISSUE Message14
	{
		testFramework.CreateMessage("Message14", WithStrongParents("Message10"), WithLikeParents("Message12"))
		testFramework.IssueMessages("Message14").WaitMessagesBooked()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1":    markers.NewMarkers(markers.NewMarker(1, 1)),
			"Message2":    markers.NewMarkers(markers.NewMarker(2, 1)),
			"Message3":    markers.NewMarkers(markers.NewMarker(3, 1)),
			"Message4":    markers.NewMarkers(markers.NewMarker(4, 1)),
			"Message5":    markers.NewMarkers(markers.NewMarker(4, 2)),
			"Message6":    markers.NewMarkers(markers.NewMarker(2, 2)),
			"Message7":    markers.NewMarkers(markers.NewMarker(2, 3)),
			"Message8":    markers.NewMarkers(markers.NewMarker(5, 4)),
			"Message9":    markers.NewMarkers(markers.NewMarker(2, 3), markers.NewMarker(3, 1)),
			"Message10":   markers.NewMarkers(markers.NewMarker(2, 4)),
			"Message11":   markers.NewMarkers(markers.NewMarker(5, 5)),
			"Message11.5": markers.NewMarkers(markers.NewMarker(2, 3), markers.NewMarker(3, 1)),
			"Message12":   markers.NewMarkers(markers.NewMarker(6, 2)),
			"Message13":   markers.NewMarkers(markers.NewMarker(7, 4)),
			"Message14":   markers.NewMarkers(markers.NewMarker(2, 4)),
		})
		checkMessageMetadataBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1":    ledgerstate.UndefinedBranchID,
			"Message2":    ledgerstate.UndefinedBranchID,
			"Message3":    ledgerstate.UndefinedBranchID,
			"Message4":    ledgerstate.UndefinedBranchID,
			"Message5":    ledgerstate.UndefinedBranchID,
			"Message6":    ledgerstate.UndefinedBranchID,
			"Message7":    ledgerstate.UndefinedBranchID,
			"Message8":    ledgerstate.UndefinedBranchID,
			"Message9":    testFramework.BranchID("A+C+D"),
			"Message10":   ledgerstate.UndefinedBranchID,
			"Message11":   ledgerstate.UndefinedBranchID,
			"Message11.5": testFramework.BranchID("A+C+D"),
			"Message12":   ledgerstate.UndefinedBranchID,
			"Message13":   ledgerstate.UndefinedBranchID,
			"Message14":   testFramework.BranchID("B+E"),
		})
		checkIndividuallyMappedMessages(t, testFramework, map[ledgerstate.BranchID][]string{
			testFramework.BranchID("A+C+D"): {"Message9", "Message11.5"},
			testFramework.BranchID("B+E"):   {"Message14"},
		})
		checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1":    testFramework.BranchID("A"),
			"Message2":    testFramework.BranchID("B"),
			"Message3":    testFramework.BranchID("C"),
			"Message4":    ledgerstate.MasterBranchID,
			"Message5":    testFramework.BranchID("D"),
			"Message6":    testFramework.BranchID("B"),
			"Message7":    testFramework.BranchID("B+D"),
			"Message8":    testFramework.BranchID("A+C+D"),
			"Message9":    testFramework.BranchID("A+C+D"),
			"Message10":   testFramework.BranchID("B+D"),
			"Message11":   testFramework.BranchID("A+C+D"),
			"Message11.5": testFramework.BranchID("A+C+D"),
			"Message12":   testFramework.BranchID("A+E"),
			"Message13":   testFramework.BranchID("B+E"),
			"Message14":   testFramework.BranchID("B+E"),
		})
	}

	// ISSUE Message15
	{
		testFramework.CreateMessage("Message15", WithStrongParents("Message5", "Message14"), WithLikeParents("Message5"), WithInputs("D"), WithOutput("I", 500))
		testFramework.IssueMessages("Message15").WaitMessagesBooked()

		testFramework.RegisterBranchID("B+D", "Message2", "Message5")

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1":    markers.NewMarkers(markers.NewMarker(1, 1)),
			"Message2":    markers.NewMarkers(markers.NewMarker(2, 1)),
			"Message3":    markers.NewMarkers(markers.NewMarker(3, 1)),
			"Message4":    markers.NewMarkers(markers.NewMarker(4, 1)),
			"Message5":    markers.NewMarkers(markers.NewMarker(4, 2)),
			"Message6":    markers.NewMarkers(markers.NewMarker(2, 2)),
			"Message7":    markers.NewMarkers(markers.NewMarker(2, 3)),
			"Message8":    markers.NewMarkers(markers.NewMarker(5, 4)),
			"Message9":    markers.NewMarkers(markers.NewMarker(2, 3), markers.NewMarker(3, 1)),
			"Message10":   markers.NewMarkers(markers.NewMarker(2, 4)),
			"Message11":   markers.NewMarkers(markers.NewMarker(5, 5)),
			"Message11.5": markers.NewMarkers(markers.NewMarker(2, 3), markers.NewMarker(3, 1)),
			"Message12":   markers.NewMarkers(markers.NewMarker(6, 2)),
			"Message13":   markers.NewMarkers(markers.NewMarker(7, 4)),
			"Message14":   markers.NewMarkers(markers.NewMarker(2, 4)),
			"Message15":   markers.NewMarkers(markers.NewMarker(2, 5)),
		})
		checkMessageMetadataBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1":    ledgerstate.UndefinedBranchID,
			"Message2":    ledgerstate.UndefinedBranchID,
			"Message3":    ledgerstate.UndefinedBranchID,
			"Message4":    ledgerstate.UndefinedBranchID,
			"Message5":    ledgerstate.UndefinedBranchID,
			"Message6":    ledgerstate.UndefinedBranchID,
			"Message7":    ledgerstate.UndefinedBranchID,
			"Message8":    ledgerstate.UndefinedBranchID,
			"Message9":    testFramework.BranchID("A+C+D"),
			"Message10":   ledgerstate.UndefinedBranchID,
			"Message11":   ledgerstate.UndefinedBranchID,
			"Message11.5": testFramework.BranchID("A+C+D"),
			"Message12":   ledgerstate.UndefinedBranchID,
			"Message13":   ledgerstate.UndefinedBranchID,
			"Message14":   testFramework.BranchID("B+E"),
			"Message15":   ledgerstate.UndefinedBranchID,
		})
		checkIndividuallyMappedMessages(t, testFramework, map[ledgerstate.BranchID][]string{
			testFramework.BranchID("A+C+D"): {"Message9", "Message11.5"},
			testFramework.BranchID("B+E"):   {"Message14"},
		})
		checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1":    testFramework.BranchID("A"),
			"Message2":    testFramework.BranchID("B"),
			"Message3":    testFramework.BranchID("C"),
			"Message4":    ledgerstate.MasterBranchID,
			"Message5":    testFramework.BranchID("D"),
			"Message6":    testFramework.BranchID("B"),
			"Message7":    testFramework.BranchID("B+D"),
			"Message8":    testFramework.BranchID("A+C+D"),
			"Message9":    testFramework.BranchID("A+C+D"),
			"Message10":   testFramework.BranchID("B+D"),
			"Message11":   testFramework.BranchID("A+C+D"),
			"Message11.5": testFramework.BranchID("A+C+D"),
			"Message12":   testFramework.BranchID("A+E"),
			"Message13":   testFramework.BranchID("B+E"),
			"Message14":   testFramework.BranchID("B+E"),
			"Message15":   testFramework.BranchID("B+D"),
		})
	}

	// ISSUE Message16
	{
		testFramework.CreateMessage("Message16", WithStrongParents("Genesis"), WithInputs("L"), WithOutput("J", 500))
		testFramework.IssueMessages("Message16").WaitMessagesBooked()

		testFramework.RegisterBranchID("F", "Message4")
		testFramework.RegisterBranchID("G", "Message16")
		testFramework.RegisterBranchID("D+F", "Message5", "Message4")
		testFramework.RegisterBranchID("B+D+F", "Message2", "Message5", "Message4")
		testFramework.RegisterBranchID("A+C+D+F", "Message1", "Message3", "Message5", "Message4")
		testFramework.RegisterBranchID("B+E+F", "Message2", "Message12", "Message4")

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1":    markers.NewMarkers(markers.NewMarker(1, 1)),
			"Message2":    markers.NewMarkers(markers.NewMarker(2, 1)),
			"Message3":    markers.NewMarkers(markers.NewMarker(3, 1)),
			"Message4":    markers.NewMarkers(markers.NewMarker(4, 1)),
			"Message5":    markers.NewMarkers(markers.NewMarker(4, 2)),
			"Message6":    markers.NewMarkers(markers.NewMarker(2, 2)),
			"Message7":    markers.NewMarkers(markers.NewMarker(2, 3)),
			"Message8":    markers.NewMarkers(markers.NewMarker(5, 4)),
			"Message9":    markers.NewMarkers(markers.NewMarker(2, 3), markers.NewMarker(3, 1)),
			"Message10":   markers.NewMarkers(markers.NewMarker(2, 4)),
			"Message11":   markers.NewMarkers(markers.NewMarker(5, 5)),
			"Message11.5": markers.NewMarkers(markers.NewMarker(2, 3), markers.NewMarker(3, 1)),
			"Message12":   markers.NewMarkers(markers.NewMarker(6, 2)),
			"Message13":   markers.NewMarkers(markers.NewMarker(7, 4)),
			"Message14":   markers.NewMarkers(markers.NewMarker(2, 4)),
			"Message15":   markers.NewMarkers(markers.NewMarker(2, 5)),
			"Message16":   markers.NewMarkers(markers.NewMarker(8, 1)),
		})
		checkMessageMetadataBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1":    ledgerstate.UndefinedBranchID,
			"Message2":    ledgerstate.UndefinedBranchID,
			"Message3":    ledgerstate.UndefinedBranchID,
			"Message4":    ledgerstate.UndefinedBranchID,
			"Message5":    ledgerstate.UndefinedBranchID,
			"Message6":    ledgerstate.UndefinedBranchID,
			"Message7":    ledgerstate.UndefinedBranchID,
			"Message8":    ledgerstate.UndefinedBranchID,
			"Message9":    testFramework.BranchID("A+C+D+F"),
			"Message10":   ledgerstate.UndefinedBranchID,
			"Message11":   ledgerstate.UndefinedBranchID,
			"Message11.5": testFramework.BranchID("A+C+D+F"),
			"Message12":   ledgerstate.UndefinedBranchID,
			"Message13":   ledgerstate.UndefinedBranchID,
			"Message14":   testFramework.BranchID("B+E+F"),
			"Message15":   ledgerstate.UndefinedBranchID,
			"Message16":   ledgerstate.UndefinedBranchID,
		})
		checkIndividuallyMappedMessages(t, testFramework, map[ledgerstate.BranchID][]string{
			testFramework.BranchID("A+C+D+F"): {"Message9", "Message11.5"},
			testFramework.BranchID("B+E+F"):   {"Message14"},
		})
		checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1":    testFramework.BranchID("A"),
			"Message2":    testFramework.BranchID("B"),
			"Message3":    testFramework.BranchID("C"),
			"Message4":    testFramework.BranchID("F"),
			"Message5":    testFramework.BranchID("D+F"),
			"Message6":    testFramework.BranchID("B"),
			"Message7":    testFramework.BranchID("B+D+F"),
			"Message8":    testFramework.BranchID("A+C+D+F"),
			"Message9":    testFramework.BranchID("A+C+D+F"),
			"Message10":   testFramework.BranchID("B+D+F"),
			"Message11":   testFramework.BranchID("A+C+D+F"),
			"Message11.5": testFramework.BranchID("A+C+D+F"),
			"Message12":   testFramework.BranchID("A+E"),
			"Message13":   testFramework.BranchID("B+E+F"),
			"Message14":   testFramework.BranchID("B+E+F"),
			"Message15":   testFramework.BranchID("B+D+F"),
			"Message16":   testFramework.BranchID("G"),
		})
	}

	// ISSUE Message17
	{
		testFramework.CreateMessage("Message17", WithStrongParents("Message14"))
		testFramework.IssueMessages("Message17").WaitMessagesBooked()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1":    markers.NewMarkers(markers.NewMarker(1, 1)),
			"Message2":    markers.NewMarkers(markers.NewMarker(2, 1)),
			"Message3":    markers.NewMarkers(markers.NewMarker(3, 1)),
			"Message4":    markers.NewMarkers(markers.NewMarker(4, 1)),
			"Message5":    markers.NewMarkers(markers.NewMarker(4, 2)),
			"Message6":    markers.NewMarkers(markers.NewMarker(2, 2)),
			"Message7":    markers.NewMarkers(markers.NewMarker(2, 3)),
			"Message8":    markers.NewMarkers(markers.NewMarker(5, 4)),
			"Message9":    markers.NewMarkers(markers.NewMarker(2, 3), markers.NewMarker(3, 1)),
			"Message10":   markers.NewMarkers(markers.NewMarker(2, 4)),
			"Message11":   markers.NewMarkers(markers.NewMarker(5, 5)),
			"Message11.5": markers.NewMarkers(markers.NewMarker(2, 3), markers.NewMarker(3, 1)),
			"Message12":   markers.NewMarkers(markers.NewMarker(6, 2)),
			"Message13":   markers.NewMarkers(markers.NewMarker(7, 4)),
			"Message14":   markers.NewMarkers(markers.NewMarker(2, 4)),
			"Message15":   markers.NewMarkers(markers.NewMarker(2, 5)),
			"Message16":   markers.NewMarkers(markers.NewMarker(8, 1)),
			"Message17":   markers.NewMarkers(markers.NewMarker(2, 4)),
		})
		checkMessageMetadataBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1":    ledgerstate.UndefinedBranchID,
			"Message2":    ledgerstate.UndefinedBranchID,
			"Message3":    ledgerstate.UndefinedBranchID,
			"Message4":    ledgerstate.UndefinedBranchID,
			"Message5":    ledgerstate.UndefinedBranchID,
			"Message6":    ledgerstate.UndefinedBranchID,
			"Message7":    ledgerstate.UndefinedBranchID,
			"Message8":    ledgerstate.UndefinedBranchID,
			"Message9":    testFramework.BranchID("A+C+D+F"),
			"Message10":   ledgerstate.UndefinedBranchID,
			"Message11":   ledgerstate.UndefinedBranchID,
			"Message11.5": testFramework.BranchID("A+C+D+F"),
			"Message12":   ledgerstate.UndefinedBranchID,
			"Message13":   ledgerstate.UndefinedBranchID,
			"Message14":   testFramework.BranchID("B+E+F"),
			"Message15":   ledgerstate.UndefinedBranchID,
			"Message16":   ledgerstate.UndefinedBranchID,
			"Message17":   testFramework.BranchID("B+E+F"),
		})
		checkIndividuallyMappedMessages(t, testFramework, map[ledgerstate.BranchID][]string{
			testFramework.BranchID("A+C+D+F"): {"Message9", "Message11.5"},
			testFramework.BranchID("B+E+F"):   {"Message14", "Message17"},
		})
		checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1":    testFramework.BranchID("A"),
			"Message2":    testFramework.BranchID("B"),
			"Message3":    testFramework.BranchID("C"),
			"Message4":    testFramework.BranchID("F"),
			"Message5":    testFramework.BranchID("D+F"),
			"Message6":    testFramework.BranchID("B"),
			"Message7":    testFramework.BranchID("B+D+F"),
			"Message8":    testFramework.BranchID("A+C+D+F"),
			"Message9":    testFramework.BranchID("A+C+D+F"),
			"Message10":   testFramework.BranchID("B+D+F"),
			"Message11":   testFramework.BranchID("A+C+D+F"),
			"Message11.5": testFramework.BranchID("A+C+D+F"),
			"Message12":   testFramework.BranchID("A+E"),
			"Message13":   testFramework.BranchID("B+E+F"),
			"Message14":   testFramework.BranchID("B+E+F"),
			"Message15":   testFramework.BranchID("B+D+F"),
			"Message16":   testFramework.BranchID("G"),
			"Message17":   testFramework.BranchID("B+E+F"),
		})
	}

	// ISSUE Message18
	{
		testFramework.CreateMessage("Message18", WithStrongParents("Message17", "Message13"))
		testFramework.IssueMessages("Message18").WaitMessagesBooked()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1":    markers.NewMarkers(markers.NewMarker(1, 1)),
			"Message2":    markers.NewMarkers(markers.NewMarker(2, 1)),
			"Message3":    markers.NewMarkers(markers.NewMarker(3, 1)),
			"Message4":    markers.NewMarkers(markers.NewMarker(4, 1)),
			"Message5":    markers.NewMarkers(markers.NewMarker(4, 2)),
			"Message6":    markers.NewMarkers(markers.NewMarker(2, 2)),
			"Message7":    markers.NewMarkers(markers.NewMarker(2, 3)),
			"Message8":    markers.NewMarkers(markers.NewMarker(5, 4)),
			"Message9":    markers.NewMarkers(markers.NewMarker(2, 3), markers.NewMarker(3, 1)),
			"Message10":   markers.NewMarkers(markers.NewMarker(2, 4)),
			"Message11":   markers.NewMarkers(markers.NewMarker(5, 5)),
			"Message11.5": markers.NewMarkers(markers.NewMarker(2, 3), markers.NewMarker(3, 1)),
			"Message12":   markers.NewMarkers(markers.NewMarker(6, 2)),
			"Message13":   markers.NewMarkers(markers.NewMarker(7, 4)),
			"Message14":   markers.NewMarkers(markers.NewMarker(2, 4)),
			"Message15":   markers.NewMarkers(markers.NewMarker(2, 5)),
			"Message16":   markers.NewMarkers(markers.NewMarker(8, 1)),
			"Message17":   markers.NewMarkers(markers.NewMarker(2, 4)),
			"Message18":   markers.NewMarkers(markers.NewMarker(7, 5)),
		})
		checkMessageMetadataBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1":    ledgerstate.UndefinedBranchID,
			"Message2":    ledgerstate.UndefinedBranchID,
			"Message3":    ledgerstate.UndefinedBranchID,
			"Message4":    ledgerstate.UndefinedBranchID,
			"Message5":    ledgerstate.UndefinedBranchID,
			"Message6":    ledgerstate.UndefinedBranchID,
			"Message7":    ledgerstate.UndefinedBranchID,
			"Message8":    ledgerstate.UndefinedBranchID,
			"Message9":    testFramework.BranchID("A+C+D+F"),
			"Message10":   ledgerstate.UndefinedBranchID,
			"Message11":   ledgerstate.UndefinedBranchID,
			"Message11.5": testFramework.BranchID("A+C+D+F"),
			"Message12":   ledgerstate.UndefinedBranchID,
			"Message13":   ledgerstate.UndefinedBranchID,
			"Message14":   testFramework.BranchID("B+E+F"),
			"Message15":   ledgerstate.UndefinedBranchID,
			"Message16":   ledgerstate.UndefinedBranchID,
			"Message17":   testFramework.BranchID("B+E+F"),
			"Message18":   ledgerstate.UndefinedBranchID,
		})
		checkIndividuallyMappedMessages(t, testFramework, map[ledgerstate.BranchID][]string{
			testFramework.BranchID("A+C+D+F"): {"Message9", "Message11.5"},
			testFramework.BranchID("B+E+F"):   {"Message14", "Message17"},
		})
		checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1":    testFramework.BranchID("A"),
			"Message2":    testFramework.BranchID("B"),
			"Message3":    testFramework.BranchID("C"),
			"Message4":    testFramework.BranchID("F"),
			"Message5":    testFramework.BranchID("D+F"),
			"Message6":    testFramework.BranchID("B"),
			"Message7":    testFramework.BranchID("B+D+F"),
			"Message8":    testFramework.BranchID("A+C+D+F"),
			"Message9":    testFramework.BranchID("A+C+D+F"),
			"Message10":   testFramework.BranchID("B+D+F"),
			"Message11":   testFramework.BranchID("A+C+D+F"),
			"Message11.5": testFramework.BranchID("A+C+D+F"),
			"Message12":   testFramework.BranchID("A+E"),
			"Message13":   testFramework.BranchID("B+E+F"),
			"Message14":   testFramework.BranchID("B+E+F"),
			"Message15":   testFramework.BranchID("B+D+F"),
			"Message16":   testFramework.BranchID("G"),
			"Message17":   testFramework.BranchID("B+E+F"),
			"Message18":   testFramework.BranchID("B+E+F"),
		})
	}

	// ISSUE Message19
	{
		testFramework.CreateMessage("Message19", WithStrongParents("Message3", "Message7"), WithLikeParents("Message3"), WithInputs("F"), WithOutput("M", 500))
		testFramework.IssueMessages("Message19").WaitMessagesBooked()

		testFramework.RegisterBranchID("H", "Message9")
		testFramework.RegisterBranchID("I", "Message19")
		testFramework.RegisterBranchID("D+F+I", "Message5", "Message4", "Message19")
		testFramework.RegisterBranchID("A+D+F+H", "Message1", "Message5", "Message4", "Message9")

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1":    markers.NewMarkers(markers.NewMarker(1, 1)),
			"Message2":    markers.NewMarkers(markers.NewMarker(2, 1)),
			"Message3":    markers.NewMarkers(markers.NewMarker(3, 1)),
			"Message4":    markers.NewMarkers(markers.NewMarker(4, 1)),
			"Message5":    markers.NewMarkers(markers.NewMarker(4, 2)),
			"Message6":    markers.NewMarkers(markers.NewMarker(2, 2)),
			"Message7":    markers.NewMarkers(markers.NewMarker(2, 3)),
			"Message8":    markers.NewMarkers(markers.NewMarker(5, 4)),
			"Message9":    markers.NewMarkers(markers.NewMarker(2, 3), markers.NewMarker(3, 1)),
			"Message10":   markers.NewMarkers(markers.NewMarker(2, 4)),
			"Message11":   markers.NewMarkers(markers.NewMarker(5, 5)),
			"Message11.5": markers.NewMarkers(markers.NewMarker(2, 3), markers.NewMarker(3, 1)),
			"Message12":   markers.NewMarkers(markers.NewMarker(6, 2)),
			"Message13":   markers.NewMarkers(markers.NewMarker(7, 4)),
			"Message14":   markers.NewMarkers(markers.NewMarker(2, 4)),
			"Message15":   markers.NewMarkers(markers.NewMarker(2, 5)),
			"Message16":   markers.NewMarkers(markers.NewMarker(8, 1)),
			"Message17":   markers.NewMarkers(markers.NewMarker(2, 4)),
			"Message18":   markers.NewMarkers(markers.NewMarker(7, 5)),
			"Message19":   markers.NewMarkers(markers.NewMarker(9, 4)),
		})
		checkMessageMetadataBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1":    ledgerstate.UndefinedBranchID,
			"Message2":    ledgerstate.UndefinedBranchID,
			"Message3":    ledgerstate.UndefinedBranchID,
			"Message4":    ledgerstate.UndefinedBranchID,
			"Message5":    ledgerstate.UndefinedBranchID,
			"Message6":    ledgerstate.UndefinedBranchID,
			"Message7":    ledgerstate.UndefinedBranchID,
			"Message8":    ledgerstate.UndefinedBranchID,
			"Message9":    testFramework.BranchID("A+D+F+H"),
			"Message10":   ledgerstate.UndefinedBranchID,
			"Message11":   ledgerstate.UndefinedBranchID,
			"Message11.5": testFramework.BranchID("A+D+F+H"),
			"Message12":   ledgerstate.UndefinedBranchID,
			"Message13":   ledgerstate.UndefinedBranchID,
			"Message14":   testFramework.BranchID("B+E+F"),
			"Message15":   ledgerstate.UndefinedBranchID,
			"Message16":   ledgerstate.UndefinedBranchID,
			"Message17":   testFramework.BranchID("B+E+F"),
			"Message18":   ledgerstate.UndefinedBranchID,
			"Message19":   ledgerstate.UndefinedBranchID,
		})
		checkIndividuallyMappedMessages(t, testFramework, map[ledgerstate.BranchID][]string{
			testFramework.BranchID("A+D+F+H"): {"Message9", "Message11.5"},
			testFramework.BranchID("B+E+F"):   {"Message14", "Message17"},
		})
		checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1":    testFramework.BranchID("A"),
			"Message2":    testFramework.BranchID("B"),
			"Message3":    testFramework.BranchID("C"),
			"Message4":    testFramework.BranchID("F"),
			"Message5":    testFramework.BranchID("D+F"),
			"Message6":    testFramework.BranchID("B"),
			"Message7":    testFramework.BranchID("B+D+F"),
			"Message8":    testFramework.BranchID("A+C+D+F"),
			"Message9":    testFramework.BranchID("A+D+F+H"),
			"Message10":   testFramework.BranchID("B+D+F"),
			"Message11":   testFramework.BranchID("A+D+F+H"),
			"Message11.5": testFramework.BranchID("A+D+F+H"),
			"Message12":   testFramework.BranchID("A+E"),
			"Message13":   testFramework.BranchID("B+E+F"),
			"Message14":   testFramework.BranchID("B+E+F"),
			"Message15":   testFramework.BranchID("B+D+F"),
			"Message16":   testFramework.BranchID("G"),
			"Message17":   testFramework.BranchID("B+E+F"),
			"Message18":   testFramework.BranchID("B+E+F"),
			"Message19":   testFramework.BranchID("D+F+I"),
		})
	}

	// ISSUE Message20 - 23
	{
		testFramework.CreateMessage("Message20", WithStrongParents("Message4"))
		testFramework.CreateMessage("Message21", WithStrongParents("Message4"))
		testFramework.CreateMessage("Message22", WithStrongParents("Message20", "Message21"))
		testFramework.CreateMessage("Message23", WithStrongParents("Message5", "Message22"))
		testFramework.IssueMessages("Message20").WaitMessagesBooked()
		testFramework.IssueMessages("Message21").WaitMessagesBooked()
		testFramework.IssueMessages("Message22").WaitMessagesBooked()
		testFramework.IssueMessages("Message23").WaitMessagesBooked()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1":    markers.NewMarkers(markers.NewMarker(1, 1)),
			"Message2":    markers.NewMarkers(markers.NewMarker(2, 1)),
			"Message3":    markers.NewMarkers(markers.NewMarker(3, 1)),
			"Message4":    markers.NewMarkers(markers.NewMarker(4, 1)),
			"Message5":    markers.NewMarkers(markers.NewMarker(4, 2)),
			"Message6":    markers.NewMarkers(markers.NewMarker(2, 2)),
			"Message7":    markers.NewMarkers(markers.NewMarker(2, 3)),
			"Message8":    markers.NewMarkers(markers.NewMarker(5, 4)),
			"Message9":    markers.NewMarkers(markers.NewMarker(2, 3), markers.NewMarker(3, 1)),
			"Message10":   markers.NewMarkers(markers.NewMarker(2, 4)),
			"Message11":   markers.NewMarkers(markers.NewMarker(5, 5)),
			"Message11.5": markers.NewMarkers(markers.NewMarker(2, 3), markers.NewMarker(3, 1)),
			"Message12":   markers.NewMarkers(markers.NewMarker(6, 2)),
			"Message13":   markers.NewMarkers(markers.NewMarker(7, 4)),
			"Message14":   markers.NewMarkers(markers.NewMarker(2, 4)),
			"Message15":   markers.NewMarkers(markers.NewMarker(2, 5)),
			"Message16":   markers.NewMarkers(markers.NewMarker(8, 1)),
			"Message17":   markers.NewMarkers(markers.NewMarker(2, 4)),
			"Message18":   markers.NewMarkers(markers.NewMarker(7, 5)),
			"Message19":   markers.NewMarkers(markers.NewMarker(9, 4)),
			"Message20":   markers.NewMarkers(markers.NewMarker(10, 2)),
			"Message21":   markers.NewMarkers(markers.NewMarker(4, 1)),
			"Message22":   markers.NewMarkers(markers.NewMarker(10, 3)),
			"Message23":   markers.NewMarkers(markers.NewMarker(4, 4)),
		})
		checkMessageMetadataBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1":    ledgerstate.UndefinedBranchID,
			"Message2":    ledgerstate.UndefinedBranchID,
			"Message3":    ledgerstate.UndefinedBranchID,
			"Message4":    ledgerstate.UndefinedBranchID,
			"Message5":    ledgerstate.UndefinedBranchID,
			"Message6":    ledgerstate.UndefinedBranchID,
			"Message7":    ledgerstate.UndefinedBranchID,
			"Message8":    ledgerstate.UndefinedBranchID,
			"Message9":    testFramework.BranchID("A+D+F+H"),
			"Message10":   ledgerstate.UndefinedBranchID,
			"Message11":   ledgerstate.UndefinedBranchID,
			"Message11.5": testFramework.BranchID("A+D+F+H"),
			"Message12":   ledgerstate.UndefinedBranchID,
			"Message13":   ledgerstate.UndefinedBranchID,
			"Message14":   testFramework.BranchID("B+E+F"),
			"Message15":   ledgerstate.UndefinedBranchID,
			"Message16":   ledgerstate.UndefinedBranchID,
			"Message17":   testFramework.BranchID("B+E+F"),
			"Message18":   ledgerstate.UndefinedBranchID,
			"Message19":   ledgerstate.UndefinedBranchID,
			"Message20":   ledgerstate.UndefinedBranchID,
			"Message21":   ledgerstate.UndefinedBranchID,
			"Message22":   ledgerstate.UndefinedBranchID,
			"Message23":   ledgerstate.UndefinedBranchID,
		})
		checkIndividuallyMappedMessages(t, testFramework, map[ledgerstate.BranchID][]string{
			testFramework.BranchID("A+D+F+H"): {"Message9", "Message11.5"},
			testFramework.BranchID("B+E+F"):   {"Message14", "Message17"},
		})
		checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1":    testFramework.BranchID("A"),
			"Message2":    testFramework.BranchID("B"),
			"Message3":    testFramework.BranchID("C"),
			"Message4":    testFramework.BranchID("F"),
			"Message5":    testFramework.BranchID("D+F"),
			"Message6":    testFramework.BranchID("B"),
			"Message7":    testFramework.BranchID("B+D+F"),
			"Message8":    testFramework.BranchID("A+C+D+F"),
			"Message9":    testFramework.BranchID("A+D+F+H"),
			"Message10":   testFramework.BranchID("B+D+F"),
			"Message11":   testFramework.BranchID("A+D+F+H"),
			"Message11.5": testFramework.BranchID("A+D+F+H"),
			"Message12":   testFramework.BranchID("A+E"),
			"Message13":   testFramework.BranchID("B+E+F"),
			"Message14":   testFramework.BranchID("B+E+F"),
			"Message15":   testFramework.BranchID("B+D+F"),
			"Message16":   testFramework.BranchID("G"),
			"Message17":   testFramework.BranchID("B+E+F"),
			"Message18":   testFramework.BranchID("B+E+F"),
			"Message19":   testFramework.BranchID("D+F+I"),
			"Message20":   testFramework.BranchID("F"),
			"Message21":   testFramework.BranchID("F"),
			"Message22":   testFramework.BranchID("F"),
			"Message23":   testFramework.BranchID("D+F"),
		})
	}

	// ISSUE Message30
	{
		msg := testFramework.CreateMessage("Message30", WithStrongParents("Message1", "Message2"))
		testFramework.IssueMessages("Message30").WaitMessagesBooked()
		tangle.Storage.MessageMetadata(msg.ID()).Consume(func(messageMetadata *MessageMetadata) {
			assert.True(t, messageMetadata.invalid)
		})

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1":    markers.NewMarkers(markers.NewMarker(1, 1)),
			"Message2":    markers.NewMarkers(markers.NewMarker(2, 1)),
			"Message3":    markers.NewMarkers(markers.NewMarker(3, 1)),
			"Message4":    markers.NewMarkers(markers.NewMarker(4, 1)),
			"Message5":    markers.NewMarkers(markers.NewMarker(4, 2)),
			"Message6":    markers.NewMarkers(markers.NewMarker(2, 2)),
			"Message7":    markers.NewMarkers(markers.NewMarker(2, 3)),
			"Message8":    markers.NewMarkers(markers.NewMarker(5, 4)),
			"Message9":    markers.NewMarkers(markers.NewMarker(2, 3), markers.NewMarker(3, 1)),
			"Message10":   markers.NewMarkers(markers.NewMarker(2, 4)),
			"Message11":   markers.NewMarkers(markers.NewMarker(5, 5)),
			"Message11.5": markers.NewMarkers(markers.NewMarker(2, 3), markers.NewMarker(3, 1)),
			"Message12":   markers.NewMarkers(markers.NewMarker(6, 2)),
			"Message13":   markers.NewMarkers(markers.NewMarker(7, 4)),
			"Message14":   markers.NewMarkers(markers.NewMarker(2, 4)),
			"Message15":   markers.NewMarkers(markers.NewMarker(2, 5)),
			"Message16":   markers.NewMarkers(markers.NewMarker(8, 1)),
			"Message17":   markers.NewMarkers(markers.NewMarker(2, 4)),
			"Message18":   markers.NewMarkers(markers.NewMarker(7, 5)),
			"Message19":   markers.NewMarkers(markers.NewMarker(9, 4)),
		})
		checkMessageMetadataBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1":    ledgerstate.UndefinedBranchID,
			"Message2":    ledgerstate.UndefinedBranchID,
			"Message3":    ledgerstate.UndefinedBranchID,
			"Message4":    ledgerstate.UndefinedBranchID,
			"Message5":    ledgerstate.UndefinedBranchID,
			"Message6":    ledgerstate.UndefinedBranchID,
			"Message7":    ledgerstate.UndefinedBranchID,
			"Message8":    ledgerstate.UndefinedBranchID,
			"Message9":    testFramework.BranchID("A+D+F+H"),
			"Message10":   ledgerstate.UndefinedBranchID,
			"Message11":   ledgerstate.UndefinedBranchID,
			"Message11.5": testFramework.BranchID("A+D+F+H"),
			"Message12":   ledgerstate.UndefinedBranchID,
			"Message13":   ledgerstate.UndefinedBranchID,
			"Message14":   testFramework.BranchID("B+E+F"),
			"Message15":   ledgerstate.UndefinedBranchID,
			"Message16":   ledgerstate.UndefinedBranchID,
			"Message17":   testFramework.BranchID("B+E+F"),
			"Message18":   ledgerstate.UndefinedBranchID,
			"Message19":   ledgerstate.UndefinedBranchID,
		})
		checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1":    testFramework.BranchID("A"),
			"Message2":    testFramework.BranchID("B"),
			"Message3":    testFramework.BranchID("C"),
			"Message4":    testFramework.BranchID("F"),
			"Message5":    testFramework.BranchID("D+F"),
			"Message6":    testFramework.BranchID("B"),
			"Message7":    testFramework.BranchID("B+D+F"),
			"Message8":    testFramework.BranchID("A+C+D+F"),
			"Message9":    testFramework.BranchID("A+D+F+H"),
			"Message10":   testFramework.BranchID("B+D+F"),
			"Message11":   testFramework.BranchID("A+D+F+H"),
			"Message11.5": testFramework.BranchID("A+D+F+H"),
			"Message12":   testFramework.BranchID("A+E"),
			"Message13":   testFramework.BranchID("B+E+F"),
			"Message14":   testFramework.BranchID("B+E+F"),
			"Message15":   testFramework.BranchID("B+D+F"),
			"Message16":   testFramework.BranchID("G"),
			"Message17":   testFramework.BranchID("B+E+F"),
			"Message18":   testFramework.BranchID("B+E+F"),
			"Message19":   testFramework.BranchID("D+F+I"),
		})
	}

	// ISSUE Message31
	{
		msg := testFramework.CreateMessage("Message31", WithStrongParents("Message1", "Message2"), WithLikeParents("Message2"), WithInputs("G"), WithOutput("O", 500))
		testFramework.IssueMessages("Message31").WaitMessagesBooked()
		tangle.Storage.MessageMetadata(msg.ID()).Consume(func(messageMetadata *MessageMetadata) {
			assert.True(t, messageMetadata.invalid)
		})

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1":    markers.NewMarkers(markers.NewMarker(1, 1)),
			"Message2":    markers.NewMarkers(markers.NewMarker(2, 1)),
			"Message3":    markers.NewMarkers(markers.NewMarker(3, 1)),
			"Message4":    markers.NewMarkers(markers.NewMarker(4, 1)),
			"Message5":    markers.NewMarkers(markers.NewMarker(4, 2)),
			"Message6":    markers.NewMarkers(markers.NewMarker(2, 2)),
			"Message7":    markers.NewMarkers(markers.NewMarker(2, 3)),
			"Message8":    markers.NewMarkers(markers.NewMarker(5, 4)),
			"Message9":    markers.NewMarkers(markers.NewMarker(2, 3), markers.NewMarker(3, 1)),
			"Message10":   markers.NewMarkers(markers.NewMarker(2, 4)),
			"Message11":   markers.NewMarkers(markers.NewMarker(5, 5)),
			"Message11.5": markers.NewMarkers(markers.NewMarker(2, 3), markers.NewMarker(3, 1)),
			"Message12":   markers.NewMarkers(markers.NewMarker(6, 2)),
			"Message13":   markers.NewMarkers(markers.NewMarker(7, 4)),
			"Message14":   markers.NewMarkers(markers.NewMarker(2, 4)),
			"Message15":   markers.NewMarkers(markers.NewMarker(2, 5)),
			"Message16":   markers.NewMarkers(markers.NewMarker(8, 1)),
			"Message17":   markers.NewMarkers(markers.NewMarker(2, 4)),
			"Message18":   markers.NewMarkers(markers.NewMarker(7, 5)),
			"Message19":   markers.NewMarkers(markers.NewMarker(9, 4)),
		})
		checkMessageMetadataBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1":    ledgerstate.UndefinedBranchID,
			"Message2":    ledgerstate.UndefinedBranchID,
			"Message3":    ledgerstate.UndefinedBranchID,
			"Message4":    ledgerstate.UndefinedBranchID,
			"Message5":    ledgerstate.UndefinedBranchID,
			"Message6":    ledgerstate.UndefinedBranchID,
			"Message7":    ledgerstate.UndefinedBranchID,
			"Message8":    ledgerstate.UndefinedBranchID,
			"Message9":    testFramework.BranchID("A+D+F+H"),
			"Message10":   ledgerstate.UndefinedBranchID,
			"Message11":   ledgerstate.UndefinedBranchID,
			"Message11.5": testFramework.BranchID("A+D+F+H"),
			"Message12":   ledgerstate.UndefinedBranchID,
			"Message13":   ledgerstate.UndefinedBranchID,
			"Message14":   testFramework.BranchID("B+E+F"),
			"Message15":   ledgerstate.UndefinedBranchID,
			"Message16":   ledgerstate.UndefinedBranchID,
			"Message17":   testFramework.BranchID("B+E+F"),
			"Message18":   ledgerstate.UndefinedBranchID,
			"Message19":   ledgerstate.UndefinedBranchID,
		})
		checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1":    testFramework.BranchID("A"),
			"Message2":    testFramework.BranchID("B"),
			"Message3":    testFramework.BranchID("C"),
			"Message4":    testFramework.BranchID("F"),
			"Message5":    testFramework.BranchID("D+F"),
			"Message6":    testFramework.BranchID("B"),
			"Message7":    testFramework.BranchID("B+D+F"),
			"Message8":    testFramework.BranchID("A+C+D+F"),
			"Message9":    testFramework.BranchID("A+D+F+H"),
			"Message10":   testFramework.BranchID("B+D+F"),
			"Message11":   testFramework.BranchID("A+D+F+H"),
			"Message11.5": testFramework.BranchID("A+D+F+H"),
			"Message12":   testFramework.BranchID("A+E"),
			"Message13":   testFramework.BranchID("B+E+F"),
			"Message14":   testFramework.BranchID("B+E+F"),
			"Message15":   testFramework.BranchID("B+D+F"),
			"Message16":   testFramework.BranchID("G"),
			"Message17":   testFramework.BranchID("B+E+F"),
			"Message18":   testFramework.BranchID("B+E+F"),
			"Message19":   testFramework.BranchID("D+F+I"),
		})
	}

	// ISSUE Message32
	{
		msg := testFramework.CreateMessage("Message32", WithStrongParents("Message5"), WithLikeParents("Message6"))
		testFramework.IssueMessages("Message32").WaitMessagesBooked()
		tangle.Storage.MessageMetadata(msg.ID()).Consume(func(messageMetadata *MessageMetadata) {
			assert.True(t, messageMetadata.invalid)
		})

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1":    markers.NewMarkers(markers.NewMarker(1, 1)),
			"Message2":    markers.NewMarkers(markers.NewMarker(2, 1)),
			"Message3":    markers.NewMarkers(markers.NewMarker(3, 1)),
			"Message4":    markers.NewMarkers(markers.NewMarker(4, 1)),
			"Message5":    markers.NewMarkers(markers.NewMarker(4, 2)),
			"Message6":    markers.NewMarkers(markers.NewMarker(2, 2)),
			"Message7":    markers.NewMarkers(markers.NewMarker(2, 3)),
			"Message8":    markers.NewMarkers(markers.NewMarker(5, 4)),
			"Message9":    markers.NewMarkers(markers.NewMarker(2, 3), markers.NewMarker(3, 1)),
			"Message10":   markers.NewMarkers(markers.NewMarker(2, 4)),
			"Message11":   markers.NewMarkers(markers.NewMarker(5, 5)),
			"Message11.5": markers.NewMarkers(markers.NewMarker(2, 3), markers.NewMarker(3, 1)),
			"Message12":   markers.NewMarkers(markers.NewMarker(6, 2)),
			"Message13":   markers.NewMarkers(markers.NewMarker(7, 4)),
			"Message14":   markers.NewMarkers(markers.NewMarker(2, 4)),
			"Message15":   markers.NewMarkers(markers.NewMarker(2, 5)),
			"Message16":   markers.NewMarkers(markers.NewMarker(8, 1)),
			"Message17":   markers.NewMarkers(markers.NewMarker(2, 4)),
			"Message18":   markers.NewMarkers(markers.NewMarker(7, 5)),
			"Message19":   markers.NewMarkers(markers.NewMarker(9, 4)),
		})
		checkMessageMetadataBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1":    ledgerstate.UndefinedBranchID,
			"Message2":    ledgerstate.UndefinedBranchID,
			"Message3":    ledgerstate.UndefinedBranchID,
			"Message4":    ledgerstate.UndefinedBranchID,
			"Message5":    ledgerstate.UndefinedBranchID,
			"Message6":    ledgerstate.UndefinedBranchID,
			"Message7":    ledgerstate.UndefinedBranchID,
			"Message8":    ledgerstate.UndefinedBranchID,
			"Message9":    testFramework.BranchID("A+D+F+H"),
			"Message10":   ledgerstate.UndefinedBranchID,
			"Message11":   ledgerstate.UndefinedBranchID,
			"Message11.5": testFramework.BranchID("A+D+F+H"),
			"Message12":   ledgerstate.UndefinedBranchID,
			"Message13":   ledgerstate.UndefinedBranchID,
			"Message14":   testFramework.BranchID("B+E+F"),
			"Message15":   ledgerstate.UndefinedBranchID,
			"Message16":   ledgerstate.UndefinedBranchID,
			"Message17":   testFramework.BranchID("B+E+F"),
			"Message18":   ledgerstate.UndefinedBranchID,
			"Message19":   ledgerstate.UndefinedBranchID,
		})
		checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1":    testFramework.BranchID("A"),
			"Message2":    testFramework.BranchID("B"),
			"Message3":    testFramework.BranchID("C"),
			"Message4":    testFramework.BranchID("F"),
			"Message5":    testFramework.BranchID("D+F"),
			"Message6":    testFramework.BranchID("B"),
			"Message7":    testFramework.BranchID("B+D+F"),
			"Message8":    testFramework.BranchID("A+C+D+F"),
			"Message9":    testFramework.BranchID("A+D+F+H"),
			"Message10":   testFramework.BranchID("B+D+F"),
			"Message11":   testFramework.BranchID("A+D+F+H"),
			"Message11.5": testFramework.BranchID("A+D+F+H"),
			"Message12":   testFramework.BranchID("A+E"),
			"Message13":   testFramework.BranchID("B+E+F"),
			"Message14":   testFramework.BranchID("B+E+F"),
			"Message15":   testFramework.BranchID("B+D+F"),
			"Message16":   testFramework.BranchID("G"),
			"Message17":   testFramework.BranchID("B+E+F"),
			"Message18":   testFramework.BranchID("B+E+F"),
			"Message19":   testFramework.BranchID("D+F+I"),
		})
	}

	// ISSUE Message33
	{
		msg := testFramework.CreateMessage("Message33", WithStrongParents("Message15", "Message11"))
		testFramework.IssueMessages("Message33").WaitMessagesBooked()
		tangle.Storage.MessageMetadata(msg.ID()).Consume(func(messageMetadata *MessageMetadata) {
			assert.True(t, messageMetadata.invalid)
		})

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1":    markers.NewMarkers(markers.NewMarker(1, 1)),
			"Message2":    markers.NewMarkers(markers.NewMarker(2, 1)),
			"Message3":    markers.NewMarkers(markers.NewMarker(3, 1)),
			"Message4":    markers.NewMarkers(markers.NewMarker(4, 1)),
			"Message5":    markers.NewMarkers(markers.NewMarker(4, 2)),
			"Message6":    markers.NewMarkers(markers.NewMarker(2, 2)),
			"Message7":    markers.NewMarkers(markers.NewMarker(2, 3)),
			"Message8":    markers.NewMarkers(markers.NewMarker(5, 4)),
			"Message9":    markers.NewMarkers(markers.NewMarker(2, 3), markers.NewMarker(3, 1)),
			"Message10":   markers.NewMarkers(markers.NewMarker(2, 4)),
			"Message11":   markers.NewMarkers(markers.NewMarker(5, 5)),
			"Message11.5": markers.NewMarkers(markers.NewMarker(2, 3), markers.NewMarker(3, 1)),
			"Message12":   markers.NewMarkers(markers.NewMarker(6, 2)),
			"Message13":   markers.NewMarkers(markers.NewMarker(7, 4)),
			"Message14":   markers.NewMarkers(markers.NewMarker(2, 4)),
			"Message15":   markers.NewMarkers(markers.NewMarker(2, 5)),
			"Message16":   markers.NewMarkers(markers.NewMarker(8, 1)),
			"Message17":   markers.NewMarkers(markers.NewMarker(2, 4)),
			"Message18":   markers.NewMarkers(markers.NewMarker(7, 5)),
			"Message19":   markers.NewMarkers(markers.NewMarker(9, 4)),
		})
		checkMessageMetadataBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1":    ledgerstate.UndefinedBranchID,
			"Message2":    ledgerstate.UndefinedBranchID,
			"Message3":    ledgerstate.UndefinedBranchID,
			"Message4":    ledgerstate.UndefinedBranchID,
			"Message5":    ledgerstate.UndefinedBranchID,
			"Message6":    ledgerstate.UndefinedBranchID,
			"Message7":    ledgerstate.UndefinedBranchID,
			"Message8":    ledgerstate.UndefinedBranchID,
			"Message9":    testFramework.BranchID("A+D+F+H"),
			"Message10":   ledgerstate.UndefinedBranchID,
			"Message11":   ledgerstate.UndefinedBranchID,
			"Message11.5": testFramework.BranchID("A+D+F+H"),
			"Message12":   ledgerstate.UndefinedBranchID,
			"Message13":   ledgerstate.UndefinedBranchID,
			"Message14":   testFramework.BranchID("B+E+F"),
			"Message15":   ledgerstate.UndefinedBranchID,
			"Message16":   ledgerstate.UndefinedBranchID,
			"Message17":   testFramework.BranchID("B+E+F"),
			"Message18":   ledgerstate.UndefinedBranchID,
			"Message19":   ledgerstate.UndefinedBranchID,
		})
		checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1":    testFramework.BranchID("A"),
			"Message2":    testFramework.BranchID("B"),
			"Message3":    testFramework.BranchID("C"),
			"Message4":    testFramework.BranchID("F"),
			"Message5":    testFramework.BranchID("D+F"),
			"Message6":    testFramework.BranchID("B"),
			"Message7":    testFramework.BranchID("B+D+F"),
			"Message8":    testFramework.BranchID("A+C+D+F"),
			"Message9":    testFramework.BranchID("A+D+F+H"),
			"Message10":   testFramework.BranchID("B+D+F"),
			"Message11":   testFramework.BranchID("A+D+F+H"),
			"Message11.5": testFramework.BranchID("A+D+F+H"),
			"Message12":   testFramework.BranchID("A+E"),
			"Message13":   testFramework.BranchID("B+E+F"),
			"Message14":   testFramework.BranchID("B+E+F"),
			"Message15":   testFramework.BranchID("B+D+F"),
			"Message16":   testFramework.BranchID("G"),
			"Message17":   testFramework.BranchID("B+E+F"),
			"Message18":   testFramework.BranchID("B+E+F"),
			"Message19":   testFramework.BranchID("D+F+I"),
		})
	}

	// ISSUE Message34
	{
		msg := testFramework.CreateMessage("Message34", WithStrongParents("Message14", "Message9"))
		testFramework.IssueMessages("Message34").WaitMessagesBooked()
		tangle.Storage.MessageMetadata(msg.ID()).Consume(func(messageMetadata *MessageMetadata) {
			assert.True(t, messageMetadata.invalid)
		})

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1":    markers.NewMarkers(markers.NewMarker(1, 1)),
			"Message2":    markers.NewMarkers(markers.NewMarker(2, 1)),
			"Message3":    markers.NewMarkers(markers.NewMarker(3, 1)),
			"Message4":    markers.NewMarkers(markers.NewMarker(4, 1)),
			"Message5":    markers.NewMarkers(markers.NewMarker(4, 2)),
			"Message6":    markers.NewMarkers(markers.NewMarker(2, 2)),
			"Message7":    markers.NewMarkers(markers.NewMarker(2, 3)),
			"Message8":    markers.NewMarkers(markers.NewMarker(5, 4)),
			"Message9":    markers.NewMarkers(markers.NewMarker(2, 3), markers.NewMarker(3, 1)),
			"Message10":   markers.NewMarkers(markers.NewMarker(2, 4)),
			"Message11":   markers.NewMarkers(markers.NewMarker(5, 5)),
			"Message11.5": markers.NewMarkers(markers.NewMarker(2, 3), markers.NewMarker(3, 1)),
			"Message12":   markers.NewMarkers(markers.NewMarker(6, 2)),
			"Message13":   markers.NewMarkers(markers.NewMarker(7, 4)),
			"Message14":   markers.NewMarkers(markers.NewMarker(2, 4)),
			"Message15":   markers.NewMarkers(markers.NewMarker(2, 5)),
			"Message16":   markers.NewMarkers(markers.NewMarker(8, 1)),
			"Message17":   markers.NewMarkers(markers.NewMarker(2, 4)),
			"Message18":   markers.NewMarkers(markers.NewMarker(7, 5)),
			"Message19":   markers.NewMarkers(markers.NewMarker(9, 4)),
		})
		checkMessageMetadataBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1":    ledgerstate.UndefinedBranchID,
			"Message2":    ledgerstate.UndefinedBranchID,
			"Message3":    ledgerstate.UndefinedBranchID,
			"Message4":    ledgerstate.UndefinedBranchID,
			"Message5":    ledgerstate.UndefinedBranchID,
			"Message6":    ledgerstate.UndefinedBranchID,
			"Message7":    ledgerstate.UndefinedBranchID,
			"Message8":    ledgerstate.UndefinedBranchID,
			"Message9":    testFramework.BranchID("A+D+F+H"),
			"Message10":   ledgerstate.UndefinedBranchID,
			"Message11":   ledgerstate.UndefinedBranchID,
			"Message11.5": testFramework.BranchID("A+D+F+H"),
			"Message12":   ledgerstate.UndefinedBranchID,
			"Message13":   ledgerstate.UndefinedBranchID,
			"Message14":   testFramework.BranchID("B+E+F"),
			"Message15":   ledgerstate.UndefinedBranchID,
			"Message16":   ledgerstate.UndefinedBranchID,
			"Message17":   testFramework.BranchID("B+E+F"),
			"Message18":   ledgerstate.UndefinedBranchID,
			"Message19":   ledgerstate.UndefinedBranchID,
		})
		checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1":    testFramework.BranchID("A"),
			"Message2":    testFramework.BranchID("B"),
			"Message3":    testFramework.BranchID("C"),
			"Message4":    testFramework.BranchID("F"),
			"Message5":    testFramework.BranchID("D+F"),
			"Message6":    testFramework.BranchID("B"),
			"Message7":    testFramework.BranchID("B+D+F"),
			"Message8":    testFramework.BranchID("A+C+D+F"),
			"Message9":    testFramework.BranchID("A+D+F+H"),
			"Message10":   testFramework.BranchID("B+D+F"),
			"Message11":   testFramework.BranchID("A+D+F+H"),
			"Message11.5": testFramework.BranchID("A+D+F+H"),
			"Message12":   testFramework.BranchID("A+E"),
			"Message13":   testFramework.BranchID("B+E+F"),
			"Message14":   testFramework.BranchID("B+E+F"),
			"Message15":   testFramework.BranchID("B+D+F"),
			"Message16":   testFramework.BranchID("G"),
			"Message17":   testFramework.BranchID("B+E+F"),
			"Message18":   testFramework.BranchID("B+E+F"),
			"Message19":   testFramework.BranchID("D+F+I"),
		})
	}

	// ISSUE Message35
	{
		testFramework.CreateMessage("Message35", WithStrongParents("Message15", "Message16"))
		testFramework.IssueMessages("Message35").WaitMessagesBooked()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1":    markers.NewMarkers(markers.NewMarker(1, 1)),
			"Message2":    markers.NewMarkers(markers.NewMarker(2, 1)),
			"Message3":    markers.NewMarkers(markers.NewMarker(3, 1)),
			"Message4":    markers.NewMarkers(markers.NewMarker(4, 1)),
			"Message5":    markers.NewMarkers(markers.NewMarker(4, 2)),
			"Message6":    markers.NewMarkers(markers.NewMarker(2, 2)),
			"Message7":    markers.NewMarkers(markers.NewMarker(2, 3)),
			"Message8":    markers.NewMarkers(markers.NewMarker(5, 4)),
			"Message9":    markers.NewMarkers(markers.NewMarker(2, 3), markers.NewMarker(3, 1)),
			"Message10":   markers.NewMarkers(markers.NewMarker(2, 4)),
			"Message11":   markers.NewMarkers(markers.NewMarker(5, 5)),
			"Message11.5": markers.NewMarkers(markers.NewMarker(2, 3), markers.NewMarker(3, 1)),
			"Message12":   markers.NewMarkers(markers.NewMarker(6, 2)),
			"Message13":   markers.NewMarkers(markers.NewMarker(7, 4)),
			"Message14":   markers.NewMarkers(markers.NewMarker(2, 4)),
			"Message15":   markers.NewMarkers(markers.NewMarker(2, 5)),
			"Message16":   markers.NewMarkers(markers.NewMarker(8, 1)),
			"Message17":   markers.NewMarkers(markers.NewMarker(2, 4)),
			"Message18":   markers.NewMarkers(markers.NewMarker(7, 5)),
			"Message19":   markers.NewMarkers(markers.NewMarker(9, 4)),
		})
		checkMessageMetadataBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1":    ledgerstate.UndefinedBranchID,
			"Message2":    ledgerstate.UndefinedBranchID,
			"Message3":    ledgerstate.UndefinedBranchID,
			"Message4":    ledgerstate.UndefinedBranchID,
			"Message5":    ledgerstate.UndefinedBranchID,
			"Message6":    ledgerstate.UndefinedBranchID,
			"Message7":    ledgerstate.UndefinedBranchID,
			"Message8":    ledgerstate.UndefinedBranchID,
			"Message9":    testFramework.BranchID("A+D+F+H"),
			"Message10":   ledgerstate.UndefinedBranchID,
			"Message11":   ledgerstate.UndefinedBranchID,
			"Message11.5": testFramework.BranchID("A+D+F+H"),
			"Message12":   ledgerstate.UndefinedBranchID,
			"Message13":   ledgerstate.UndefinedBranchID,
			"Message14":   testFramework.BranchID("B+E+F"),
			"Message15":   ledgerstate.UndefinedBranchID,
			"Message16":   ledgerstate.UndefinedBranchID,
			"Message17":   testFramework.BranchID("B+E+F"),
			"Message18":   ledgerstate.UndefinedBranchID,
			"Message19":   ledgerstate.UndefinedBranchID,
		})
		checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1":    testFramework.BranchID("A"),
			"Message2":    testFramework.BranchID("B"),
			"Message3":    testFramework.BranchID("C"),
			"Message4":    testFramework.BranchID("F"),
			"Message5":    testFramework.BranchID("D+F"),
			"Message6":    testFramework.BranchID("B"),
			"Message7":    testFramework.BranchID("B+D+F"),
			"Message8":    testFramework.BranchID("A+C+D+F"),
			"Message9":    testFramework.BranchID("A+D+F+H"),
			"Message10":   testFramework.BranchID("B+D+F"),
			"Message11":   testFramework.BranchID("A+D+F+H"),
			"Message11.5": testFramework.BranchID("A+D+F+H"),
			"Message12":   testFramework.BranchID("A+E"),
			"Message13":   testFramework.BranchID("B+E+F"),
			"Message14":   testFramework.BranchID("B+E+F"),
			"Message15":   testFramework.BranchID("B+D+F"),
			"Message16":   testFramework.BranchID("G"),
			"Message17":   testFramework.BranchID("B+E+F"),
			"Message18":   testFramework.BranchID("B+E+F"),
			"Message19":   testFramework.BranchID("D+F+I"),
		})
	}
}

func checkIndividuallyMappedMessages(t *testing.T, testFramework *MessageTestFramework, expectedIndividuallyMappedMessages map[ledgerstate.BranchID][]string) {
	expectedMappings := 0

	for branchID, expectedMessageAliases := range expectedIndividuallyMappedMessages {
		for _, alias := range expectedMessageAliases {
			expectedMappings++
			assert.True(t, testFramework.tangle.Storage.IndividuallyMappedMessage(branchID, testFramework.Message(alias).ID()).Consume(func(individuallyMappedMessage *IndividuallyMappedMessage) {}))
		}
	}

	// check that there's only exactly as many individually mapped messages as expected (ie old stuff gets cleaned up)
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
