package tangle

import (
	"context"
	"crypto/ed25519"
	"fmt"
	"github.com/iotaledger/goshimmer/packages/consensus"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/identity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "golang.org/x/crypto/blake2b"

	"github.com/iotaledger/goshimmer/packages/clock"
	"github.com/iotaledger/goshimmer/packages/pow"
	"github.com/iotaledger/goshimmer/packages/tangle/payload"
)

const (
	targetPOW     = 10
	powTimeout    = 10 * time.Second
	totalMessages = 2000
)

func TestMessageFactory_BuildMessage(t *testing.T) {
	selfLocalIdentity := identity.GenerateLocalIdentity()
	tangle := NewTestTangle(Identity(selfLocalIdentity))
	defer tangle.Shutdown()
	mockOTV := &SimpleMockOnTangleVoting{}
	tangle.OTVConsensusManager = NewOTVConsensusManager(mockOTV)

	tangle.MessageFactory = NewMessageFactory(
		tangle,
		TipSelectorFunc(func(p payload.Payload, countParents int) (parents MessageIDs, err error) {
			return []MessageID{EmptyMessageID}, nil
		}),
		func(parents MessageIDs, tangle *Tangle) (MessageIDs, error) {
			return []MessageID{}, nil
		},
	)
	tangle.MessageFactory.SetTimeout(powTimeout)
	defer tangle.MessageFactory.Shutdown()

	// keep track of sequence numbers
	sequenceNumbers := sync.Map{}

	// attach to event and count
	countEvents := uint64(0)
	tangle.MessageFactory.Events.MessageConstructed.Attach(events.NewClosure(func(msg *Message) {
		atomic.AddUint64(&countEvents, 1)
	}))

	t.Run("CheckProperties", func(t *testing.T) {
		p := payload.NewGenericDataPayload([]byte("TestCheckProperties"))
		msg, err := tangle.MessageFactory.IssuePayload(p)
		require.NoError(t, err)

		// TODO: approval switch: make test case with weak parents
		assert.NotEmpty(t, msg.ParentsByType(StrongParentType))

		// time in range of 0.1 seconds
		assert.InDelta(t, clock.SyncedTime().UnixNano(), msg.IssuingTime().UnixNano(), 100000000)

		// check payload
		assert.Equal(t, p, msg.Payload())

		// check total events and sequence number
		assert.EqualValues(t, 1, countEvents)
		assert.EqualValues(t, 0, msg.SequenceNumber())

		sequenceNumbers.Store(msg.SequenceNumber(), true)
	})

	// create messages in parallel
	t.Run("ParallelCreation", func(t *testing.T) {
		for i := 1; i < totalMessages; i++ {
			t.Run("test", func(t *testing.T) {
				t.Parallel()

				p := payload.NewGenericDataPayload([]byte("TestParallelCreation"))
				msg, err := tangle.MessageFactory.IssuePayload(p)
				require.NoError(t, err)

				// TODO: approval switch: make test case with weak parents
				assert.NotEmpty(t, msg.ParentsByType(StrongParentType))

				// time in range of 0.1 seconds
				assert.InDelta(t, clock.SyncedTime().UnixNano(), msg.IssuingTime().UnixNano(), 100000000)

				// check payload
				assert.Equal(t, p, msg.Payload())

				sequenceNumbers.Store(msg.SequenceNumber(), true)
			})
		}
	})

	// check total events and sequence number
	assert.EqualValues(t, totalMessages, countEvents)

	max := uint64(0)
	countSequence := 0
	sequenceNumbers.Range(func(key, value interface{}) bool {
		seq := key.(uint64)
		val := value.(bool)
		if val != true {
			return false
		}

		// check for max sequence number
		if seq > max {
			max = seq
		}
		countSequence++
		return true
	})
	assert.EqualValues(t, totalMessages-1, max)
	assert.EqualValues(t, totalMessages, countSequence)
}

func TestMessageFactory_POW(t *testing.T) {
	mockOTV := &SimpleMockOnTangleVoting{}

	tangle := NewTestTangle()
	defer tangle.Shutdown()
	tangle.OTVConsensusManager = NewOTVConsensusManager(mockOTV)

	msgFactory := NewMessageFactory(
		tangle,
		TipSelectorFunc(func(p payload.Payload, countParents int) (parentsMessageIDs MessageIDs, err error) {
			return []MessageID{EmptyMessageID}, nil
		}),
		func(parents MessageIDs, tangle *Tangle) (MessageIDs, error) {
			return []MessageID{}, nil
		},
	)
	defer msgFactory.Shutdown()

	worker := pow.New(1)

	msgFactory.SetWorker(WorkerFunc(func(msgBytes []byte) (uint64, error) {
		content := msgBytes[:len(msgBytes)-ed25519.SignatureSize-8]
		return worker.Mine(context.Background(), content, targetPOW)
	}))
	msgFactory.SetTimeout(powTimeout)
	msg, err := msgFactory.IssuePayload(payload.NewGenericDataPayload([]byte("test")))
	require.NoError(t, err)

	msgBytes := msg.Bytes()
	content := msgBytes[:len(msgBytes)-ed25519.SignatureSize-8]

	zeroes, err := worker.LeadingZerosWithNonce(content, msg.Nonce())
	assert.GreaterOrEqual(t, zeroes, targetPOW)
	assert.NoError(t, err)
}

func TestMessageFactory_PrepareLikedReferences(t *testing.T) {
	tangle := NewTestTangle()
	defer tangle.Shutdown()
	tangle.Booker.Setup()

	// TODO add (mocked?) otv mechanism

	wallets := make(map[string]wallet)
	walletsByAddress := make(map[ledgerstate.Address]wallet)
	w := createWallets(6)
	wallets["GENESIS"] = w[0]
	wallets["O1"] = w[1]
	wallets["O2"] = w[2]
	wallets["O3"] = w[3]
	wallets["O4"] = w[4]
	wallets["O5"] = w[5]

	for _, wallet := range wallets {
		walletsByAddress[wallet.address] = wallet
	}

	genesisBalance := ledgerstate.NewColoredBalances(
		map[ledgerstate.Color]uint64{
			ledgerstate.ColorIOTA: 5,
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
	fmt.Println(genesisTransaction.ID())
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

	// Message 1
	inputs["GENESIS"] = ledgerstate.NewUTXOInput(ledgerstate.NewOutputID(genesisTransaction.ID(), 0))
	outputs["O1"] = ledgerstate.NewSigLockedSingleOutput(2, wallets["O1"].address)
	outputs["O2"] = ledgerstate.NewSigLockedSingleOutput(3, wallets["O2"].address)

	transactions["1"] = makeTransaction(ledgerstate.NewInputs(inputs["GENESIS"]), ledgerstate.NewOutputs(outputs["O1"], outputs["O2"]), outputsByID, walletsByAddress, wallets["GENESIS"])
	messages["1"] = newTestParentsPayloadMessage(transactions["1"], []MessageID{EmptyMessageID}, []MessageID{})
	tangle.Storage.StoreMessage(messages["1"])

	err := tangle.Booker.BookMessage(messages["1"].ID())
	require.NoError(t, err)

	// Message 2
	inputs["O1"] = ledgerstate.NewUTXOInput(ledgerstate.NewOutputID(transactions["1"].ID(), selectIndex(transactions["1"], wallets["O1"])))
	outputsByID[inputs["O1"].ReferencedOutputID()] = ledgerstate.NewOutputs(outputs["O1"])[0]
	outputs["O3"] = ledgerstate.NewSigLockedSingleOutput(2, wallets["O3"].address)
	transactions["2"] = makeTransaction(ledgerstate.NewInputs(inputs["O1"]), ledgerstate.NewOutputs(outputs["O3"]), outputsByID, walletsByAddress)
	messages["2"] = newTestParentsPayloadMessage(transactions["2"], []MessageID{messages["1"].ID()}, []MessageID{})
	tangle.Storage.StoreMessage(messages["2"])

	err = tangle.Booker.BookMessage(messages["2"].ID())
	require.NoError(t, err)

	// Message 3
	inputs["O2"] = ledgerstate.NewUTXOInput(ledgerstate.NewOutputID(transactions["1"].ID(), selectIndex(transactions["1"], wallets["O2"])))
	outputsByID[inputs["O2"].ReferencedOutputID()] = ledgerstate.NewOutputs(outputs["O2"])[0]
	outputs["O5"] = ledgerstate.NewSigLockedSingleOutput(3, wallets["O5"].address)
	transactions["3"] = makeTransaction(ledgerstate.NewInputs(inputs["O2"]), ledgerstate.NewOutputs(outputs["O5"]), outputsByID, walletsByAddress)
	messages["3"] = newTestParentsPayloadMessage(transactions["3"], []MessageID{messages["2"].ID()}, []MessageID{})
	tangle.Storage.StoreMessage(messages["3"])

	err = tangle.Booker.BookMessage(messages["3"].ID())
	require.NoError(t, err)

	// Message 4
	outputs["O4"] = ledgerstate.NewSigLockedSingleOutput(5, wallets["O4"].address)
	transactions["4"] = makeTransaction(ledgerstate.NewInputs(inputs["O2"], inputs["O1"]), ledgerstate.NewOutputs(outputs["O4"]), outputsByID, walletsByAddress)
	messages["4"] = newTestParentsPayloadMessage(transactions["4"], []MessageID{messages["1"].ID()}, []MessageID{})
	tangle.Storage.StoreMessage(messages["4"])

	fmt.Println("Transaction IDs:")
	fmt.Println(genesisTransaction.ID())
	fmt.Println(transactions["1"].ID())
	fmt.Println(transactions["2"].ID())
	fmt.Println(transactions["3"].ID())
	fmt.Println(transactions["4"].ID())

	err = tangle.Booker.BookMessage(messages["4"].ID())
	require.NoError(t, err)

	branches["1"], _ = transactionBranchID(tangle, transactions["1"].ID())
	branches["2"], _ = transactionBranchID(tangle, transactions["2"].ID())
	branches["3"], _ = transactionBranchID(tangle, transactions["3"].ID())
	branches["4"], _ = transactionBranchID(tangle, transactions["4"].ID())
	fmt.Println(branches)

	branches["1"], _ = tangle.Booker.MessageBranchID(messages["1"].ID())
	branches["2"], _ = tangle.Booker.MessageBranchID(messages["2"].ID())
	branches["3"], _ = tangle.Booker.MessageBranchID(messages["3"].ID())
	branches["4"], _ = tangle.Booker.MessageBranchID(messages["4"].ID())
	fmt.Println(branches)

	mockOTV := &SimpleMockOnTangleVoting{
		disliked: ledgerstate.NewBranchIDs(branches["4"]),
		likedInstead: map[ledgerstate.BranchID][]consensus.OpinionTuple{branches["4"]: {consensus.OpinionTuple{Liked: branches["2"], Disliked: branches["4"]},
			consensus.OpinionTuple{Liked: branches["3"], Disliked: branches["4"]}}},
	}
	tangle.OTVConsensusManager = NewOTVConsensusManager(mockOTV)

	references, err := PrepareLikeReferences(MessageIDs{messages["4"].ID(), messages["3"].ID()}, tangle)

	require.NoError(t, err)

	assert.Contains(t, references, messages["2"].ID())
	assert.Contains(t, references, messages["3"].ID())
	fmt.Println(references)
}
