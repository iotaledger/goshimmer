package tangle

import (
	"sync"
	"testing"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/markers"
	"github.com/iotaledger/hive.go/events"
	"github.com/stretchr/testify/assert"
)

func TestUtils_AllTransactionsApprovedByMessages(t *testing.T) {
	// imgages/util-AllTransactionsApprovedByMessages-parallel-markers.png

	tangle := New()
	tangle.Setup()
	defer tangle.Shutdown()

	tangle.Events.Error.Attach(events.NewClosure(func(err error) {
		panic(err)
	}))

	wallets := make(map[string]wallet)
	walletsByAddress := make(map[ledgerstate.Address]wallet)
	transactions := make(map[string]*ledgerstate.Transaction)
	inputs := make(map[string]*ledgerstate.UTXOInput)
	outputs := make(map[string]*ledgerstate.SigLockedSingleOutput)
	outputsByID := make(map[ledgerstate.OutputID]ledgerstate.Output)
	messages := make(map[string]*Message)

	// Setup boilerplate
	{
		walletAliases := []string{"G1", "G2", "A", "B", "C", "D"}

		for i, wallet := range createWallets(len(walletAliases)) {
			wallets[walletAliases[i]] = wallet
		}

		for _, wallet := range wallets {
			walletsByAddress[wallet.address] = wallet
		}

		g1Balance := ledgerstate.NewColoredBalances(map[ledgerstate.Color]uint64{
			ledgerstate.ColorIOTA: 5,
		})
		g2Balance := ledgerstate.NewColoredBalances(map[ledgerstate.Color]uint64{
			ledgerstate.ColorIOTA: 8,
		})

		tangle.LedgerState.LoadSnapshot(map[ledgerstate.TransactionID]map[ledgerstate.Address]*ledgerstate.ColoredBalances{
			ledgerstate.GenesisTransactionID: {
				wallets["G1"].address: g1Balance,
				wallets["G2"].address: g2Balance,
			},
		})

		// determine genesis index so that correct output can be referenced
		var g1, g2 uint16
		tangle.LedgerState.utxoDAG.Output(ledgerstate.NewOutputID(ledgerstate.GenesisTransactionID, 0)).Consume(func(output ledgerstate.Output) {
			balance, _ := output.Balances().Get(ledgerstate.ColorIOTA)
			if balance == uint64(5) {
				g1 = 0
				g2 = 1
			} else {
				g1 = 1
				g2 = 0
			}
		})

		inputs["G1"] = ledgerstate.NewUTXOInput(ledgerstate.NewOutputID(ledgerstate.GenesisTransactionID, g1))
		inputs["G2"] = ledgerstate.NewUTXOInput(ledgerstate.NewOutputID(ledgerstate.GenesisTransactionID, g2))
	}

	messages["Message1"] = newTestParentsDataMessage("Message1", []MessageID{EmptyMessageID}, nil)

	outputs["A"] = ledgerstate.NewSigLockedSingleOutput(5, wallets["A"].address)
	transactions["TX1"] = makeTransaction(ledgerstate.NewInputs(inputs["G1"]), ledgerstate.NewOutputs(outputs["A"]), outputsByID, walletsByAddress, wallets["G1"])
	messages["Message2"] = newTestParentsPayloadMessage(transactions["TX1"], []MessageID{messages["Message1"].ID()}, nil)

	messages["Message3"] = newTestParentsDataMessage("Message3", []MessageID{messages["Message2"].ID()}, nil)

	messages["Message4"] = newTestParentsDataMessage("Message4", []MessageID{EmptyMessageID}, nil)

	outputs["B"] = ledgerstate.NewSigLockedSingleOutput(4, wallets["B"].address)
	outputs["C"] = ledgerstate.NewSigLockedSingleOutput(4, wallets["C"].address)
	transactions["TX2"] = makeTransaction(ledgerstate.NewInputs(inputs["G2"]), ledgerstate.NewOutputs(outputs["B"], outputs["C"]), outputsByID, walletsByAddress, wallets["G2"])
	messages["Message5"] = newTestParentsPayloadMessage(transactions["TX2"], []MessageID{messages["Message4"].ID()}, nil)

	messages["Message6"] = newTestParentsDataMessage("Message6", []MessageID{messages["Message5"].ID()}, nil)

	inputs["A"] = ledgerstate.NewUTXOInput(ledgerstate.NewOutputID(transactions["TX1"].ID(), selectIndex(transactions["TX1"], wallets["A"])))
	inputs["B"] = ledgerstate.NewUTXOInput(ledgerstate.NewOutputID(transactions["TX2"].ID(), selectIndex(transactions["TX2"], wallets["B"])))
	inputs["C"] = ledgerstate.NewUTXOInput(ledgerstate.NewOutputID(transactions["TX2"].ID(), selectIndex(transactions["TX2"], wallets["C"])))
	outputs["D"] = ledgerstate.NewSigLockedSingleOutput(13, wallets["D"].address)
	outputsByID[inputs["A"].ReferencedOutputID()] = outputs["A"]
	outputsByID[inputs["B"].ReferencedOutputID()] = outputs["B"]
	outputsByID[inputs["C"].ReferencedOutputID()] = outputs["C"]
	transactions["TX3"] = makeTransaction(ledgerstate.NewInputs(inputs["A"], inputs["B"], inputs["C"]), ledgerstate.NewOutputs(outputs["D"]), outputsByID, walletsByAddress)
	messages["Message7"] = newTestParentsPayloadMessage(transactions["TX3"], []MessageID{messages["Message3"].ID(), messages["Message6"].ID()}, nil)

	var waitGroup sync.WaitGroup
	tangle.Booker.Events.MessageBooked.Attach(events.NewClosure(func(messageID MessageID) {
		waitGroup.Done()
	}))

	waitGroup.Add(len(messages))
	tangle.Storage.StoreMessage(messages["Message1"])
	tangle.Storage.StoreMessage(messages["Message2"])
	tangle.Storage.StoreMessage(messages["Message3"])
	tangle.Storage.StoreMessage(messages["Message4"])
	tangle.Storage.StoreMessage(messages["Message5"])
	tangle.Storage.StoreMessage(messages["Message6"])
	tangle.Storage.StoreMessage(messages["Message7"])

	waitGroup.Wait()

	for messageName, expectedMarkers := range map[string]*markers.Markers{
		"Message1": markers.NewMarkers(markers.NewMarker(1, 1)),
		"Message2": markers.NewMarkers(markers.NewMarker(1, 2)),
		"Message3": markers.NewMarkers(markers.NewMarker(1, 3)),
		"Message4": markers.NewMarkers(markers.NewMarker(0, 0)),
		"Message5": markers.NewMarkers(markers.NewMarker(0, 0)),
		"Message6": markers.NewMarkers(markers.NewMarker(0, 0)),
		"Message7": markers.NewMarkers(markers.NewMarker(1, 4)),
	} {
		tangle.Storage.MessageMetadata(messages[messageName].ID()).Consume(func(messageMetadata *MessageMetadata) {
			assert.True(t, messageMetadata.StructureDetails().PastMarkers.Equals(expectedMarkers))
		})
	}
}
