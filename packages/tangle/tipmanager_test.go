package tangle

import (
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/identity"
	"github.com/iotaledger/hive.go/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/markers"
)

func TestTipManager_AddTip(t *testing.T) {
	tangle := NewTestTangle()
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
	// set up scenario (images/tipmanager-add-tips.png)
	messages := make(map[string]*Message)

	// Message 1
	{
		messages["1"] = createAndStoreParentsDataMessageInMasterBranch(tangle, []MessageID{EmptyMessageID}, []MessageID{})
		tipManager.AddTip(messages["1"])

		assert.Equal(t, 1, tipManager.TipCount())
		assert.Contains(t, tipManager.tips.Keys(), messages["1"].ID())
	}

	// Message 2
	{
		messages["2"] = createAndStoreParentsDataMessageInMasterBranch(tangle, []MessageID{EmptyMessageID}, []MessageID{})
		tipManager.AddTip(messages["2"])

		assert.Equal(t, 2, tipManager.TipCount())
		assert.Contains(t, tipManager.tips.Keys(), messages["1"].ID(), messages["2"].ID())
	}

	// Message 3
	{
		messages["3"] = createAndStoreParentsDataMessageInMasterBranch(tangle, []MessageID{EmptyMessageID, messages["1"].ID(), messages["2"].ID()}, []MessageID{})
		tipManager.AddTip(messages["3"])

		assert.Equal(t, 1, tipManager.TipCount())
		assert.Contains(t, tipManager.tips.Keys(), messages["3"].ID())
	}
}

func TestTipManager_DataMessageTips(t *testing.T) {
	tangle := NewTestTangle()
	defer tangle.Shutdown()
	tipManager := tangle.TipManager

	// set up scenario (images/tipmanager-DataMessageTips-test.png)
	messages := make(map[string]*Message)

	// without any tip -> genesis
	{
		parents, err := tipManager.Tips(nil, 2)
		assert.NoError(t, err)
		assert.Len(t, parents, 1)
		assert.Contains(t, parents, EmptyMessageID)
	}

	// without any count -> 1 tip, in this case genesis
	{
		parents, err := tipManager.Tips(nil, 0)
		assert.NoError(t, err)
		assert.Len(t, parents, 1)
		assert.Contains(t, parents, EmptyMessageID)
	}

	// Message 1
	{
		messages["1"] = createAndStoreParentsDataMessageInMasterBranch(tangle, []MessageID{EmptyMessageID}, []MessageID{})
		tipManager.AddTip(messages["1"])
		tangle.TimeManager.updateTime(messages["1"].ID())

		assert.Equal(t, 1, tipManager.TipCount())
		assert.Contains(t, tipManager.tips.Keys(), messages["1"].ID())

		parents, err := tipManager.Tips(nil, 2)
		assert.NoError(t, err)
		assert.Len(t, parents, 1)
		assert.Contains(t, parents, messages["1"].ID())
	}

	// Message 2
	{
		messages["2"] = createAndStoreParentsDataMessageInMasterBranch(tangle, []MessageID{EmptyMessageID}, []MessageID{})
		tipManager.AddTip(messages["2"])

		assert.Equal(t, 2, tipManager.TipCount())
		assert.Contains(t, tipManager.tips.Keys(), messages["1"].ID(), messages["2"].ID())

		parents, err := tipManager.Tips(nil, 3)
		assert.NoError(t, err)
		assert.Len(t, parents, 2)
		assert.Contains(t, parents, messages["1"].ID(), messages["2"].ID())
	}

	// Message 3
	{
		messages["3"] = createAndStoreParentsDataMessageInMasterBranch(tangle, []MessageID{messages["1"].ID(), messages["2"].ID()}, []MessageID{})
		tipManager.AddTip(messages["3"])

		assert.Equal(t, 1, tipManager.TipCount())
		assert.Contains(t, tipManager.tips.Keys(), messages["3"].ID())

		parents, err := tipManager.Tips(nil, 2)
		assert.NoError(t, err)
		assert.Len(t, parents, 1)
		assert.Contains(t, parents, messages["3"].ID())
	}

	// Add Message 4-8
	{
		tips := make([]MessageID, 0, 9)
		tips = append(tips, messages["3"].ID())
		for count, n := range []int{4, 5, 6, 7, 8} {
			nString := strconv.Itoa(n)
			messages[nString] = createAndStoreParentsDataMessageInMasterBranch(tangle, []MessageID{messages["1"].ID()}, []MessageID{})
			tipManager.AddTip(messages[nString])
			tips = append(tips, messages[nString].ID())

			assert.Equalf(t, count+2, tipManager.TipCount(), "TipCount does not match after adding Message %d", n)
			assert.ElementsMatchf(t, tipManager.tips.Keys(), tips, "Elements in strongTips do not match after adding Message %d", n)
			assert.Contains(t, tipManager.tips.Keys(), messages["3"].ID())
		}
	}

	// now we have 6 tips
	// Tips(4) -> 4
	{
		parents, err := tipManager.Tips(nil, 4)
		assert.NoError(t, err)
		assert.Len(t, parents, 4)
	}
	// Tips(8) -> 6
	{
		parents, err := tipManager.Tips(nil, 8)
		assert.NoError(t, err)
		assert.Len(t, parents, 6)
	}
	// Tips(0) -> 1
	{
		parents, err := tipManager.Tips(nil, 0)
		assert.NoError(t, err)
		assert.Len(t, parents, 1)
	}
}

func TestTipManager_TransactionTips(t *testing.T) {
	// set up scenario (images/tipmanager-TransactionTips-test.png)
	tangle := NewTestTangle()
	defer tangle.Shutdown()
	tipManager := tangle.TipManager
	confirmedMessageIDs := &MessageIDs{}
	tangle.ConfirmationOracle = &MockConfirmationOracleTipManagerTest{confirmedMessageIDs: confirmedMessageIDs}

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
		messages["1"] = newTestParentsPayloadWithTimestamp(transactions["1"], ParentMessageIDs{
			StrongParentType: MessageIDsSlice{EmptyMessageID}.ToMessageIDs(),
		}, issueTime)

		storeAndBookMessage(t, tangle, messages["1"])

		tipManager.AddTip(messages["1"])
		assert.Equal(t, 0, tipManager.TipCount())

		// mark this message as confirmed
		*confirmedMessageIDs = MessageIDs{messages["1"].ID(): types.Void}
	}

	// Message 2
	{
		inputs["G2"] = ledgerstate.NewUTXOInput(ledgerstate.NewOutputID(genesisTransaction.ID(), g2))
		outputs["D"] = ledgerstate.NewSigLockedSingleOutput(6, wallets["D"].address)
		outputs["E"] = ledgerstate.NewSigLockedSingleOutput(1, wallets["E"].address)
		outputs["F"] = ledgerstate.NewSigLockedSingleOutput(1, wallets["F"].address)

		transactions["2"] = makeTransaction(ledgerstate.NewInputs(inputs["G2"]), ledgerstate.NewOutputs(outputs["D"], outputs["E"], outputs["F"]), outputsByID, walletsByAddress, wallets["G2"])
		messages["2"] = newTestParentsPayloadMessage(transactions["2"], ParentMessageIDs{
			StrongParentType: MessageIDsSlice{EmptyMessageID}.ToMessageIDs(),
		})

		storeAndBookMessage(t, tangle, messages["2"])

		tipManager.AddTip(messages["2"])
		assert.Equal(t, 1, tipManager.TipCount())

		// use this message to set TangleTime
		tangle.TimeManager.updateTime(messages["2"].ID())
	}

	// Message 3
	{
		inputs["A"] = ledgerstate.NewUTXOInput(ledgerstate.NewOutputID(transactions["1"].ID(), selectIndex(transactions["1"], wallets["A"])))
		outputsByID[inputs["A"].ReferencedOutputID()] = ledgerstate.NewOutputs(outputs["A"])[0]
		outputs["H"] = ledgerstate.NewSigLockedSingleOutput(1, wallets["H"].address)
		outputs["I"] = ledgerstate.NewSigLockedSingleOutput(1, wallets["I"].address)
		outputs["J"] = ledgerstate.NewSigLockedSingleOutput(1, wallets["J"].address)

		transactions["3"] = makeTransaction(ledgerstate.NewInputs(inputs["A"]), ledgerstate.NewOutputs(outputs["H"], outputs["I"], outputs["J"]), outputsByID, walletsByAddress)
		messages["3"] = newTestParentsPayloadMessage(transactions["3"], ParentMessageIDs{
			StrongParentType: MessageIDsSlice{messages["1"].ID(), EmptyMessageID}.ToMessageIDs(),
		})

		storeAndBookMessage(t, tangle, messages["3"])

		tipManager.AddTip(messages["3"])
		assert.Equal(t, 2, tipManager.TipCount())
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
		messages["4"] = newTestParentsPayloadMessage(transactions["4"], ParentMessageIDs{
			StrongParentType: MessageIDsSlice{messages["2"].ID(), EmptyMessageID}.ToMessageIDs(),
		})

		storeAndBookMessage(t, tangle, messages["4"])

		tipManager.AddTip(messages["4"])
		assert.Equal(t, 2, tipManager.TipCount())
	}

	// Message 5
	{
		messages["5"] = newTestParentsDataMessage("data", ParentMessageIDs{
			StrongParentType: MessageIDsSlice{messages["1"].ID(), EmptyMessageID}.ToMessageIDs(),
		})

		storeAndBookMessage(t, tangle, messages["5"])

		tipManager.AddTip(messages["5"])
		assert.Equal(t, 3, tipManager.TipCount())
	}

	createScenarioMessageWith1Input1Output := func(messageStringID, transactionStringID, consumedTransactionStringID, inputStringID, outputStringID string, strongParents []MessageID) {
		inputs[inputStringID] = ledgerstate.NewUTXOInput(ledgerstate.NewOutputID(transactions[consumedTransactionStringID].ID(), selectIndex(transactions[consumedTransactionStringID], wallets[inputStringID])))
		outputsByID[inputs[inputStringID].ReferencedOutputID()] = ledgerstate.NewOutputs(outputs[inputStringID])[0]
		outputs[outputStringID] = ledgerstate.NewSigLockedSingleOutput(1, wallets[outputStringID].address)

		transactions[transactionStringID] = makeTransaction(ledgerstate.NewInputs(inputs[inputStringID]), ledgerstate.NewOutputs(outputs[outputStringID]), outputsByID, walletsByAddress)
		messages[messageStringID] = newTestParentsPayloadMessage(transactions[transactionStringID], ParentMessageIDs{
			StrongParentType: MessageIDsSlice(strongParents).ToMessageIDs(),
		})

		storeAndBookMessage(t, tangle, messages[messageStringID])
	}

	// Message 6
	{
		createScenarioMessageWith1Input1Output("6", "5", "3", "H", "Q", []MessageID{messages["3"].ID(), messages["5"].ID()})
		tipManager.AddTip(messages["6"])
		assert.Equal(t, 2, tipManager.TipCount())
	}

	// Message 7
	{
		createScenarioMessageWith1Input1Output("7", "6", "3", "I", "R", []MessageID{messages["3"].ID(), messages["5"].ID()})
		tipManager.AddTip(messages["7"])
		assert.Equal(t, 3, tipManager.TipCount())
	}

	// Message 8
	{
		createScenarioMessageWith1Input1Output("8", "7", "3", "J", "S", []MessageID{messages["3"].ID(), messages["5"].ID()})
		tipManager.AddTip(messages["8"])
		assert.Equal(t, 4, tipManager.TipCount())
	}

	// Message 9
	{
		createScenarioMessageWith1Input1Output("9", "8", "4", "K", "T", []MessageID{messages["4"].ID()})
		tipManager.AddTip(messages["9"])
		assert.Equal(t, 4, tipManager.TipCount())
	}

	// Message 10
	{
		createScenarioMessageWith1Input1Output("10", "9", "4", "L", "U", []MessageID{messages["2"].ID(), messages["4"].ID()})
		tipManager.AddTip(messages["10"])
		assert.Equal(t, 5, tipManager.TipCount())
	}

	// Message 11
	{
		createScenarioMessageWith1Input1Output("11", "10", "4", "M", "V", []MessageID{messages["2"].ID(), messages["4"].ID()})
		tipManager.AddTip(messages["11"])
		assert.Equal(t, 6, tipManager.TipCount())
	}

	// Message 12
	{
		createScenarioMessageWith1Input1Output("12", "11", "4", "N", "X", []MessageID{messages["3"].ID(), messages["4"].ID()})
		tipManager.AddTip(messages["12"])
		assert.Equal(t, 7, tipManager.TipCount())
	}

	// Message 13
	{
		createScenarioMessageWith1Input1Output("13", "12", "4", "O", "Y", []MessageID{messages["4"].ID()})
		tipManager.AddTip(messages["13"])
		assert.Equal(t, 8, tipManager.TipCount())
	}

	// Message 14
	{
		createScenarioMessageWith1Input1Output("14", "13", "4", "P", "Z", []MessageID{messages["4"].ID()})
		tipManager.AddTip(messages["14"])
		assert.Equal(t, 9, tipManager.TipCount())
	}

	// Message 15
	{
		messages["15"] = newTestParentsDataMessage("data", ParentMessageIDs{
			StrongParentType: MessageIDsSlice{messages["10"].ID(), messages["11"].ID()}.ToMessageIDs(),
		})

		storeAndBookMessage(t, tangle, messages["15"])

		tipManager.AddTip(messages["15"])
		assert.Equal(t, 8, tipManager.TipCount())
	}

	// Message 16
	{
		messages["16"] = newTestParentsDataMessage("data", ParentMessageIDs{
			StrongParentType: MessageIDsSlice{messages["10"].ID(), messages["11"].ID(), messages["14"].ID()}.ToMessageIDs(),
		})

		storeAndBookMessage(t, tangle, messages["16"])

		tipManager.AddTip(messages["16"])
		assert.Equal(t, 8, tipManager.TipCount())
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

		parents, err := tipManager.Tips(transactions["14"], 4)
		assert.NoError(t, err)
		assert.ElementsMatch(t, parents, []MessageID{
			messages["6"].ID(),
			messages["7"].ID(),
			messages["8"].ID(),
			messages["9"].ID(),
			messages["10"].ID(),
			messages["11"].ID(),
			messages["12"].ID(),
			messages["13"].ID(),
		})
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

		parents, err := tipManager.Tips(transactions["15"], 4)
		assert.NoError(t, err)
		// there are possible parents to be selected, however, since the directly referenced messages are tips as well
		// there is a chance that these are doubly selected, resulting 6 to 8 parents
		assert.GreaterOrEqual(t, len(parents), 6)
		assert.LessOrEqual(t, len(parents), 8)
		assert.Contains(t, parents,
			messages["6"].ID(),
			messages["7"].ID(),
			messages["8"].ID(),
			messages["9"].ID(),
			messages["10"].ID(),
			messages["11"].ID(),
		)
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

		parents, err := tipManager.Tips(transactions["16"], 4)
		assert.NoError(t, err)

		// we reference 11, 14 directly. 1 is too old and should not be directly referenced
		assert.GreaterOrEqual(t, len(parents), 4)
		assert.LessOrEqual(t, len(parents), 8)
		assert.Contains(t, parents,
			messages["11"].ID(),
			messages["14"].ID(),
		)
		assert.NotContains(t, parents,
			messages["1"].ID(),
		)
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

		parents, err := tipManager.Tips(transactions["17"], 4)
		assert.NoError(t, err)

		// there are 9 inputs to be directly referenced -> we need to reference them via tips (8 tips available)
		// due to the tips' nature they contain all transactions in the past cone
		assert.Len(t, parents, 8)
	}
}

// Test based on packages/tangle/images/TSC_scenario.png
func TestTipManager_TimeSinceConfirmation(t *testing.T) {
	tangle := NewTestTangle()
	tangle.Booker.MarkersManager.Manager = markers.NewManager(markers.WithCacheTime(0), markers.WithMaxPastMarkerDistance(10))

	defer tangle.Shutdown()
	tipManager := tangle.TipManager

	testFramework := NewMessageTestFramework(
		tangle,
	)

	tangle.Setup()
	var lastMsgAlias string

	// SEQUENCE 0
	{
		testFramework.CreateMessage("Marker-0/1", WithStrongParents("Genesis"), WithIssuingTime(time.Now().Add(-9*time.Minute)))
		testFramework.IssueMessages("Marker-0/1").WaitMessagesBooked()
		testFramework.PreventNewMarkers(true)
		lastMsgAlias = issueMessages(testFramework, "0/1-preTSC", 3, []string{"Marker-0/1"}, time.Minute*8)
		lastMsgAlias = issueMessages(testFramework, "0/1-postTSC", 3, []string{lastMsgAlias}, time.Minute)
		testFramework.PreventNewMarkers(false)
		testFramework.CreateMessage("Marker-0/2", WithStrongParents(lastMsgAlias))
		testFramework.IssueMessages("Marker-0/2").WaitMessagesBooked()
		testFramework.PreventNewMarkers(true)
		lastMsgAlias = issueMessages(testFramework, "0/2", 5, []string{"Marker-0/2"}, 0)
		testFramework.PreventNewMarkers(false)
		testFramework.CreateMessage("Marker-0/3", WithStrongParents(lastMsgAlias))
		testFramework.IssueMessages("Marker-0/3").WaitMessagesBooked()
		testFramework.PreventNewMarkers(true)
		lastMsgAlias = issueMessages(testFramework, "0/3", 5, []string{"Marker-0/3"}, 0)
		testFramework.PreventNewMarkers(false)
		testFramework.CreateMessage("Marker-0/4", WithStrongParents(lastMsgAlias))
		testFramework.IssueMessages("Marker-0/4").WaitMessagesBooked()
		testFramework.PreventNewMarkers(true)
		lastMsgAlias = issueMessages(testFramework, "0/4", 5, []string{"Marker-0/4"}, 0)
		testFramework.PreventNewMarkers(false)

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Marker-0/1":    markers.NewMarkers(markers.NewMarker(0, 1)),
			"0/1-preTSC_0":  markers.NewMarkers(markers.NewMarker(0, 1)),
			"0/1-preTSC_1":  markers.NewMarkers(markers.NewMarker(0, 1)),
			"0/1-preTSC_2":  markers.NewMarkers(markers.NewMarker(0, 1)),
			"0/1-postTSC_0": markers.NewMarkers(markers.NewMarker(0, 1)),
			"0/1-postTSC_1": markers.NewMarkers(markers.NewMarker(0, 1)),
			"0/1-postTSC_2": markers.NewMarkers(markers.NewMarker(0, 1)),
			"Marker-0/2":    markers.NewMarkers(markers.NewMarker(0, 2)),
			"Marker-0/3":    markers.NewMarkers(markers.NewMarker(0, 3)),
			"Marker-0/4":    markers.NewMarkers(markers.NewMarker(0, 4)),
		})
	}
	// SEQUENCE 1
	{
		testFramework.PreventNewMarkers(true)
		lastMsgAlias = issueMessages(testFramework, "0/1-preTSCSeq1", 3, []string{"Marker-0/1"}, time.Minute*6)
		lastMsgAlias = issueMessages(testFramework, "0/1-postTSCSeq1", 6, []string{lastMsgAlias}, time.Minute*4)
		testFramework.PreventNewMarkers(false)
		testFramework.CreateMessage("Marker-1/2", WithStrongParents(lastMsgAlias), WithIssuingTime(time.Now().Add(-3*time.Minute)))
		testFramework.IssueMessages("Marker-1/2").WaitMessagesBooked()
		testFramework.PreventNewMarkers(true)
		lastMsgAlias = issueMessages(testFramework, "1/2", 5, []string{"Marker-1/2"}, 0)
		testFramework.PreventNewMarkers(false)
		testFramework.CreateMessage("Marker-1/3", WithStrongParents(lastMsgAlias))
		testFramework.IssueMessages("Marker-1/3").WaitMessagesBooked()
		testFramework.PreventNewMarkers(true)
		lastMsgAlias = issueMessages(testFramework, "1/3", 5, []string{"Marker-1/3"}, 0)
		testFramework.PreventNewMarkers(false)

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"0/1-preTSCSeq1_0":  markers.NewMarkers(markers.NewMarker(0, 1)),
			"0/1-preTSCSeq1_1":  markers.NewMarkers(markers.NewMarker(0, 1)),
			"0/1-preTSCSeq1_2":  markers.NewMarkers(markers.NewMarker(0, 1)),
			"0/1-postTSCSeq1_0": markers.NewMarkers(markers.NewMarker(0, 1)),
			"0/1-postTSCSeq1_1": markers.NewMarkers(markers.NewMarker(0, 1)),
			"0/1-postTSCSeq1_2": markers.NewMarkers(markers.NewMarker(0, 1)),
			"0/1-postTSCSeq1_3": markers.NewMarkers(markers.NewMarker(0, 1)),
			"0/1-postTSCSeq1_4": markers.NewMarkers(markers.NewMarker(0, 1)),
			"0/1-postTSCSeq1_5": markers.NewMarkers(markers.NewMarker(0, 1)),
			"Marker-1/2":        markers.NewMarkers(markers.NewMarker(1, 2)),
			"1/2_0":             markers.NewMarkers(markers.NewMarker(1, 2)),
			"1/2_1":             markers.NewMarkers(markers.NewMarker(1, 2)),
			"1/2_2":             markers.NewMarkers(markers.NewMarker(1, 2)),
			"1/2_3":             markers.NewMarkers(markers.NewMarker(1, 2)),
			"1/2_4":             markers.NewMarkers(markers.NewMarker(1, 2)),
			"Marker-1/3":        markers.NewMarkers(markers.NewMarker(1, 3)),
			"1/3_0":             markers.NewMarkers(markers.NewMarker(1, 3)),
			"1/3_1":             markers.NewMarkers(markers.NewMarker(1, 3)),
			"1/3_2":             markers.NewMarkers(markers.NewMarker(1, 3)),
			"1/3_3":             markers.NewMarkers(markers.NewMarker(1, 3)),
			"1/3_4":             markers.NewMarkers(markers.NewMarker(1, 3)),
		})
	}

	// SEQUENCE 2
	{
		testFramework.PreventNewMarkers(true)
		lastMsgAlias = issueMessages(testFramework, "0/1-preTSCSeq2", 3, []string{"Marker-0/1"}, time.Minute*6)
		lastMsgAlias = issueMessages(testFramework, "0/1-postTSCSeq2", 6, []string{lastMsgAlias}, time.Minute*4)
		testFramework.PreventNewMarkers(false)
		testFramework.CreateMessage("Marker-2/2", WithStrongParents(lastMsgAlias), WithIssuingTime(time.Now().Add(-3*time.Minute)))
		testFramework.IssueMessages("Marker-2/2").WaitMessagesBooked()
		testFramework.PreventNewMarkers(true)
		lastMsgAlias = issueMessages(testFramework, "2/2", 5, []string{"Marker-2/2"}, 0)
		testFramework.PreventNewMarkers(false)
		testFramework.CreateMessage("Marker-2/3", WithStrongParents(lastMsgAlias))
		testFramework.IssueMessages("Marker-2/3").WaitMessagesBooked()
		testFramework.PreventNewMarkers(true)
		lastMsgAlias = issueMessages(testFramework, "2/3", 5, []string{"Marker-2/3"}, 0)
		testFramework.PreventNewMarkers(false)

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"0/1-preTSCSeq2_0":  markers.NewMarkers(markers.NewMarker(0, 1)),
			"0/1-preTSCSeq2_1":  markers.NewMarkers(markers.NewMarker(0, 1)),
			"0/1-preTSCSeq2_2":  markers.NewMarkers(markers.NewMarker(0, 1)),
			"0/1-postTSCSeq2_0": markers.NewMarkers(markers.NewMarker(0, 1)),
			"0/1-postTSCSeq2_1": markers.NewMarkers(markers.NewMarker(0, 1)),
			"0/1-postTSCSeq2_2": markers.NewMarkers(markers.NewMarker(0, 1)),
			"0/1-postTSCSeq2_3": markers.NewMarkers(markers.NewMarker(0, 1)),
			"0/1-postTSCSeq2_4": markers.NewMarkers(markers.NewMarker(0, 1)),
			"0/1-postTSCSeq2_5": markers.NewMarkers(markers.NewMarker(0, 1)),
			"Marker-2/2":        markers.NewMarkers(markers.NewMarker(2, 2)),
			"2/2_0":             markers.NewMarkers(markers.NewMarker(2, 2)),
			"2/2_1":             markers.NewMarkers(markers.NewMarker(2, 2)),
			"2/2_2":             markers.NewMarkers(markers.NewMarker(2, 2)),
			"2/2_3":             markers.NewMarkers(markers.NewMarker(2, 2)),
			"2/2_4":             markers.NewMarkers(markers.NewMarker(2, 2)),
			"Marker-2/3":        markers.NewMarkers(markers.NewMarker(2, 3)),
			"2/3_0":             markers.NewMarkers(markers.NewMarker(2, 3)),
			"2/3_1":             markers.NewMarkers(markers.NewMarker(2, 3)),
			"2/3_2":             markers.NewMarkers(markers.NewMarker(2, 3)),
			"2/3_3":             markers.NewMarkers(markers.NewMarker(2, 3)),
			"2/3_4":             markers.NewMarkers(markers.NewMarker(2, 3)),
		})
	}

	// SEQUENCE 2 + 0
	{
		testFramework.CreateMessage("Marker-2/5", WithStrongParents("0/4_4", "2/3_4"))
		testFramework.IssueMessages("Marker-2/5").WaitMessagesBooked()
		testFramework.PreventNewMarkers(true)
		lastMsgAlias = issueMessages(testFramework, "2/5", 5, []string{"Marker-2/5"}, 0)
		testFramework.PreventNewMarkers(false)

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Marker-2/5": markers.NewMarkers(markers.NewMarker(2, 5)),
			"2/5_0":      markers.NewMarkers(markers.NewMarker(2, 5)),
			"2/5_1":      markers.NewMarkers(markers.NewMarker(2, 5)),
			"2/5_2":      markers.NewMarkers(markers.NewMarker(2, 5)),
			"2/5_3":      markers.NewMarkers(markers.NewMarker(2, 5)),
			"2/5_4":      markers.NewMarkers(markers.NewMarker(2, 5)),
		})
	}

	// SEQUENCE 3
	{
		testFramework.PreventNewMarkers(true)
		lastMsgAlias = issueMessages(testFramework, "0/1-postTSCSeq3", 6, []string{"0/1-preTSCSeq2_2"}, 0)
		testFramework.PreventNewMarkers(false)
		testFramework.CreateMessage("Marker-3/2", WithStrongParents(lastMsgAlias))
		testFramework.IssueMessages("Marker-3/2").WaitMessagesBooked()
		testFramework.PreventNewMarkers(true)
		lastMsgAlias = issueMessages(testFramework, "3/2", 5, []string{"Marker-3/2"}, 0)
		testFramework.PreventNewMarkers(false)

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"0/1-postTSCSeq3_0": markers.NewMarkers(markers.NewMarker(0, 1)),
			"0/1-postTSCSeq3_1": markers.NewMarkers(markers.NewMarker(0, 1)),
			"0/1-postTSCSeq3_2": markers.NewMarkers(markers.NewMarker(0, 1)),
			"0/1-postTSCSeq3_3": markers.NewMarkers(markers.NewMarker(0, 1)),
			"0/1-postTSCSeq3_4": markers.NewMarkers(markers.NewMarker(0, 1)),
			"Marker-3/2":        markers.NewMarkers(markers.NewMarker(3, 2)),
			"3/2_0":             markers.NewMarkers(markers.NewMarker(3, 2)),
			"3/2_1":             markers.NewMarkers(markers.NewMarker(3, 2)),
			"3/2_2":             markers.NewMarkers(markers.NewMarker(3, 2)),
			"3/2_3":             markers.NewMarkers(markers.NewMarker(3, 2)),
			"3/2_4":             markers.NewMarkers(markers.NewMarker(3, 2)),
		})
	}

	// SEQUENCE 2 + 0 (two past markers) -> SEQUENCE 4
	{
		testFramework.PreventNewMarkers(true)
		lastMsgAlias = issueMessages(testFramework, "2/3+0/4", 5, []string{"0/4_4", "2/3_4"}, 0)
		testFramework.CreateMessage("Marker-4/5", WithStrongParents(lastMsgAlias))
		testFramework.IssueMessages("Marker-4/5").WaitMessagesBooked()
		testFramework.PreventNewMarkers(false)

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"2/3+0/4_0":  markers.NewMarkers(markers.NewMarker(2, 3), markers.NewMarker(0, 4)),
			"2/3+0/4_1":  markers.NewMarkers(markers.NewMarker(2, 3), markers.NewMarker(0, 4)),
			"2/3+0/4_2":  markers.NewMarkers(markers.NewMarker(2, 3), markers.NewMarker(0, 4)),
			"2/3+0/4_3":  markers.NewMarkers(markers.NewMarker(2, 3), markers.NewMarker(0, 4)),
			"Marker-4/5": markers.NewMarkers(markers.NewMarker(4, 5)),
		})
	}
	// SEQUENCE 5
	{
		testFramework.PreventNewMarkers(true)
		lastMsgAlias = issueMessages(testFramework, "0/1-preTSCSeq5", 6, []string{"0/1-preTSCSeq2_2"}, time.Minute*6)
		testFramework.PreventNewMarkers(false)
		testFramework.CreateMessage("Marker-5/2", WithStrongParents(lastMsgAlias))
		testFramework.IssueMessages("Marker-5/2").WaitMessagesBooked()
		testFramework.PreventNewMarkers(true)
		lastMsgAlias = issueMessages(testFramework, "5/2", 5, []string{"Marker-5/2"}, 0)
		testFramework.PreventNewMarkers(false)

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"0/1-preTSCSeq5_0": markers.NewMarkers(markers.NewMarker(0, 1)),
			"0/1-preTSCSeq5_1": markers.NewMarkers(markers.NewMarker(0, 1)),
			"0/1-preTSCSeq5_2": markers.NewMarkers(markers.NewMarker(0, 1)),
			"0/1-preTSCSeq5_3": markers.NewMarkers(markers.NewMarker(0, 1)),
			"0/1-preTSCSeq5_4": markers.NewMarkers(markers.NewMarker(0, 1)),
			"Marker-5/2":       markers.NewMarkers(markers.NewMarker(5, 2)),
			"5/2_0":            markers.NewMarkers(markers.NewMarker(5, 2)),
			"5/2_1":            markers.NewMarkers(markers.NewMarker(5, 2)),
			"5/2_2":            markers.NewMarkers(markers.NewMarker(5, 2)),
			"5/2_3":            markers.NewMarkers(markers.NewMarker(5, 2)),
			"5/2_4":            markers.NewMarkers(markers.NewMarker(5, 2)),
		})
	}
	confirmedMessageIDsString := []string{"Marker-0/1", "0/1-preTSCSeq1_0", "0/1-preTSCSeq1_1", "0/1-preTSCSeq1_2", "0/1-postTSCSeq1_0", "0/1-postTSCSeq1_1", "0/1-postTSCSeq1_2", "0/1-postTSCSeq1_3", "0/1-postTSCSeq1_4", "0/1-postTSCSeq1_5", "Marker-1/2", "0/1-preTSCSeq2_0", "0/1-preTSCSeq2_1", "0/1-preTSCSeq2_2", "0/1-postTSCSeq2_0", "0/1-postTSCSeq2_1", "0/1-postTSCSeq2_2", "0/1-postTSCSeq2_3", "0/1-postTSCSeq2_4", "0/1-postTSCSeq2_5", "Marker-2/2", "2/2_0", "2/2_1", "2/2_2", "2/2_3", "2/2_4", "Marker-2/3"}
	confirmedMessageIDs := prepareConfirmedMessageIDs(testFramework, confirmedMessageIDsString)
	confirmedMarkers := markers.NewMarkers(markers.NewMarker(0, 1), markers.NewMarker(1, 2), markers.NewMarker(2, 3))

	tangle.ConfirmationOracle = &MockConfirmationOracleTipManagerTest{confirmedMessageIDs: confirmedMessageIDs, confirmedMarkers: confirmedMarkers}
	tangle.TimeManager.updateTime(testFramework.Message("Marker-2/3").ID())

	// case #1
	assert.False(t, tipManager.isPastConeTimestampCorrect(testFramework.Message("0/3_4").ID()))
	// case #2
	assert.True(t, tipManager.isPastConeTimestampCorrect(testFramework.Message("1/3_4").ID()))
	// case #3
	assert.True(t, tipManager.isPastConeTimestampCorrect(testFramework.Message("2/3_4").ID()))
	// case #4 (marker message)
	assert.True(t, tipManager.isPastConeTimestampCorrect(testFramework.Message("Marker-1/2").ID()))
	// case #5
	assert.False(t, tipManager.isPastConeTimestampCorrect(testFramework.Message("2/5_4").ID()))
	//case #6 (attach to unconfirmed message older than TSC)
	assert.False(t, tipManager.isPastConeTimestampCorrect(testFramework.Message("0/1-preTSC_2").ID()))
	//// case #7
	assert.True(t, tipManager.isPastConeTimestampCorrect(testFramework.Message("3/2_4").ID()))
	//case #8
	assert.False(t, tipManager.isPastConeTimestampCorrect(testFramework.Message("2/3+0/4_3").ID()))
	// case #9
	assert.False(t, tipManager.isPastConeTimestampCorrect(testFramework.Message("Marker-4/5").ID()))
	// case #10 (attach to confirmed message older than TSC)
	assert.True(t, tipManager.isPastConeTimestampCorrect(testFramework.Message("0/1-preTSCSeq2_2").ID()))
	// case #11
	assert.False(t, tipManager.isPastConeTimestampCorrect(testFramework.Message("5/2_4").ID()))
}

func prepareConfirmedMessageIDs(testFramework *MessageTestFramework, confirmedIDs []string) *MessageIDs {
	confirmedMessageIDs := &MessageIDs{}
	for _, id := range confirmedIDs {
		confirmedMessageIDs.Add(testFramework.Message(id).ID())
	}
	return confirmedMessageIDs
}

func issueMessages(testFramework *MessageTestFramework, msgPrefix string, msgCount int, parents []string, timestampOffset time.Duration) string {
	msgAlias := fmt.Sprintf("%s_%d", msgPrefix, 0)

	testFramework.CreateMessage(msgAlias, WithStrongParents(parents...), WithIssuingTime(time.Now().Add(-timestampOffset)))
	testFramework.IssueMessages(msgAlias).WaitMessagesBooked()

	for i := 1; i < msgCount; i++ {
		alias := fmt.Sprintf("%s_%d", msgPrefix, i)
		testFramework.CreateMessage(alias, WithStrongParents(msgAlias), WithIssuingTime(time.Now().Add(-timestampOffset)))
		testFramework.IssueMessages(alias).WaitMessagesBooked()

		msgAlias = alias
	}
	return msgAlias
}
func storeAndBookMessage(t *testing.T, tangle *Tangle, message *Message) {
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
		messageBranchIDs, err := tangle.Booker.MessageBranchIDs(message.ID())
		assert.NoError(t, err)
		require.Equal(t, ledgerstate.NewBranchIDs(ledgerstate.MasterBranchID), messageBranchIDs)
	})
}

func createAndStoreParentsDataMessageInMasterBranch(tangle *Tangle, strongParents, weakParents MessageIDsSlice) (message *Message) {
	message = newTestParentsDataMessage("testmessage", ParentMessageIDs{
		StrongParentType: strongParents.ToMessageIDs(),
		WeakParentType:   weakParents.ToMessageIDs(),
	})
	tangle.Storage.StoreMessage(message)

	return
}

type MockConfirmationOracleTipManagerTest struct {
	confirmedMessageIDs *MessageIDs
	confirmedMarkers    *markers.Markers

	MockConfirmationOracle
}

// IsMessageConfirmed mocks its interface function.
func (m *MockConfirmationOracleTipManagerTest) IsMessageConfirmed(msgID MessageID) bool {
	return containsMessageID(*m.confirmedMessageIDs, msgID)
}

// FirstUnconfirmedMarkerIndex mocks its interface function.
func (m *MockConfirmationOracleTipManagerTest) FirstUnconfirmedMarkerIndex(sequenceID markers.SequenceID) (unconfirmedMarkerIndex markers.Index) {
	confirmedMarkerIndex, exists := m.confirmedMarkers.Get(sequenceID)
	if exists {
		return confirmedMarkerIndex + 1
	}
	return 0
}

// IsMessageConfirmed mocks its interface function.
func (m *MockConfirmationOracleTipManagerTest) IsMarkerConfirmed(marker *markers.Marker) bool {
	confirmedMarkerIndex, exists := m.confirmedMarkers.Get(marker.SequenceID())
	if !exists {
		return false
	}
	return marker.Index() <= confirmedMarkerIndex
}
func containsMessageID(s MessageIDs, e MessageID) bool {
	for a := range s {
		if a == e {
			return true
		}
	}
	return false
}
