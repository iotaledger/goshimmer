package tangle

import (
	"sync"
	"testing"
	"time"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/vote"
	"github.com/iotaledger/goshimmer/packages/vote/opinion"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/identity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpinionFormer_Scenario2(t *testing.T) {
	LikedThreshold = 2 * time.Second
	LocallyFinalizedThreshold = 2 * time.Second

	tangle := New()
	defer tangle.Shutdown()
	tangle.Setup()

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
	inputs := make(map[string]*ledgerstate.UTXOInput)
	outputs := make(map[string]*ledgerstate.SigLockedSingleOutput)
	outputsByID := make(map[ledgerstate.OutputID]ledgerstate.Output)

	// Message 1
	inputs["GENESIS"] = ledgerstate.NewUTXOInput(ledgerstate.NewOutputID(ledgerstate.GenesisTransactionID, 0))
	outputs["A"] = ledgerstate.NewSigLockedSingleOutput(1, wallets["A"].address)
	outputs["B"] = ledgerstate.NewSigLockedSingleOutput(1, wallets["B"].address)
	outputs["C"] = ledgerstate.NewSigLockedSingleOutput(1, wallets["C"].address)
	transactions["1"] = makeTransaction(ledgerstate.NewInputs(inputs["GENESIS"]), ledgerstate.NewOutputs(outputs["A"], outputs["B"], outputs["C"]), outputsByID, walletsByAddress, wallets["GENESIS"])
	messages["1"] = newTestParentsPayloadMessage(transactions["1"], []MessageID{EmptyMessageID}, []MessageID{})

	// Message 2
	inputs["B"] = ledgerstate.NewUTXOInput(ledgerstate.NewOutputID(transactions["1"].ID(), selectIndex(transactions["1"], wallets["B"])))
	outputsByID[inputs["B"].ReferencedOutputID()] = ledgerstate.NewOutputs(outputs["B"])[0]
	inputs["C"] = ledgerstate.NewUTXOInput(ledgerstate.NewOutputID(transactions["1"].ID(), selectIndex(transactions["1"], wallets["C"])))
	outputsByID[inputs["C"].ReferencedOutputID()] = ledgerstate.NewOutputs(outputs["C"])[0]
	outputs["E"] = ledgerstate.NewSigLockedSingleOutput(2, wallets["E"].address)
	transactions["2"] = makeTransaction(ledgerstate.NewInputs(inputs["B"], inputs["C"]), ledgerstate.NewOutputs(outputs["E"]), outputsByID, walletsByAddress)
	messages["2"] = newTestParentsPayloadMessage(transactions["2"], []MessageID{EmptyMessageID, messages["1"].ID()}, []MessageID{})

	// Message 3 (Reattachemnt of transaction 2)
	messages["3"] = newTestParentsPayloadMessage(transactions["2"], []MessageID{messages["1"].ID(), messages["2"].ID()}, []MessageID{})

	// Message 4
	inputs["A"] = ledgerstate.NewUTXOInput(ledgerstate.NewOutputID(transactions["1"].ID(), selectIndex(transactions["1"], wallets["A"])))
	outputsByID[inputs["A"].ReferencedOutputID()] = ledgerstate.NewOutputs(outputs["A"])[0]
	outputs["D"] = ledgerstate.NewSigLockedSingleOutput(1, wallets["D"].address)
	transactions["3"] = makeTransaction(ledgerstate.NewInputs(inputs["A"]), ledgerstate.NewOutputs(outputs["D"]), outputsByID, walletsByAddress)
	messages["4"] = newTestParentsPayloadMessage(transactions["3"], []MessageID{EmptyMessageID, messages["1"].ID()}, []MessageID{})

	// Message 5
	outputs["F"] = ledgerstate.NewSigLockedSingleOutput(1, wallets["F"].address)
	transactions["4"] = makeTransaction(ledgerstate.NewInputs(inputs["A"]), ledgerstate.NewOutputs(outputs["F"]), outputsByID, walletsByAddress)
	messages["5"] = newTestParentsPayloadMessage(transactions["4"], []MessageID{messages["1"].ID()}, []MessageID{messages["2"].ID()})

	// Message 6
	inputs["E"] = ledgerstate.NewUTXOInput(ledgerstate.NewOutputID(transactions["2"].ID(), 0))
	outputsByID[inputs["E"].ReferencedOutputID()] = ledgerstate.NewOutputs(outputs["E"])[0]
	inputs["F"] = ledgerstate.NewUTXOInput(ledgerstate.NewOutputID(transactions["4"].ID(), 0))
	outputsByID[inputs["F"].ReferencedOutputID()] = ledgerstate.NewOutputs(outputs["F"])[0]
	outputs["L"] = ledgerstate.NewSigLockedSingleOutput(3, wallets["L"].address)
	transactions["5"] = makeTransaction(ledgerstate.NewInputs(inputs["E"], inputs["F"]), ledgerstate.NewOutputs(outputs["L"]), outputsByID, walletsByAddress)
	messages["6"] = newTestParentsPayloadMessage(transactions["5"], []MessageID{messages["2"].ID(), messages["5"].ID()}, []MessageID{})

	// Message 7
	outputs["H"] = ledgerstate.NewSigLockedSingleOutput(1, wallets["H"].address)
	transactions["6"] = makeTransaction(ledgerstate.NewInputs(inputs["C"]), ledgerstate.NewOutputs(outputs["H"]), outputsByID, walletsByAddress)
	messages["7"] = newTestParentsPayloadMessage(transactions["6"], []MessageID{messages["1"].ID(), messages["4"].ID()}, []MessageID{})

	// Message 8
	inputs["H"] = ledgerstate.NewUTXOInput(ledgerstate.NewOutputID(transactions["6"].ID(), 0))
	outputsByID[inputs["H"].ReferencedOutputID()] = ledgerstate.NewOutputs(outputs["H"])[0]
	inputs["D"] = ledgerstate.NewUTXOInput(ledgerstate.NewOutputID(transactions["3"].ID(), 0))
	outputsByID[inputs["D"].ReferencedOutputID()] = ledgerstate.NewOutputs(outputs["D"])[0]
	outputs["I"] = ledgerstate.NewSigLockedSingleOutput(2, wallets["I"].address)
	transactions["7"] = makeTransaction(ledgerstate.NewInputs(inputs["D"], inputs["H"]), ledgerstate.NewOutputs(outputs["I"]), outputsByID, walletsByAddress)
	messages["8"] = newTestParentsPayloadMessage(transactions["7"], []MessageID{messages["4"].ID(), messages["7"].ID()}, []MessageID{})

	// Message 9
	outputs["J"] = ledgerstate.NewSigLockedSingleOutput(1, wallets["J"].address)
	transactions["8"] = makeTransaction(ledgerstate.NewInputs(inputs["B"]), ledgerstate.NewOutputs(outputs["J"]), outputsByID, walletsByAddress)
	messages["9"] = newTestParentsPayloadMessage(transactions["8"], []MessageID{messages["4"].ID(), messages["7"].ID()}, []MessageID{})

	// setup mock voter

	transactionLiked := make(map[ledgerstate.TransactionID]bool)
	transactionLiked[transactions["1"].ID()] = true
	transactionLiked[transactions["2"].ID()] = false
	transactionLiked[transactions["3"].ID()] = true
	transactionLiked[transactions["4"].ID()] = false
	transactionLiked[transactions["6"].ID()] = true
	transactionLiked[transactions["8"].ID()] = false

	tangle.OpinionFormer.payloadOpinionProvider.Vote().Attach(events.NewClosure(func(transactionID string, initialOpinion opinion.Opinion) {
		t.Log("Voting requested for:", transactionID)
		txID, err := ledgerstate.TransactionIDFromBase58(transactionID)
		require.NoError(t, err)
		o := opinion.Dislike
		if transactionLiked[txID] {
			o = opinion.Like
		}
		tangle.OpinionFormer.payloadOpinionProvider.ProcessVote(&vote.OpinionEvent{
			ID:      transactionID,
			Opinion: o,
			Ctx:     vote.Context{Type: vote.ConflictType}})
	}))

	tangle.OpinionFormer.payloadOpinionProvider.VoteError().Attach(events.NewClosure(func(err error) {
		t.Log("VoteError", err)
	}))

	payloadLiked := make(map[MessageID]bool)
	payloadLiked[messages["1"].ID()] = true
	payloadLiked[messages["2"].ID()] = false
	payloadLiked[messages["3"].ID()] = false
	payloadLiked[messages["4"].ID()] = true
	payloadLiked[messages["5"].ID()] = false
	payloadLiked[messages["6"].ID()] = false
	payloadLiked[messages["7"].ID()] = true
	payloadLiked[messages["8"].ID()] = true
	payloadLiked[messages["9"].ID()] = false

	tangle.Solidifier.Events.MessageSolid.Attach(events.NewClosure(func(messageID MessageID) {
		t.Log("Message solid:", messageID)
	}))

	tangle.Scheduler.Events.MessageScheduled.Attach(events.NewClosure(func(messageID MessageID) {
		t.Log("Message scheduled:", messageID)
	}))

	tangle.Booker.Events.MessageBooked.Attach(events.NewClosure(func(messageID MessageID) {
		t.Log("Message Booked:", messageID)
	}))

	var wg sync.WaitGroup
	tangle.OpinionFormer.Events.MessageOpinionFormed.Attach(events.NewClosure(func(messageID MessageID) {
		t.Logf("MessageOpinionFormed for %s", messageID)

		assert.True(t, tangle.OpinionFormer.MessageEligible(messageID))
		assert.Equal(t, payloadLiked[messageID], tangle.OpinionFormer.PayloadLiked(messageID))
		t.Log("Payload Liked:", tangle.OpinionFormer.PayloadLiked(messageID))
		wg.Done()
	}))

	wg.Add(9)

	tangle.Storage.StoreMessage(messages["1"])
	tangle.Storage.StoreMessage(messages["2"])
	tangle.Storage.StoreMessage(messages["3"])
	tangle.Storage.StoreMessage(messages["4"])
	tangle.Storage.StoreMessage(messages["5"])
	tangle.Storage.StoreMessage(messages["6"])
	tangle.Storage.StoreMessage(messages["7"])
	tangle.Storage.StoreMessage(messages["8"])
	tangle.Storage.StoreMessage(messages["9"])

	wg.Wait()
}

func TestOpinionFormer(t *testing.T) {
	LikedThreshold = 1 * time.Second
	LocallyFinalizedThreshold = 2 * time.Second

	tangle := New()
	defer tangle.Shutdown()

	messageA := newTestDataMessage("A")

	wallets := createWallets(2)

	snapshot := map[ledgerstate.TransactionID]map[ledgerstate.Address]*ledgerstate.ColoredBalances{
		ledgerstate.GenesisTransactionID: {
			wallets[0].address: ledgerstate.NewColoredBalances(
				map[ledgerstate.Color]uint64{
					ledgerstate.ColorIOTA: 10000,
				})},
	}

	tangle.LedgerState.LoadSnapshot(snapshot)

	input := ledgerstate.NewUTXOInput(ledgerstate.NewOutputID(ledgerstate.GenesisTransactionID, 0))
	output := ledgerstate.NewSigLockedSingleOutput(10000, wallets[0].address)
	txEssence := ledgerstate.NewTransactionEssence(0, time.Now(), identity.ID{}, identity.ID{}, ledgerstate.NewInputs(input), ledgerstate.NewOutputs(output))
	tx1 := ledgerstate.NewTransaction(txEssence, wallets[0].unlockBlocks(txEssence))
	t.Log("Transacion1: ", tx1.ID())
	messageB := newTestParentsPayloadMessage(tx1, []MessageID{messageA.ID()}, []MessageID{})

	output2 := ledgerstate.NewSigLockedSingleOutput(10000, wallets[1].address)
	txEssence2 := ledgerstate.NewTransactionEssence(0, time.Now(), identity.ID{}, identity.ID{}, ledgerstate.NewInputs(input), ledgerstate.NewOutputs(output2))
	tx2 := ledgerstate.NewTransaction(txEssence2, wallets[0].unlockBlocks(txEssence2))
	t.Log("Transacion2: ", tx2.ID())
	messageC := newTestParentsPayloadMessage(tx2, []MessageID{messageA.ID()}, []MessageID{})

	payloadLiked := make(map[MessageID]bool)
	payloadLiked[messageA.ID()] = true
	payloadLiked[messageB.ID()] = false
	payloadLiked[messageC.ID()] = false

	tangle.Setup()

	tangle.Booker.Events.MessageBooked.Attach(events.NewClosure(func(messageID MessageID) {
		t.Log("Message Booked:", messageID)
	}))

	tangle.Events.MessageInvalid.Attach(events.NewClosure(func(messageID MessageID) {
		t.Log("Invalid message:", messageID)
	}))

	var wg sync.WaitGroup

	tangle.OpinionFormer.Events.MessageOpinionFormed.Attach(events.NewClosure(func(messageID MessageID) {
		t.Log("MessageOpinionFormed for ", messageID)

		assert.True(t, tangle.OpinionFormer.MessageEligible(messageID))
		assert.Equal(t, payloadLiked[messageID], tangle.OpinionFormer.PayloadLiked(messageID))
		t.Log("Payload Liked:", tangle.OpinionFormer.PayloadLiked(messageID))
		wg.Done()
	}))

	tangle.OpinionFormer.payloadOpinionProvider.Vote().Attach(events.NewClosure(func(transactionID string, initialOpinion opinion.Opinion) {
		t.Log("Voting requested for:", transactionID)
		tangle.OpinionFormer.payloadOpinionProvider.ProcessVote(&vote.OpinionEvent{
			ID:      transactionID,
			Opinion: opinion.Dislike,
			Ctx:     vote.Context{Type: vote.ConflictType}})
	}))

	tangle.OpinionFormer.payloadOpinionProvider.VoteError().Attach(events.NewClosure(func(err error) {
		t.Log("VoteError", err)
	}))

	wg.Add(3)

	tangle.Storage.StoreMessage(messageA)
	tangle.Storage.StoreMessage(messageB)
	tangle.Storage.StoreMessage(messageC)

	wg.Wait()

	t.Log("Waiting shutdown..")
}
