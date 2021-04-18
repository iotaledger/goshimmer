package fcob

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/identity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/epochs"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/goshimmer/packages/tangle/payload"
	"github.com/iotaledger/goshimmer/packages/vote"
	"github.com/iotaledger/goshimmer/packages/vote/opinion"
)

func TestOpinionFormer_Scenario2(t *testing.T) {
	LikedThreshold = 2 * time.Second
	LocallyFinalizedThreshold = 2 * time.Second

	consensusProvider := NewConsensusMechanism()

	testTangle := tangle.New(tangle.Consensus(consensusProvider))
	defer testTangle.Shutdown()
	testTangle.Setup()

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
		time.Unix(epochs.DefaultGenesisTime, 0),
		identity.ID{},
		identity.ID{},
		ledgerstate.NewInputs(ledgerstate.NewUTXOInput(ledgerstate.NewOutputID(ledgerstate.GenesisTransactionID, 0))),
		ledgerstate.NewOutputs(ledgerstate.NewSigLockedColoredOutput(genesisBalance, wallets["GENESIS"].address)),
	)

	genesisTransaction := ledgerstate.NewTransaction(genesisEssence, ledgerstate.UnlockBlocks{ledgerstate.NewReferenceUnlockBlock(0)})

	snapshot := &ledgerstate.Snapshot{
		Transactions: map[ledgerstate.TransactionID]*ledgerstate.TransactionEssence{
			genesisTransaction.ID(): genesisEssence,
		},
	}

	testTangle.LedgerState.LoadSnapshot(snapshot)

	messages := make(map[string]*tangle.Message)
	transactions := make(map[string]*ledgerstate.Transaction)
	inputs := make(map[string]*ledgerstate.UTXOInput)
	outputs := make(map[string]*ledgerstate.SigLockedSingleOutput)
	outputsByID := make(map[ledgerstate.OutputID]ledgerstate.Output)

	// Message 1
	inputs["GENESIS"] = ledgerstate.NewUTXOInput(ledgerstate.NewOutputID(genesisTransaction.ID(), 0))
	outputs["A"] = ledgerstate.NewSigLockedSingleOutput(1, wallets["A"].address)
	outputs["B"] = ledgerstate.NewSigLockedSingleOutput(1, wallets["B"].address)
	outputs["C"] = ledgerstate.NewSigLockedSingleOutput(1, wallets["C"].address)
	transactions["1"] = makeTransaction(ledgerstate.NewInputs(inputs["GENESIS"]), ledgerstate.NewOutputs(outputs["A"], outputs["B"], outputs["C"]), outputsByID, walletsByAddress, wallets["GENESIS"])
	messages["1"] = newTestParentsPayloadMessage(transactions["1"], []tangle.MessageID{tangle.EmptyMessageID}, []tangle.MessageID{})

	// Message 2
	inputs["B"] = ledgerstate.NewUTXOInput(ledgerstate.NewOutputID(transactions["1"].ID(), selectIndex(transactions["1"], wallets["B"])))
	outputsByID[inputs["B"].ReferencedOutputID()] = ledgerstate.NewOutputs(outputs["B"])[0]
	inputs["C"] = ledgerstate.NewUTXOInput(ledgerstate.NewOutputID(transactions["1"].ID(), selectIndex(transactions["1"], wallets["C"])))
	outputsByID[inputs["C"].ReferencedOutputID()] = ledgerstate.NewOutputs(outputs["C"])[0]
	outputs["E"] = ledgerstate.NewSigLockedSingleOutput(2, wallets["E"].address)
	transactions["2"] = makeTransaction(ledgerstate.NewInputs(inputs["B"], inputs["C"]), ledgerstate.NewOutputs(outputs["E"]), outputsByID, walletsByAddress)
	messages["2"] = newTestParentsPayloadMessage(transactions["2"], []tangle.MessageID{tangle.EmptyMessageID, messages["1"].ID()}, []tangle.MessageID{})

	// Message 3 (Reattachemnt of transaction 2)
	messages["3"] = newTestParentsPayloadMessage(transactions["2"], []tangle.MessageID{messages["1"].ID(), messages["2"].ID()}, []tangle.MessageID{})

	// Message 4
	inputs["A"] = ledgerstate.NewUTXOInput(ledgerstate.NewOutputID(transactions["1"].ID(), selectIndex(transactions["1"], wallets["A"])))
	outputsByID[inputs["A"].ReferencedOutputID()] = ledgerstate.NewOutputs(outputs["A"])[0]
	outputs["D"] = ledgerstate.NewSigLockedSingleOutput(1, wallets["D"].address)
	transactions["3"] = makeTransaction(ledgerstate.NewInputs(inputs["A"]), ledgerstate.NewOutputs(outputs["D"]), outputsByID, walletsByAddress)
	messages["4"] = newTestParentsPayloadMessage(transactions["3"], []tangle.MessageID{tangle.EmptyMessageID, messages["1"].ID()}, []tangle.MessageID{})

	// Message 5
	outputs["F"] = ledgerstate.NewSigLockedSingleOutput(1, wallets["F"].address)
	transactions["4"] = makeTransaction(ledgerstate.NewInputs(inputs["A"]), ledgerstate.NewOutputs(outputs["F"]), outputsByID, walletsByAddress)
	messages["5"] = newTestParentsPayloadMessage(transactions["4"], []tangle.MessageID{messages["1"].ID()}, []tangle.MessageID{messages["2"].ID()})

	// Message 6
	inputs["E"] = ledgerstate.NewUTXOInput(ledgerstate.NewOutputID(transactions["2"].ID(), 0))
	outputsByID[inputs["E"].ReferencedOutputID()] = ledgerstate.NewOutputs(outputs["E"])[0]
	inputs["F"] = ledgerstate.NewUTXOInput(ledgerstate.NewOutputID(transactions["4"].ID(), 0))
	outputsByID[inputs["F"].ReferencedOutputID()] = ledgerstate.NewOutputs(outputs["F"])[0]
	outputs["L"] = ledgerstate.NewSigLockedSingleOutput(3, wallets["L"].address)
	transactions["5"] = makeTransaction(ledgerstate.NewInputs(inputs["E"], inputs["F"]), ledgerstate.NewOutputs(outputs["L"]), outputsByID, walletsByAddress)
	messages["6"] = newTestParentsPayloadMessage(transactions["5"], []tangle.MessageID{messages["2"].ID(), messages["5"].ID()}, []tangle.MessageID{})

	// Message 7
	outputs["H"] = ledgerstate.NewSigLockedSingleOutput(1, wallets["H"].address)
	transactions["6"] = makeTransaction(ledgerstate.NewInputs(inputs["C"]), ledgerstate.NewOutputs(outputs["H"]), outputsByID, walletsByAddress)
	messages["7"] = newTestParentsPayloadMessage(transactions["6"], []tangle.MessageID{messages["1"].ID(), messages["4"].ID()}, []tangle.MessageID{})

	// Message 8
	inputs["H"] = ledgerstate.NewUTXOInput(ledgerstate.NewOutputID(transactions["6"].ID(), 0))
	outputsByID[inputs["H"].ReferencedOutputID()] = ledgerstate.NewOutputs(outputs["H"])[0]
	inputs["D"] = ledgerstate.NewUTXOInput(ledgerstate.NewOutputID(transactions["3"].ID(), 0))
	outputsByID[inputs["D"].ReferencedOutputID()] = ledgerstate.NewOutputs(outputs["D"])[0]
	outputs["I"] = ledgerstate.NewSigLockedSingleOutput(2, wallets["I"].address)
	transactions["7"] = makeTransaction(ledgerstate.NewInputs(inputs["D"], inputs["H"]), ledgerstate.NewOutputs(outputs["I"]), outputsByID, walletsByAddress)
	messages["8"] = newTestParentsPayloadMessage(transactions["7"], []tangle.MessageID{messages["4"].ID(), messages["7"].ID()}, []tangle.MessageID{})

	// Message 9
	outputs["J"] = ledgerstate.NewSigLockedSingleOutput(1, wallets["J"].address)
	transactions["8"] = makeTransaction(ledgerstate.NewInputs(inputs["B"]), ledgerstate.NewOutputs(outputs["J"]), outputsByID, walletsByAddress)
	messages["9"] = newTestParentsPayloadMessage(transactions["8"], []tangle.MessageID{messages["4"].ID(), messages["7"].ID()}, []tangle.MessageID{})

	// setup mock voter

	transactionLiked := make(map[ledgerstate.TransactionID]bool)
	transactionLiked[transactions["1"].ID()] = true
	transactionLiked[transactions["2"].ID()] = false
	transactionLiked[transactions["3"].ID()] = true
	transactionLiked[transactions["4"].ID()] = false
	transactionLiked[transactions["6"].ID()] = true
	transactionLiked[transactions["8"].ID()] = false

	consensusProvider.Events.Vote.Attach(events.NewClosure(func(transactionID string, initialOpinion opinion.Opinion) {
		t.Log("Voting requested for:", transactionID)
		txID, err := ledgerstate.TransactionIDFromBase58(transactionID)
		require.NoError(t, err)
		o := opinion.Dislike
		if transactionLiked[txID] {
			o = opinion.Like
		}
		consensusProvider.ProcessVote(&vote.OpinionEvent{
			ID:      transactionID,
			Opinion: o,
			Ctx:     vote.Context{Type: vote.ConflictType},
		})
	}))

	consensusProvider.Events.Error.Attach(events.NewClosure(func(err error) {
		t.Log("VoteError", err)
	}))

	payloadLiked := make(map[tangle.MessageID]bool)
	payloadLiked[messages["1"].ID()] = true
	payloadLiked[messages["2"].ID()] = false
	payloadLiked[messages["3"].ID()] = false
	payloadLiked[messages["4"].ID()] = true
	payloadLiked[messages["5"].ID()] = false
	payloadLiked[messages["6"].ID()] = false
	payloadLiked[messages["7"].ID()] = true
	payloadLiked[messages["8"].ID()] = true
	payloadLiked[messages["9"].ID()] = false

	testTangle.Solidifier.Events.MessageSolid.Attach(events.NewClosure(func(messageID tangle.MessageID) {
		t.Log("Message solid: ", messageID)
	}))

	testTangle.Scheduler.Events.MessageScheduled.Attach(events.NewClosure(func(messageID tangle.MessageID) {
		t.Log("Message scheduled: ", messageID)
	}))

	testTangle.Booker.Events.MessageBooked.Attach(events.NewClosure(func(messageID tangle.MessageID) {
		t.Log("Message Booked: ", messageID)
	}))

	testTangle.Events.Error.Attach(events.NewClosure(func(err error) {
		t.Log("Tangle Error: ", err)
	}))

	var wg sync.WaitGroup
	testTangle.ConsensusManager.Events.MessageOpinionFormed.Attach(events.NewClosure(func(messageID tangle.MessageID) {
		t.Logf("MessageOpinionFormed for %s", messageID)

		assert.True(t, testTangle.ConsensusManager.MessageEligible(messageID))
		assert.Equal(t, payloadLiked[messageID], testTangle.ConsensusManager.PayloadLiked(messageID))
		t.Log("Payload Liked:", testTangle.ConsensusManager.PayloadLiked(messageID))
		wg.Done()
	}))

	wg.Add(9)

	testTangle.Storage.StoreMessage(messages["1"])
	testTangle.Storage.StoreMessage(messages["2"])
	testTangle.Storage.StoreMessage(messages["3"])
	testTangle.Storage.StoreMessage(messages["4"])
	testTangle.Storage.StoreMessage(messages["5"])
	testTangle.Storage.StoreMessage(messages["6"])
	testTangle.Storage.StoreMessage(messages["7"])
	testTangle.Storage.StoreMessage(messages["8"])
	testTangle.Storage.StoreMessage(messages["9"])

	wg.Wait()
}

func TestOpinionFormer(t *testing.T) {
	LikedThreshold = 1 * time.Second
	LocallyFinalizedThreshold = 2 * time.Second

	consensusProvider := NewConsensusMechanism()

	testTangle := tangle.New(tangle.Consensus(consensusProvider))
	defer testTangle.Shutdown()

	messageA := newTestDataMessage("A")

	wallets := createWallets(2)

	genesisBalance := ledgerstate.NewColoredBalances(
		map[ledgerstate.Color]uint64{
			ledgerstate.ColorIOTA: 10000,
		})

	genesisEssence := ledgerstate.NewTransactionEssence(
		0,
		time.Unix(epochs.DefaultGenesisTime, 0),
		identity.ID{},
		identity.ID{},
		ledgerstate.NewInputs(ledgerstate.NewUTXOInput(ledgerstate.NewOutputID(ledgerstate.GenesisTransactionID, 0))),
		ledgerstate.NewOutputs(ledgerstate.NewSigLockedColoredOutput(genesisBalance, wallets[0].address)),
	)

	genesisTransaction := ledgerstate.NewTransaction(genesisEssence, ledgerstate.UnlockBlocks{ledgerstate.NewReferenceUnlockBlock(0)})

	snapshot := &ledgerstate.Snapshot{
		Transactions: map[ledgerstate.TransactionID]*ledgerstate.TransactionEssence{
			genesisTransaction.ID(): genesisEssence,
		},
	}

	testTangle.LedgerState.LoadSnapshot(snapshot)

	input := ledgerstate.NewUTXOInput(ledgerstate.NewOutputID(genesisTransaction.ID(), 0))
	output := ledgerstate.NewSigLockedSingleOutput(10000, wallets[0].address)
	txEssence := ledgerstate.NewTransactionEssence(0, time.Now(), identity.ID{}, identity.ID{}, ledgerstate.NewInputs(input), ledgerstate.NewOutputs(output))
	tx1 := ledgerstate.NewTransaction(txEssence, wallets[0].unlockBlocks(txEssence))
	t.Log("Transacion1: ", tx1.ID())
	messageB := newTestParentsPayloadMessage(tx1, []tangle.MessageID{messageA.ID()}, []tangle.MessageID{})

	output2 := ledgerstate.NewSigLockedSingleOutput(10000, wallets[1].address)
	txEssence2 := ledgerstate.NewTransactionEssence(0, time.Now(), identity.ID{}, identity.ID{}, ledgerstate.NewInputs(input), ledgerstate.NewOutputs(output2))
	tx2 := ledgerstate.NewTransaction(txEssence2, wallets[0].unlockBlocks(txEssence2))
	t.Log("Transacion2: ", tx2.ID())
	messageC := newTestParentsPayloadMessage(tx2, []tangle.MessageID{messageA.ID()}, []tangle.MessageID{})

	payloadLiked := make(map[tangle.MessageID]bool)
	payloadLiked[messageA.ID()] = true
	payloadLiked[messageB.ID()] = false
	payloadLiked[messageC.ID()] = false

	testTangle.Setup()

	testTangle.Booker.Events.MessageBooked.Attach(events.NewClosure(func(messageID tangle.MessageID) {
		t.Log("Message Booked:", messageID)
	}))

	testTangle.Events.MessageInvalid.Attach(events.NewClosure(func(messageID tangle.MessageID) {
		t.Log("Invalid message:", messageID)
	}))

	var wg sync.WaitGroup

	testTangle.ConsensusManager.Events.MessageOpinionFormed.Attach(events.NewClosure(func(messageID tangle.MessageID) {
		t.Log("MessageOpinionFormed for ", messageID)

		assert.True(t, testTangle.ConsensusManager.MessageEligible(messageID))
		assert.Equal(t, payloadLiked[messageID], testTangle.ConsensusManager.PayloadLiked(messageID))
		t.Log("Payload Liked:", testTangle.ConsensusManager.PayloadLiked(messageID))
		wg.Done()
	}))

	consensusProvider.Events.Vote.Attach(events.NewClosure(func(transactionID string, initialOpinion opinion.Opinion) {
		t.Log("Voting requested for:", transactionID)
		consensusProvider.ProcessVote(&vote.OpinionEvent{
			ID:      transactionID,
			Opinion: opinion.Dislike,
			Ctx:     vote.Context{Type: vote.ConflictType},
		})
	}))

	consensusProvider.Events.Error.Attach(events.NewClosure(func(err error) {
		t.Log("VoteError", err)
	}))

	wg.Add(3)

	testTangle.Storage.StoreMessage(messageA)
	testTangle.Storage.StoreMessage(messageB)
	testTangle.Storage.StoreMessage(messageC)

	wg.Wait()

	t.Log("Waiting shutdown..")
}

func TestDeriveOpinion(t *testing.T) {
	now := time.Now()

	// A{t0, pending} -> B{t-t0 < c, Dislike, One}
	{
		conflictSet := ConflictSet{
			OpinionEssence{
				timestamp:        now,
				liked:            false,
				levelOfKnowledge: Pending,
			},
		}

		opinion := deriveOpinion(now.Add(1*time.Second), conflictSet)
		assert.Equal(t, OpinionEssence{
			timestamp:        now.Add(1 * time.Second),
			liked:            false,
			levelOfKnowledge: One,
		}, opinion)
	}

	// A{t0, Like, One} -> B{c < t-t0 < c + d, Dislike, One}
	{
		{
			conflictSet := ConflictSet{
				OpinionEssence{
					timestamp:        now,
					liked:            true,
					levelOfKnowledge: One,
				},
			}

			opinion := deriveOpinion(now.Add(1*time.Second), conflictSet)
			assert.Equal(t, OpinionEssence{
				timestamp:        now.Add(1 * time.Second),
				liked:            false,
				levelOfKnowledge: One,
			}, opinion)
		}
	}

	// {t0, Like, Two} -> {t-t0 > c + d, Dislike, Two}
	{
		{
			conflictSet := ConflictSet{
				OpinionEssence{
					timestamp:        now,
					liked:            true,
					levelOfKnowledge: Two,
				},
			}

			opinion := deriveOpinion(now.Add(1*time.Second), conflictSet)
			assert.Equal(t, OpinionEssence{
				timestamp:        now.Add(1 * time.Second),
				liked:            false,
				levelOfKnowledge: Two,
			}, opinion)
		}
	}

	// A{t0, Dislike, Two}, B{t1, Dislike, Two} -> {t-t0 >> c + d, Dislike, Pending}
	{
		{
			conflictSet := ConflictSet{
				OpinionEssence{
					timestamp:        now,
					liked:            false,
					levelOfKnowledge: Two,
				}, OpinionEssence{
					timestamp:        now,
					liked:            false,
					levelOfKnowledge: Two,
				},
			}

			opinion := deriveOpinion(now.Add(1*time.Second), conflictSet)
			assert.Equal(t, OpinionEssence{
				timestamp:        now.Add(1 * time.Second),
				liked:            false,
				levelOfKnowledge: Pending,
			}, opinion)
		}
	}

	// * double check this case
	//  {t0, Dislike, One}, {t1, Dislike, One} -> {t-t0 > 0, Dislike, one}
	{
		{
			conflictSet := ConflictSet{
				OpinionEssence{
					timestamp:        now,
					liked:            false,
					levelOfKnowledge: One,
				}, OpinionEssence{
					timestamp:        now.Add(1 * time.Second),
					liked:            false,
					levelOfKnowledge: One,
				},
			}

			opinion := deriveOpinion(now.Add(10*time.Second), conflictSet)
			assert.Equal(t, OpinionEssence{
				timestamp:        now.Add(10 * time.Second),
				liked:            false,
				levelOfKnowledge: One,
			}, opinion)
		}
	}

	// * double check this case
	//  {t0, Like, One}, {t1, Dislike, One} -> {t - t0 > c, Dislike, one}
	{
		{
			conflictSet := ConflictSet{
				OpinionEssence{
					timestamp:        now,
					liked:            true,
					levelOfKnowledge: One,
				}, OpinionEssence{
					timestamp:        now.Add(1 * time.Second),
					liked:            false,
					levelOfKnowledge: One,
				},
			}

			opinion := deriveOpinion(now.Add(10*time.Second), conflictSet)
			assert.Equal(t, OpinionEssence{
				timestamp:        now.Add(10 * time.Second),
				liked:            false,
				levelOfKnowledge: One,
			}, opinion)
		}
	}

	//  {t0, Dislike, Two}, {t1, Like, One} -> {t10, Dislike, one}
	{
		{
			conflictSet := ConflictSet{
				OpinionEssence{
					timestamp:        now,
					liked:            false,
					levelOfKnowledge: Two,
				}, OpinionEssence{
					timestamp:        now.Add(1 * time.Second),
					liked:            true,
					levelOfKnowledge: One,
				},
			}

			opinion := deriveOpinion(now.Add(6*time.Second), conflictSet)
			assert.Equal(t, OpinionEssence{
				timestamp:        now.Add(6 * time.Second),
				liked:            false,
				levelOfKnowledge: One,
			}, opinion)
		}
	}

	//  {t0, Dislike, Two}, {t1, Dislike, One} -> {t10, Dislike, one}
	{
		{
			conflictSet := ConflictSet{
				OpinionEssence{
					timestamp:        now,
					liked:            false,
					levelOfKnowledge: Two,
				}, OpinionEssence{
					timestamp:        now.Add(1 * time.Second),
					liked:            false,
					levelOfKnowledge: One,
				},
			}

			opinion := deriveOpinion(now.Add(6*time.Second), conflictSet)
			assert.Equal(t, OpinionEssence{
				timestamp:        now.Add(6 * time.Second),
				liked:            false,
				levelOfKnowledge: One,
			}, opinion)
		}
	}
}

type wallet struct {
	keyPair ed25519.KeyPair
	address *ledgerstate.ED25519Address
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
			ledgerstate.NewED25519Address(kp.PublicKey),
		}
	}
	return wallets
}

func (w wallet) sign(txEssence *ledgerstate.TransactionEssence) *ledgerstate.ED25519Signature {
	return ledgerstate.NewED25519Signature(w.publicKey(), w.privateKey().Sign(txEssence.Bytes()))
}

func (w wallet) unlockBlocks(txEssence *ledgerstate.TransactionEssence) []ledgerstate.UnlockBlock {
	unlockBlock := ledgerstate.NewSignatureUnlockBlock(w.sign(txEssence))
	unlockBlocks := make([]ledgerstate.UnlockBlock, len(txEssence.Inputs()))
	for i := range txEssence.Inputs() {
		unlockBlocks[i] = unlockBlock
	}
	return unlockBlocks
}

func makeTransaction(inputs ledgerstate.Inputs, outputs ledgerstate.Outputs, outputsByID map[ledgerstate.OutputID]ledgerstate.Output, walletsByAddress map[ledgerstate.Address]wallet, genesisWallet ...wallet) *ledgerstate.Transaction {
	txEssence := ledgerstate.NewTransactionEssence(0, time.Now(), identity.ID{}, identity.ID{}, inputs, outputs)
	unlockBlocks := make([]ledgerstate.UnlockBlock, len(txEssence.Inputs()))
	for i, input := range txEssence.Inputs() {
		w := wallet{}
		if genesisWallet != nil {
			w = genesisWallet[0]
		} else {
			w = walletsByAddress[addressFromInput(input, outputsByID)]
		}
		unlockBlocks[i] = ledgerstate.NewSignatureUnlockBlock(w.sign(txEssence))
	}
	return ledgerstate.NewTransaction(txEssence, unlockBlocks)
}

func selectIndex(transaction *ledgerstate.Transaction, w wallet) (index uint16) {
	for i, output := range transaction.Essence().Outputs() {
		if w.address == output.(*ledgerstate.SigLockedSingleOutput).Address() {
			return uint16(i)
		}
	}
	return
}

func newTestParentsPayloadMessage(payload payload.Payload, strongParents, weakParents []tangle.MessageID) *tangle.Message {
	return tangle.NewMessage(strongParents, weakParents, time.Now(), ed25519.PublicKey{}, nextSequenceNumber(), payload, 0, ed25519.Signature{})
}

var sequenceNumber uint64

func nextSequenceNumber() uint64 {
	return atomic.AddUint64(&sequenceNumber, 1) - 1
}

func newTestDataMessage(payloadString string) *tangle.Message {
	return tangle.NewMessage([]tangle.MessageID{tangle.EmptyMessageID}, []tangle.MessageID{}, time.Now(), ed25519.PublicKey{}, nextSequenceNumber(), payload.NewGenericDataPayload([]byte(payloadString)), 0, ed25519.Signature{})
}

// addressFromInput retrieves the Address belonging to an Input by looking it up in the outputs that we have created for
// the tests.
func addressFromInput(input ledgerstate.Input, outputsByID ledgerstate.OutputsByID) ledgerstate.Address {
	typeCastedInput, ok := input.(*ledgerstate.UTXOInput)
	if !ok {
		panic("unexpected Input type")
	}

	switch referencedOutput := outputsByID[typeCastedInput.ReferencedOutputID()]; referencedOutput.Type() {
	case ledgerstate.SigLockedSingleOutputType:
		typeCastedOutput, ok := referencedOutput.(*ledgerstate.SigLockedSingleOutput)
		if !ok {
			panic("failed to type cast SigLockedSingleOutput")
		}

		return typeCastedOutput.Address()
	case ledgerstate.SigLockedColoredOutputType:
		typeCastedOutput, ok := referencedOutput.(*ledgerstate.SigLockedColoredOutput)
		if !ok {
			panic("failed to type cast SigLockedColoredOutput")
		}
		return typeCastedOutput.Address()
	default:
		panic("unexpected Output type")
	}
}
