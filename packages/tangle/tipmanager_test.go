package tangle

import (
	"strconv"
	"testing"
	"time"

	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/identity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
)

func TestTipManager_AddTip(t *testing.T) {
	tangle := newTestTangle()
	defer tangle.Shutdown()
	tangle.Storage.Setup()
	tangle.Solidifier.Setup()
	tipManager := tangle.TipManager

	seed := ed25519.NewSeed()

	output := ledgerstate.NewSigLockedColoredOutput(
		ledgerstate.NewColoredBalances(map[ledgerstate.Color]uint64{
			ledgerstate.ColorIOTA: 10000,
		}),
		ledgerstate.NewED25519Address(seed.KeyPair(0).PublicKey),
	)

	genesisEssence := ledgerstate.NewTransactionEssence(
		0,
		time.Unix(DefaultGenesisTime, 0),
		identity.ID{},
		identity.ID{},
		ledgerstate.NewInputs(ledgerstate.NewUTXOInput(ledgerstate.NewOutputID(ledgerstate.GenesisTransactionID, 0))),
		ledgerstate.NewOutputs(output),
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

	// not eligible messages -> nothing is added
	{
		message := newTestParentsDataMessage("testmessage", []MessageID{EmptyMessageID}, []MessageID{})
		tangle.Storage.StoreMessage(message)
		tangle.Booker.BookMessage(message.ID())
		tangle.Storage.MessageMetadata(message.ID()).Consume(func(messageMetadata *MessageMetadata) {
			messageMetadata.SetEligible(false)
		})

		tipManager.AddTip(message)
		assert.Equal(t, 0, tipManager.StrongTipCount())
		assert.Equal(t, 0, tipManager.WeakTipCount())
	}

	// payload not liked -> nothing is added
	{
		randomID, err := identity.RandomID()
		require.NoError(t, err)

		transactionEssence := ledgerstate.NewTransactionEssence(
			1,
			time.Now(),
			randomID,
			randomID,
			ledgerstate.NewInputs(
				ledgerstate.NewUTXOInput(
					ledgerstate.NewOutputID(genesisTransaction.ID(), 0),
				),
			),
			ledgerstate.NewOutputs(
				ledgerstate.NewSigLockedSingleOutput(10000, ledgerstate.NewED25519Address(seed.KeyPair(1).PublicKey)),
			),
		)

		transaction := ledgerstate.NewTransaction(
			transactionEssence,
			[]ledgerstate.UnlockBlock{
				ledgerstate.NewSignatureUnlockBlock(ledgerstate.NewED25519Signature(seed.KeyPair(0).PublicKey, seed.KeyPair(0).PrivateKey.Sign(transactionEssence.Bytes()))),
			},
		)

		message := newTestParentsPayloadMessage(transaction, []MessageID{EmptyMessageID}, []MessageID{})
		tangle.Storage.StoreMessage(message)
		tangle.Booker.BookMessage(message.ID())
		tangle.Storage.MessageMetadata(message.ID()).Consume(func(messageMetadata *MessageMetadata) {
			messageMetadata.SetEligible(true)
		})

		// mock the Tangle's PayloadOpinionProvider so that we can add payloads without actually building opinions
		tangle.Options.ConsensusMechanism = &mockConsensusProvider{
			func(transactionID ledgerstate.TransactionID) bool {
				return false
			},
		}

		tipManager.AddTip(message)
		assert.Equal(t, 0, tipManager.StrongTipCount())
		assert.Equal(t, 0, tipManager.WeakTipCount())
	}

	// clean up
	err := tangle.Prune()
	require.NoError(t, err)

	// set up scenario (images/tipmanager-add-tips.png)
	messages := make(map[string]*Message)

	// Message 1
	{
		messages["1"] = createAndStoreEligibleTestParentsDataMessageInMasterBranch(tangle, []MessageID{EmptyMessageID}, []MessageID{})
		tipManager.AddTip(messages["1"])

		assert.Equal(t, 1, tipManager.StrongTipCount())
		assert.Equal(t, 0, tipManager.WeakTipCount())
		assert.Contains(t, tipManager.strongTips.Keys(), messages["1"].ID())
	}

	// Message 2
	{
		messages["2"] = createAndStoreEligibleTestParentsDataMessageInMasterBranch(tangle, []MessageID{EmptyMessageID}, []MessageID{})
		tipManager.AddTip(messages["2"])

		assert.Equal(t, 2, tipManager.StrongTipCount())
		assert.Equal(t, 0, tipManager.WeakTipCount())
		assert.Contains(t, tipManager.strongTips.Keys(), messages["1"].ID(), messages["2"].ID())
	}

	// Message 3
	{
		messages["3"] = createAndStoreEligibleTestParentsDataMessageInMasterBranch(tangle, []MessageID{EmptyMessageID, messages["1"].ID(), messages["2"].ID()}, []MessageID{})
		tipManager.AddTip(messages["3"])

		assert.Equal(t, 1, tipManager.StrongTipCount())
		assert.Equal(t, 0, tipManager.WeakTipCount())
		assert.Contains(t, tipManager.strongTips.Keys(), messages["3"].ID())
	}

	// Message 4
	{
		messages["4"] = createAndStoreEligibleTestParentsDataMessageInInvalidBranch(tangle, []MessageID{messages["2"].ID()}, []MessageID{messages["3"].ID()})
		tipManager.AddTip(messages["4"])

		assert.Equal(t, 1, tipManager.StrongTipCount())
		assert.Equal(t, 1, tipManager.WeakTipCount())
		assert.Contains(t, tipManager.strongTips.Keys(), messages["3"].ID())
		assert.Contains(t, tipManager.weakTips.Keys(), messages["4"].ID())
	}

	// Message 5
	{
		messages["5"] = createAndStoreEligibleTestParentsDataMessageInInvalidBranch(tangle, []MessageID{messages["3"].ID(), messages["4"].ID()}, []MessageID{})
		tipManager.AddTip(messages["5"])

		assert.Equal(t, 1, tipManager.StrongTipCount())
		assert.Equal(t, 2, tipManager.WeakTipCount())
		assert.Contains(t, tipManager.strongTips.Keys(), messages["3"].ID())
		assert.Contains(t, tipManager.weakTips.Keys(), messages["4"].ID(), messages["5"].ID())
	}

	// Message 6
	{
		messages["6"] = createAndStoreEligibleTestParentsDataMessageInMasterBranch(tangle, []MessageID{messages["3"].ID()}, []MessageID{messages["4"].ID(), messages["5"].ID()})
		tipManager.AddTip(messages["6"])

		assert.Equal(t, 1, tipManager.StrongTipCount())
		assert.Equal(t, 0, tipManager.WeakTipCount())
		assert.Contains(t, tipManager.strongTips.Keys(), messages["6"].ID())
	}
}

func TestTipManager_DataMessageTips(t *testing.T) {
	tangle := newTestTangle()
	defer tangle.Shutdown()
	tipManager := tangle.TipManager

	// set up scenario (images/tipmanager-DataMessageTips-test.png)
	messages := make(map[string]*Message)

	// without any tip -> genesis
	{
		strongParents, weakParents, err := tipManager.Tips(nil, 2, 2)
		assert.NoError(t, err)
		assert.Len(t, strongParents, 1)
		assert.Contains(t, strongParents, EmptyMessageID)
		assert.Empty(t, weakParents)
	}

	// without any count -> 1 tip, in this case genesis
	{
		strongParents, weakParents, err := tipManager.Tips(nil, 0, 0)
		assert.NoError(t, err)
		assert.Len(t, strongParents, 1)
		assert.Contains(t, strongParents, EmptyMessageID)
		assert.Empty(t, weakParents)
	}

	// Message 1
	{
		messages["1"] = createAndStoreEligibleTestParentsDataMessageInMasterBranch(tangle, []MessageID{EmptyMessageID}, []MessageID{})
		tipManager.AddTip(messages["1"])

		assert.Equal(t, 1, tipManager.StrongTipCount())
		assert.Equal(t, 0, tipManager.WeakTipCount())
		assert.Contains(t, tipManager.strongTips.Keys(), messages["1"].ID())

		strongParents, weakParents, err := tipManager.Tips(nil, 2, 2)
		assert.NoError(t, err)
		assert.Len(t, strongParents, 1)
		assert.Contains(t, strongParents, messages["1"].ID())
		assert.Empty(t, weakParents)
	}

	// Message 2
	{
		messages["2"] = createAndStoreEligibleTestParentsDataMessageInMasterBranch(tangle, []MessageID{EmptyMessageID}, []MessageID{})
		tipManager.AddTip(messages["2"])

		assert.Equal(t, 2, tipManager.StrongTipCount())
		assert.Equal(t, 0, tipManager.WeakTipCount())
		assert.Contains(t, tipManager.strongTips.Keys(), messages["1"].ID(), messages["2"].ID())

		strongParents, weakParents, err := tipManager.Tips(nil, 2, 2)
		assert.NoError(t, err)
		assert.Len(t, strongParents, 2)
		assert.Contains(t, strongParents, messages["1"].ID(), messages["2"].ID())
		assert.Empty(t, weakParents)
	}

	// Message 3
	{
		messages["3"] = createAndStoreEligibleTestParentsDataMessageInMasterBranch(tangle, []MessageID{messages["1"].ID(), messages["2"].ID()}, []MessageID{})
		tipManager.AddTip(messages["3"])

		assert.Equal(t, 1, tipManager.StrongTipCount())
		assert.Equal(t, 0, tipManager.WeakTipCount())
		assert.Contains(t, tipManager.strongTips.Keys(), messages["3"].ID())

		strongParents, weakParents, err := tipManager.Tips(nil, 2, 2)
		assert.NoError(t, err)
		assert.Len(t, strongParents, 1)
		assert.Contains(t, strongParents, messages["3"].ID())
		assert.Empty(t, weakParents)
	}

	// Message 4
	{
		messages["4"] = createAndStoreEligibleTestParentsDataMessageInInvalidBranch(tangle, []MessageID{messages["2"].ID()}, []MessageID{messages["3"].ID()})
		tipManager.AddTip(messages["4"])

		assert.Equal(t, 1, tipManager.StrongTipCount())
		assert.Equal(t, 1, tipManager.WeakTipCount())
		assert.Contains(t, tipManager.strongTips.Keys(), messages["3"].ID())
		assert.Contains(t, tipManager.weakTips.Keys(), messages["4"].ID())

		strongParents, weakParents, err := tipManager.Tips(nil, 2, 2)
		assert.NoError(t, err)
		assert.Len(t, strongParents, 1)
		assert.Contains(t, strongParents, messages["3"].ID())
		assert.Len(t, weakParents, 1)
		assert.Contains(t, weakParents, messages["4"].ID())
	}

	// Message 5
	{
		messages["5"] = createAndStoreEligibleTestParentsDataMessageInInvalidBranch(tangle, []MessageID{messages["3"].ID(), messages["4"].ID()}, []MessageID{})
		tipManager.AddTip(messages["5"])

		assert.Equal(t, 1, tipManager.StrongTipCount())
		assert.Equal(t, 2, tipManager.WeakTipCount())
		assert.Contains(t, tipManager.strongTips.Keys(), messages["3"].ID())
		assert.Contains(t, tipManager.weakTips.Keys(), messages["4"].ID(), messages["5"].ID())

		strongParents, weakParents, err := tipManager.Tips(nil, 2, 2)
		assert.NoError(t, err)
		assert.Len(t, strongParents, 1)
		assert.Contains(t, strongParents, messages["3"].ID())
		assert.Len(t, weakParents, 2)
		assert.Contains(t, weakParents, messages["4"].ID(), messages["5"].ID())
	}

	// Add Message 6-12
	{
		strongTips := make([]MessageID, 0, 9)
		strongTips = append(strongTips, messages["3"].ID())
		for count, n := range []int{6, 7, 8, 9, 10, 11, 12, 13} {
			nString := strconv.Itoa(n)
			messages[nString] = createAndStoreEligibleTestParentsDataMessageInMasterBranch(tangle, []MessageID{messages["1"].ID()}, []MessageID{})
			tipManager.AddTip(messages[nString])
			strongTips = append(strongTips, messages[nString].ID())

			assert.Equalf(t, count+2, tipManager.StrongTipCount(), "StrongTipCount does not match after adding Message %d", n)
			assert.Equalf(t, 2, tipManager.WeakTipCount(), "WeakTipCount does not match after adding Message %d", n)
			assert.ElementsMatchf(t, tipManager.strongTips.Keys(), strongTips, "Elements in strongTips do not match after adding Message %d", n)
			assert.Contains(t, tipManager.weakTips.Keys(), messages["4"].ID(), messages["5"].ID())
		}
	}

	// now we have strongTips: 9, weakTips: 2
	// Tips(2,2) -> 2,2
	{
		strongParents, weakParents, err := tipManager.Tips(nil, 2, 2)
		assert.NoError(t, err)
		assert.Len(t, strongParents, 2)
		assert.Len(t, weakParents, 2)
	}
	// Tips(8,2) -> 8,0
	{
		strongParents, weakParents, err := tipManager.Tips(nil, 8, 2)
		assert.NoError(t, err)
		assert.Len(t, strongParents, 8)
		assert.Len(t, weakParents, 0)
	}
	// Tips(9,2) -> 8,0
	{
		strongParents, weakParents, err := tipManager.Tips(nil, 9, 2)
		assert.NoError(t, err)
		assert.Len(t, strongParents, 8)
		assert.Len(t, weakParents, 0)
	}
	// Tips(7,2) -> 7,1
	{
		strongParents, weakParents, err := tipManager.Tips(nil, 7, 2)
		assert.NoError(t, err)
		assert.Len(t, strongParents, 7)
		assert.Len(t, weakParents, 1)
	}
	// Tips(6,2) -> 6,2
	{
		strongParents, weakParents, err := tipManager.Tips(nil, 6, 2)
		assert.NoError(t, err)
		assert.Len(t, strongParents, 6)
		assert.Len(t, weakParents, 2)
	}
	// Tips(4,1) -> 4,1
	{
		strongParents, weakParents, err := tipManager.Tips(nil, 4, 1)
		assert.NoError(t, err)
		assert.Len(t, strongParents, 4)
		assert.Len(t, weakParents, 1)
	}
	// Tips(0,2) -> 1,2
	{
		strongParents, weakParents, err := tipManager.Tips(nil, 0, 2)
		assert.NoError(t, err)
		assert.Len(t, strongParents, 1)
		assert.Len(t, weakParents, 2)
	}
}

func TestTipManager_TransactionTips(t *testing.T) {
	// set up scenario (images/tipmanager-TransactionTips-test.png)
	tangle := newTestTangle()
	defer tangle.Shutdown()
	tipManager := tangle.TipManager
	tangle.Solidifier.Setup()

	wallets := make(map[string]wallet)
	walletsByAddress := make(map[ledgerstate.Address]wallet)
	w := createWallets(27)
	wallets["G1"] = w[0]
	wallets["G2"] = w[1]
	wallets["A"] = w[2]
	wallets["B"] = w[3]
	wallets["C"] = w[4]
	wallets["D"] = w[5]
	wallets["E"] = w[6]
	wallets["F"] = w[7]
	wallets["H"] = w[8]
	wallets["I"] = w[9]
	wallets["J"] = w[10]
	wallets["K"] = w[11]
	wallets["L"] = w[12]
	wallets["M"] = w[13]
	wallets["N"] = w[14]
	wallets["O"] = w[15]
	wallets["P"] = w[16]
	wallets["Q"] = w[17]
	wallets["R"] = w[18]
	wallets["S"] = w[19]
	wallets["T"] = w[20]
	wallets["U"] = w[21]
	wallets["V"] = w[22]
	wallets["X"] = w[23]
	wallets["Y"] = w[24]
	wallets["Z"] = w[25]
	wallets["OUT"] = w[26]

	for _, wallet := range wallets {
		walletsByAddress[wallet.address] = wallet
	}

	g1Balance := ledgerstate.NewColoredBalances(
		map[ledgerstate.Color]uint64{
			ledgerstate.ColorIOTA: 5,
		})
	g2Balance := ledgerstate.NewColoredBalances(
		map[ledgerstate.Color]uint64{
			ledgerstate.ColorIOTA: 8,
		})

	genesisEssence := ledgerstate.NewTransactionEssence(
		0,
		time.Unix(DefaultGenesisTime, 0),
		identity.ID{},
		identity.ID{},
		ledgerstate.NewInputs(ledgerstate.NewUTXOInput(ledgerstate.NewOutputID(ledgerstate.GenesisTransactionID, 0))),
		ledgerstate.NewOutputs([]ledgerstate.Output{
			ledgerstate.NewSigLockedColoredOutput(g1Balance, wallets["G1"].address),
			ledgerstate.NewSigLockedColoredOutput(g2Balance, wallets["G2"].address),
		}...),
	)

	genesisTransaction := ledgerstate.NewTransaction(genesisEssence, ledgerstate.UnlockBlocks{ledgerstate.NewReferenceUnlockBlock(0)})

	snapshot := &ledgerstate.Snapshot{
		Transactions: map[ledgerstate.TransactionID]ledgerstate.Record{
			genesisTransaction.ID(): {
				Essence:        genesisEssence,
				UnlockBlocks:   ledgerstate.UnlockBlocks{ledgerstate.NewReferenceUnlockBlock(0)},
				UnspentOutputs: []bool{true, true},
			},
		},
	}

	tangle.LedgerState.LoadSnapshot(snapshot)
	// determine genesis index so that correct output can be referenced
	var g1, g2 uint16
	tangle.LedgerState.UTXODAG.CachedOutput(ledgerstate.NewOutputID(genesisTransaction.ID(), 0)).Consume(func(output ledgerstate.Output) {
		balance, _ := output.Balances().Get(ledgerstate.ColorIOTA)
		if balance == uint64(5) {
			g1 = 0
			g2 = 1
		} else {
			g1 = 1
			g2 = 0
		}
	})

	messages := make(map[string]*Message)
	transactions := make(map[string]*ledgerstate.Transaction)
	inputs := make(map[string]*ledgerstate.UTXOInput)
	outputs := make(map[string]*ledgerstate.SigLockedSingleOutput)
	outputsByID := make(map[ledgerstate.OutputID]ledgerstate.Output)

	// mock the Tangle's PayloadOpinionProvider so that we can add transaction payloads without actually building opinions
	tangle.Options.ConsensusMechanism = &mockConsensusProvider{
		func(transactionID ledgerstate.TransactionID) bool {
			for _, msg := range messages {
				if msg.Payload().Type() == ledgerstate.TransactionType {
					if transactionID == msg.Payload().(*ledgerstate.Transaction).ID() {
						return true
					}
				}
			}
			return false
		},
	}

	// region prepare scenario /////////////////////////////////////////////////////////////////////////////////////////

	// Message 1
	{
		inputs["G1"] = ledgerstate.NewUTXOInput(ledgerstate.NewOutputID(genesisTransaction.ID(), g1))
		outputs["A"] = ledgerstate.NewSigLockedSingleOutput(3, wallets["A"].address)
		outputs["B"] = ledgerstate.NewSigLockedSingleOutput(1, wallets["B"].address)
		outputs["C"] = ledgerstate.NewSigLockedSingleOutput(1, wallets["C"].address)

		transactions["1"] = makeTransaction(ledgerstate.NewInputs(inputs["G1"]), ledgerstate.NewOutputs(outputs["A"], outputs["B"], outputs["C"]), outputsByID, walletsByAddress, wallets["G1"])
		// make sure that message is too old and cannot be directly referenced
		issueTime := time.Now().Add(-maxParentsTimeDifference - 5*time.Minute)
		messages["1"] = newTestParentsPayloadWithTimestamp(transactions["1"], []MessageID{EmptyMessageID}, []MessageID{}, issueTime)

		storeBookLikeMessage(t, tangle, messages["1"])

		tipManager.AddTip(messages["1"])
		assert.Equal(t, 0, tipManager.StrongTipCount())
		assert.Equal(t, 0, tipManager.WeakTipCount())
	}

	// Message 2
	{
		inputs["G2"] = ledgerstate.NewUTXOInput(ledgerstate.NewOutputID(genesisTransaction.ID(), g2))
		outputs["D"] = ledgerstate.NewSigLockedSingleOutput(6, wallets["D"].address)
		outputs["E"] = ledgerstate.NewSigLockedSingleOutput(1, wallets["E"].address)
		outputs["F"] = ledgerstate.NewSigLockedSingleOutput(1, wallets["F"].address)

		transactions["2"] = makeTransaction(ledgerstate.NewInputs(inputs["G2"]), ledgerstate.NewOutputs(outputs["D"], outputs["E"], outputs["F"]), outputsByID, walletsByAddress, wallets["G2"])
		messages["2"] = newTestParentsPayloadMessage(transactions["2"], []MessageID{EmptyMessageID}, []MessageID{})

		storeBookLikeMessage(t, tangle, messages["2"])

		tipManager.AddTip(messages["2"])
		assert.Equal(t, 1, tipManager.StrongTipCount())
		assert.Equal(t, 0, tipManager.WeakTipCount())
	}

	// Message 3
	{
		inputs["A"] = ledgerstate.NewUTXOInput(ledgerstate.NewOutputID(transactions["1"].ID(), selectIndex(transactions["1"], wallets["A"])))
		outputsByID[inputs["A"].ReferencedOutputID()] = ledgerstate.NewOutputs(outputs["A"])[0]
		outputs["H"] = ledgerstate.NewSigLockedSingleOutput(1, wallets["H"].address)
		outputs["I"] = ledgerstate.NewSigLockedSingleOutput(1, wallets["I"].address)
		outputs["J"] = ledgerstate.NewSigLockedSingleOutput(1, wallets["J"].address)

		transactions["3"] = makeTransaction(ledgerstate.NewInputs(inputs["A"]), ledgerstate.NewOutputs(outputs["H"], outputs["I"], outputs["J"]), outputsByID, walletsByAddress)
		messages["3"] = newTestParentsPayloadMessage(transactions["3"], []MessageID{messages["1"].ID(), EmptyMessageID}, []MessageID{})

		storeBookLikeMessage(t, tangle, messages["3"])

		tipManager.AddTip(messages["3"])
		assert.Equal(t, 2, tipManager.StrongTipCount())
		assert.Equal(t, 0, tipManager.WeakTipCount())
	}

	// Message 4
	{
		inputs["D"] = ledgerstate.NewUTXOInput(ledgerstate.NewOutputID(transactions["2"].ID(), selectIndex(transactions["2"], wallets["D"])))
		outputsByID[inputs["D"].ReferencedOutputID()] = ledgerstate.NewOutputs(outputs["D"])[0]
		outputs["K"] = ledgerstate.NewSigLockedSingleOutput(1, wallets["K"].address)
		outputs["L"] = ledgerstate.NewSigLockedSingleOutput(1, wallets["L"].address)
		outputs["M"] = ledgerstate.NewSigLockedSingleOutput(1, wallets["M"].address)
		outputs["N"] = ledgerstate.NewSigLockedSingleOutput(1, wallets["N"].address)
		outputs["O"] = ledgerstate.NewSigLockedSingleOutput(1, wallets["O"].address)
		outputs["P"] = ledgerstate.NewSigLockedSingleOutput(1, wallets["P"].address)

		transactions["4"] = makeTransaction(
			ledgerstate.NewInputs(inputs["D"]),
			ledgerstate.NewOutputs(
				outputs["K"],
				outputs["L"],
				outputs["M"],
				outputs["N"],
				outputs["O"],
				outputs["P"],
			),
			outputsByID,
			walletsByAddress,
		)
		messages["4"] = newTestParentsPayloadMessage(transactions["4"], []MessageID{messages["2"].ID(), EmptyMessageID}, []MessageID{})

		storeBookLikeMessage(t, tangle, messages["4"])

		tipManager.AddTip(messages["4"])
		assert.Equal(t, 2, tipManager.StrongTipCount())
		assert.Equal(t, 0, tipManager.WeakTipCount())
	}

	// Message 5
	{
		messages["5"] = newTestParentsDataMessage("data", []MessageID{messages["1"].ID(), EmptyMessageID}, []MessageID{})

		storeBookLikeMessage(t, tangle, messages["5"])

		tipManager.AddTip(messages["5"])
		assert.Equal(t, 3, tipManager.StrongTipCount())
		assert.Equal(t, 0, tipManager.WeakTipCount())
	}

	createScenarioMessageWith1Input1Output := func(messageStringID, transactionStringID, consumedTransactionStringID, inputStringID, outputStringID string, strongParents []MessageID) {
		inputs[inputStringID] = ledgerstate.NewUTXOInput(ledgerstate.NewOutputID(transactions[consumedTransactionStringID].ID(), selectIndex(transactions[consumedTransactionStringID], wallets[inputStringID])))
		outputsByID[inputs[inputStringID].ReferencedOutputID()] = ledgerstate.NewOutputs(outputs[inputStringID])[0]
		outputs[outputStringID] = ledgerstate.NewSigLockedSingleOutput(1, wallets[outputStringID].address)

		transactions[transactionStringID] = makeTransaction(ledgerstate.NewInputs(inputs[inputStringID]), ledgerstate.NewOutputs(outputs[outputStringID]), outputsByID, walletsByAddress)
		messages[messageStringID] = newTestParentsPayloadMessage(transactions[transactionStringID], strongParents, []MessageID{})

		storeBookLikeMessage(t, tangle, messages[messageStringID])
	}

	// Message 6
	{
		createScenarioMessageWith1Input1Output("6", "5", "3", "H", "Q", []MessageID{messages["3"].ID(), messages["5"].ID()})
		tipManager.AddTip(messages["6"])
		assert.Equal(t, 2, tipManager.StrongTipCount())
		assert.Equal(t, 0, tipManager.WeakTipCount())
	}

	// Message 7
	{
		createScenarioMessageWith1Input1Output("7", "6", "3", "I", "R", []MessageID{messages["3"].ID(), messages["5"].ID()})
		tipManager.AddTip(messages["7"])
		assert.Equal(t, 3, tipManager.StrongTipCount())
		assert.Equal(t, 0, tipManager.WeakTipCount())
	}

	// Message 8
	{
		createScenarioMessageWith1Input1Output("8", "7", "3", "J", "S", []MessageID{messages["3"].ID(), messages["5"].ID()})
		tipManager.AddTip(messages["8"])
		assert.Equal(t, 4, tipManager.StrongTipCount())
		assert.Equal(t, 0, tipManager.WeakTipCount())
	}

	// Message 9
	{
		createScenarioMessageWith1Input1Output("9", "8", "4", "K", "T", []MessageID{messages["4"].ID()})
		tipManager.AddTip(messages["9"])
		assert.Equal(t, 4, tipManager.StrongTipCount())
		assert.Equal(t, 0, tipManager.WeakTipCount())
	}

	// Message 10
	{
		createScenarioMessageWith1Input1Output("10", "9", "4", "L", "U", []MessageID{messages["2"].ID(), messages["4"].ID()})
		tipManager.AddTip(messages["10"])
		assert.Equal(t, 5, tipManager.StrongTipCount())
		assert.Equal(t, 0, tipManager.WeakTipCount())
	}

	// Message 11
	{
		createScenarioMessageWith1Input1Output("11", "10", "4", "M", "V", []MessageID{messages["2"].ID(), messages["4"].ID()})
		tipManager.AddTip(messages["11"])
		assert.Equal(t, 6, tipManager.StrongTipCount())
		assert.Equal(t, 0, tipManager.WeakTipCount())
	}

	// Message 12
	{
		createScenarioMessageWith1Input1Output("12", "11", "4", "N", "X", []MessageID{messages["3"].ID(), messages["4"].ID()})
		tipManager.AddTip(messages["12"])
		assert.Equal(t, 7, tipManager.StrongTipCount())
		assert.Equal(t, 0, tipManager.WeakTipCount())
	}

	// Message 13
	{
		createScenarioMessageWith1Input1Output("13", "12", "4", "O", "Y", []MessageID{messages["4"].ID()})
		tipManager.AddTip(messages["13"])
		assert.Equal(t, 8, tipManager.StrongTipCount())
		assert.Equal(t, 0, tipManager.WeakTipCount())
	}

	// Message 14
	{
		createScenarioMessageWith1Input1Output("14", "13", "4", "P", "Z", []MessageID{messages["4"].ID()})
		tipManager.AddTip(messages["14"])
		assert.Equal(t, 9, tipManager.StrongTipCount())
		assert.Equal(t, 0, tipManager.WeakTipCount())
	}

	// Message 15
	{
		messages["15"] = newTestParentsDataMessage("data", []MessageID{messages["10"].ID(), messages["11"].ID()}, []MessageID{})

		storeBookLikeMessage(t, tangle, messages["15"])

		tipManager.AddTip(messages["15"])
		assert.Equal(t, 8, tipManager.StrongTipCount())
		assert.Equal(t, 0, tipManager.WeakTipCount())
	}

	// Message 16
	{
		messages["16"] = newTestParentsDataMessage("data", []MessageID{messages["10"].ID(), messages["11"].ID(), messages["14"].ID()}, []MessageID{})

		storeBookLikeMessage(t, tangle, messages["16"])

		tipManager.AddTip(messages["16"])
		assert.Equal(t, 8, tipManager.StrongTipCount())
		assert.Equal(t, 0, tipManager.WeakTipCount())
	}
	// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////

	// now we can finally start the actual tests
	inputs["B"] = ledgerstate.NewUTXOInput(ledgerstate.NewOutputID(transactions["1"].ID(), selectIndex(transactions["1"], wallets["B"])))
	outputsByID[inputs["B"].ReferencedOutputID()] = ledgerstate.NewOutputs(outputs["B"])[0]
	inputs["Q"] = ledgerstate.NewUTXOInput(ledgerstate.NewOutputID(transactions["5"].ID(), selectIndex(transactions["5"], wallets["Q"])))
	outputsByID[inputs["Q"].ReferencedOutputID()] = ledgerstate.NewOutputs(outputs["Q"])[0]
	inputs["R"] = ledgerstate.NewUTXOInput(ledgerstate.NewOutputID(transactions["6"].ID(), selectIndex(transactions["6"], wallets["R"])))
	outputsByID[inputs["R"].ReferencedOutputID()] = ledgerstate.NewOutputs(outputs["R"])[0]
	inputs["S"] = ledgerstate.NewUTXOInput(ledgerstate.NewOutputID(transactions["7"].ID(), selectIndex(transactions["7"], wallets["S"])))
	outputsByID[inputs["S"].ReferencedOutputID()] = ledgerstate.NewOutputs(outputs["S"])[0]
	inputs["T"] = ledgerstate.NewUTXOInput(ledgerstate.NewOutputID(transactions["8"].ID(), selectIndex(transactions["8"], wallets["T"])))
	outputsByID[inputs["T"].ReferencedOutputID()] = ledgerstate.NewOutputs(outputs["T"])[0]
	inputs["U"] = ledgerstate.NewUTXOInput(ledgerstate.NewOutputID(transactions["9"].ID(), selectIndex(transactions["9"], wallets["U"])))
	outputsByID[inputs["U"].ReferencedOutputID()] = ledgerstate.NewOutputs(outputs["U"])[0]
	inputs["V"] = ledgerstate.NewUTXOInput(ledgerstate.NewOutputID(transactions["10"].ID(), selectIndex(transactions["10"], wallets["V"])))
	outputsByID[inputs["V"].ReferencedOutputID()] = ledgerstate.NewOutputs(outputs["V"])[0]
	inputs["X"] = ledgerstate.NewUTXOInput(ledgerstate.NewOutputID(transactions["11"].ID(), selectIndex(transactions["11"], wallets["X"])))
	outputsByID[inputs["X"].ReferencedOutputID()] = ledgerstate.NewOutputs(outputs["X"])[0]
	inputs["Y"] = ledgerstate.NewUTXOInput(ledgerstate.NewOutputID(transactions["12"].ID(), selectIndex(transactions["12"], wallets["Y"])))
	outputsByID[inputs["Y"].ReferencedOutputID()] = ledgerstate.NewOutputs(outputs["Y"])[0]
	inputs["Z"] = ledgerstate.NewUTXOInput(ledgerstate.NewOutputID(transactions["13"].ID(), selectIndex(transactions["13"], wallets["Z"])))
	outputsByID[inputs["Z"].ReferencedOutputID()] = ledgerstate.NewOutputs(outputs["Z"])[0]

	// Message 17
	{
		outputs["OUT"] = ledgerstate.NewSigLockedSingleOutput(8, wallets["OUT"].address)

		transactions["14"] = makeTransaction(
			ledgerstate.NewInputs(
				inputs["Q"],
				inputs["R"],
				inputs["S"],
				inputs["T"],
				inputs["U"],
				inputs["V"],
				inputs["X"],
				inputs["Y"],
			),
			ledgerstate.NewOutputs(
				outputs["OUT"],
			),
			outputsByID,
			walletsByAddress,
		)

		strongParents, weakParents, err := tipManager.Tips(transactions["14"], 2, 2)
		assert.NoError(t, err)
		assert.ElementsMatch(t, strongParents, []MessageID{
			messages["6"].ID(),
			messages["7"].ID(),
			messages["8"].ID(),
			messages["9"].ID(),
			messages["10"].ID(),
			messages["11"].ID(),
			messages["12"].ID(),
			messages["13"].ID(),
		})
		assert.Len(t, weakParents, 0)
	}

	// Message 18
	{
		outputs["OUT"] = ledgerstate.NewSigLockedSingleOutput(6, wallets["OUT"].address)

		transactions["15"] = makeTransaction(
			ledgerstate.NewInputs(
				inputs["Q"],
				inputs["R"],
				inputs["S"],
				inputs["T"],
				inputs["U"],
				inputs["V"],
			),
			ledgerstate.NewOutputs(
				outputs["OUT"],
			),
			outputsByID,
			walletsByAddress,
		)

		strongParents, weakParents, err := tipManager.Tips(transactions["15"], 2, 2)
		assert.NoError(t, err)
		// there are possible parents to be selected, however, since the directly referenced messages are tips as well
		// there is a chance that these are doubly selected, resulting 6 to 8 parents
		assert.GreaterOrEqual(t, len(strongParents), 6)
		assert.LessOrEqual(t, len(strongParents), 8)
		assert.Contains(t, strongParents,
			messages["6"].ID(),
			messages["7"].ID(),
			messages["8"].ID(),
			messages["9"].ID(),
			messages["10"].ID(),
			messages["11"].ID(),
		)
		assert.Len(t, weakParents, 0)
	}

	// Message 19
	{
		outputs["OUT"] = ledgerstate.NewSigLockedSingleOutput(3, wallets["OUT"].address)

		transactions["16"] = makeTransaction(
			ledgerstate.NewInputs(
				inputs["B"],
				inputs["V"],
				inputs["Z"],
			),
			ledgerstate.NewOutputs(
				outputs["OUT"],
			),
			outputsByID,
			walletsByAddress,
		)

		strongParents, weakParents, err := tipManager.Tips(transactions["16"], 2, 2)
		assert.NoError(t, err)

		// we reference 11, 14 directly. 1 is too old and should not be directly referenced
		assert.GreaterOrEqual(t, len(strongParents), 4)
		assert.LessOrEqual(t, len(strongParents), 8)
		assert.Contains(t, strongParents,
			messages["11"].ID(),
			messages["14"].ID(),
		)
		assert.NotContains(t, strongParents,
			messages["1"].ID(),
		)
		assert.Len(t, weakParents, 0)
	}

	// Message 20
	{
		outputs["OUT"] = ledgerstate.NewSigLockedSingleOutput(9, wallets["OUT"].address)

		transactions["17"] = makeTransaction(
			ledgerstate.NewInputs(
				inputs["Q"],
				inputs["R"],
				inputs["S"],
				inputs["T"],
				inputs["U"],
				inputs["V"],
				inputs["X"],
				inputs["Y"],
				inputs["Z"],
			),
			ledgerstate.NewOutputs(
				outputs["OUT"],
			),
			outputsByID,
			walletsByAddress,
		)

		strongParents, weakParents, err := tipManager.Tips(transactions["17"], 2, 2)
		assert.NoError(t, err)

		// there are 9 inputs to be directly referenced -> we need to reference them via tips (8 tips available)
		// due to the tips' nature they contain all transactions in the past cone
		assert.Len(t, strongParents, 8)
		assert.Len(t, weakParents, 0)
	}
}

func storeBookLikeMessage(t *testing.T, tangle *Tangle, message *Message) {
	// we need to store and book transactions so that we also have attachments of transactions available
	tangle.Storage.StoreMessage(message)
	// TODO: CheckTransaction should be removed here once the booker passes on errors
	if message.payload.Type() == ledgerstate.TransactionType {
		err := tangle.LedgerState.UTXODAG.CheckTransaction(message.payload.(*ledgerstate.Transaction))
		require.NoError(t, err)
	}
	err := tangle.Booker.BookMessage(message.ID())
	require.NoError(t, err)

	tangle.Storage.MessageMetadata(message.ID()).Consume(func(messageMetadata *MessageMetadata) {
		// make sure that everything was booked into master branch
		require.True(t, messageMetadata.booked)
		messageBranchID, err := tangle.Booker.MessageBranchID(message.ID())
		assert.NoError(t, err)
		require.Equal(t, ledgerstate.MasterBranchID, messageBranchID)

		messageMetadata.SetEligible(true)
	})
}

func createAndStoreEligibleTestParentsDataMessageInMasterBranch(tangle *Tangle, strongParents, weakParents MessageIDs) (message *Message) {
	message = newTestParentsDataMessage("testmessage", strongParents, weakParents)
	tangle.Storage.StoreMessage(message)
	message.setMessageMetadata(tangle, true, ledgerstate.MasterBranchID)

	return
}

func createAndStoreEligibleTestParentsDataMessageInInvalidBranch(tangle *Tangle, strongParents, weakParents MessageIDs) (message *Message) {
	message = newTestParentsDataMessage("testmessage", strongParents, weakParents)
	tangle.Storage.StoreMessage(message)
	message.setMessageMetadata(tangle, true, ledgerstate.InvalidBranchID)

	return
}

func (m *Message) setMessageMetadata(tangle *Tangle, eligible bool, branchID ledgerstate.BranchID) {
	tangle.Storage.MessageMetadata(m.ID()).Consume(func(messageMetadata *MessageMetadata) {
		messageMetadata.SetEligible(eligible)
		messageMetadata.SetBranchID(branchID)
	})
}

type mockConsensusProvider struct {
	opinionLikedFnc func(transactionID ledgerstate.TransactionID) bool
}

func (m *mockConsensusProvider) Init(*Tangle) {}

func (m *mockConsensusProvider) Setup() {}

func (m *mockConsensusProvider) TransactionLiked(transactionID ledgerstate.TransactionID) (liked bool) {
	return m.opinionLikedFnc(transactionID)
}

func (m *mockConsensusProvider) SetTransactionLiked(transactionID ledgerstate.TransactionID, liked bool) (modified bool) {
	return
}

func (m *mockConsensusProvider) Shutdown() {}
