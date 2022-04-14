package tangle

import (
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/identity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/markers"
)

func TestTipManager_AddTip(t *testing.T) {
	tangle := NewTestTangle()
	defer func(tangle *Tangle) {
		_ = tangle.Prune()
		tangle.Shutdown()
	}(tangle)
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
		messages["1"] = createAndStoreParentsDataMessageInMasterBranch(tangle, NewMessageIDs(EmptyMessageID), NewMessageIDs())
		tipManager.AddTip(messages["1"])

		assert.Equal(t, 1, tipManager.TipCount())
		assert.Contains(t, tipManager.tips.Keys(), messages["1"].ID())
	}

	// Message 2
	{
		messages["2"] = createAndStoreParentsDataMessageInMasterBranch(tangle, NewMessageIDs(EmptyMessageID), NewMessageIDs())
		tipManager.AddTip(messages["2"])

		assert.Equal(t, 2, tipManager.TipCount())
		assert.Contains(t, tipManager.tips.Keys(), messages["1"].ID(), messages["2"].ID())
	}

	// Message 3
	{
		messages["3"] = createAndStoreParentsDataMessageInMasterBranch(tangle, NewMessageIDs(EmptyMessageID, messages["1"].ID(), messages["2"].ID()), NewMessageIDs())
		tipManager.AddTip(messages["3"])

		assert.Equal(t, 1, tipManager.TipCount())
		assert.Contains(t, tipManager.tips.Keys(), messages["3"].ID())
	}
}

func TestTipManager_DataMessageTips(t *testing.T) {
	tangle := NewTestTangle()
	defer func(tangle *Tangle) {
		_ = tangle.Prune()
		tangle.Shutdown()
	}(tangle)
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
		messages["1"] = createAndStoreParentsDataMessageInMasterBranch(tangle, NewMessageIDs(EmptyMessageID), NewMessageIDs())
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
		messages["2"] = createAndStoreParentsDataMessageInMasterBranch(tangle, NewMessageIDs(EmptyMessageID), NewMessageIDs())
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
		messages["3"] = createAndStoreParentsDataMessageInMasterBranch(tangle, NewMessageIDs(messages["1"].ID(), messages["2"].ID()), NewMessageIDs())
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
		tips := NewMessageIDs()
		tips.Add(messages["3"].ID())
		for count, n := range []int{4, 5, 6, 7, 8} {
			nString := strconv.Itoa(n)
			messages[nString] = createAndStoreParentsDataMessageInMasterBranch(tangle, NewMessageIDs(messages["1"].ID()), NewMessageIDs())
			tipManager.AddTip(messages[nString])
			tips.Add(messages[nString].ID())

			assert.Equalf(t, count+2, tipManager.TipCount(), "TipCount does not match after adding Message %d", n)
			assert.ElementsMatchf(t, tipManager.tips.Keys(), tips.Slice(), "Elements in strongTips do not match after adding Message %d", n)
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
	confirmedMessageIDs := NewMessageIDs()
	tangle.ConfirmationOracle = &MockConfirmationOracleTipManagerTest{confirmedMessageIDs: confirmedMessageIDs, confirmedMarkers: markers.NewMarkers(markers.NewMarker(0, 1))}

	testFramework := NewMessageTestFramework(tangle, WithGenesisOutput("G1", 5), WithGenesisOutput("G2", 8))

	// region prepare scenario /////////////////////////////////////////////////////////////////////////////////////////

	// Message 1
	{
		issueTime := time.Now().Add(-maxParentsTimeDifference - 5*time.Minute)

		testFramework.CreateMessage(
			"Message1",
			WithStrongParents("Genesis"),
			WithIssuingTime(issueTime),
			WithInputs("G1"),
			WithOutput("A", 3),
			WithOutput("B", 1),
			WithOutput("c", 1),
		)
		testFramework.IssueMessages("Message1")
		bookMessage(t, tangle, testFramework.Message("Message1"))
		testFramework.WaitMessagesBooked()

		tipManager.AddTip(testFramework.Message("Message1"))
		assert.Equal(t, 0, tipManager.TipCount())

		// mark this message as confirmed
		confirmedMessageIDs.Add(testFramework.Message("Message1").ID())
	}

	// Message 2
	{
		testFramework.CreateMessage(
			"Message2",
			WithStrongParents("Genesis"),
			WithInputs("G2"),
			WithOutput("D", 6),
			WithOutput("E", 1),
			WithOutput("F", 1),
		)
		testFramework.IssueMessages("Message2")
		bookMessage(t, tangle, testFramework.Message("Message2"))
		testFramework.WaitMessagesBooked()

		tipManager.AddTip(testFramework.Message("Message2"))
		assert.Equal(t, 1, tipManager.TipCount())

		// use this message to set TangleTime
		tangle.TimeManager.updateTime(testFramework.Message("Message2").ID())
	}

	// Message 3
	{
		testFramework.CreateMessage(
			"Message3",
			WithStrongParents("Genesis"),
			WithInputs("A"),
			WithOutput("H", 1),
			WithOutput("I", 1),
			WithOutput("J", 1),
		)
		testFramework.IssueMessages("Message3")
		bookMessage(t, tangle, testFramework.Message("Message3"))
		testFramework.WaitMessagesBooked()
		tipManager.AddTip(testFramework.Message("Message3"))
		assert.Equal(t, 2, tipManager.TipCount())
	}

	// Message 4
	{

		testFramework.CreateMessage(
			"Message4",
			WithStrongParents("Genesis", "Message2"),
			WithInputs("D"),
			WithOutput("K", 1),
			WithOutput("L", 1),
			WithOutput("M", 1),
			WithOutput("N", 1),
			WithOutput("O", 1),
			WithOutput("P", 1),
		)
		testFramework.IssueMessages("Message4")
		bookMessage(t, tangle, testFramework.Message("Message4"))
		testFramework.WaitMessagesBooked()
		tipManager.AddTip(testFramework.Message("Message4"))
		assert.Equal(t, 2, tipManager.TipCount())
	}

	// Message 5
	{
		testFramework.CreateMessage(
			"Message5",
			WithStrongParents("Genesis", "Message1"),
		)
		testFramework.IssueMessages("Message5")
		bookMessage(t, tangle, testFramework.Message("Message5"))
		testFramework.WaitMessagesBooked()
		tipManager.AddTip(testFramework.Message("Message5"))
		assert.Equal(t, 3, tipManager.TipCount())
	}

	createScenarioMessageWith1Input1Output := func(messageStringID, inputStringID, outputStringID string, strongParents []string) {
		testFramework.CreateMessage(
			messageStringID,
			WithInputs(inputStringID),
			WithOutput(outputStringID, 1),
			WithStrongParents(strongParents...),
		)
		testFramework.IssueMessages(messageStringID)
		bookMessage(t, tangle, testFramework.Message(messageStringID))
		testFramework.WaitMessagesBooked()
	}

	// Message 6
	{
		createScenarioMessageWith1Input1Output("Message6", "H", "Q", []string{"Message3", "Message5"})
		tipManager.AddTip(testFramework.Message("Message6"))
		assert.Equal(t, 2, tipManager.TipCount())
	}

	// Message 7
	{
		createScenarioMessageWith1Input1Output("Message7", "I", "R", []string{"Message3", "Message5"})
		tipManager.AddTip(testFramework.Message("Message7"))
		assert.Equal(t, 3, tipManager.TipCount())
	}

	// Message 8
	{
		createScenarioMessageWith1Input1Output("Message8", "J", "S", []string{"Message3", "Message5"})
		tipManager.AddTip(testFramework.Message("Message8"))
		assert.Equal(t, 4, tipManager.TipCount())
	}

	// Message 9
	{
		createScenarioMessageWith1Input1Output("Message9", "K", "T", []string{"Message4"})
		tipManager.AddTip(testFramework.Message("Message9"))
		assert.Equal(t, 4, tipManager.TipCount())
	}

	// Message 10
	{
		createScenarioMessageWith1Input1Output("Message10", "L", "U", []string{"Message2", "Message4"})
		tipManager.AddTip(testFramework.Message("Message10"))
		assert.Equal(t, 5, tipManager.TipCount())
	}

	// Message 11
	{
		createScenarioMessageWith1Input1Output("Message11", "M", "V", []string{"Message2", "Message4"})
		tipManager.AddTip(testFramework.Message("Message11"))
		assert.Equal(t, 6, tipManager.TipCount())
	}

	// Message 12
	{
		createScenarioMessageWith1Input1Output("Message12", "N", "X", []string{"Message3", "Message4"})
		tipManager.AddTip(testFramework.Message("Message12"))
		assert.Equal(t, 7, tipManager.TipCount())
	}

	// Message 13
	{
		createScenarioMessageWith1Input1Output("Message13", "O", "Y", []string{"Message4"})
		tipManager.AddTip(testFramework.Message("Message13"))
		assert.Equal(t, 8, tipManager.TipCount())
	}

	// Message 14
	{
		createScenarioMessageWith1Input1Output("Message14", "P", "Z", []string{"Message4"})
		tipManager.AddTip(testFramework.Message("Message14"))
		assert.Equal(t, 9, tipManager.TipCount())
	}

	// Message 15
	{
		testFramework.CreateMessage(
			"Message15",
			WithStrongParents("Message10", "Message11"),
		)
		testFramework.IssueMessages("Message15")
		bookMessage(t, tangle, testFramework.Message("Message15"))
		testFramework.WaitMessagesBooked()
		tipManager.AddTip(testFramework.Message("Message15"))
		assert.Equal(t, 8, tipManager.TipCount())
	}

	// Message 16
	{
		testFramework.CreateMessage(
			"Message16",
			WithStrongParents("Message10", "Message11", "Message14"),
		)
		testFramework.IssueMessages("Message16")
		bookMessage(t, tangle, testFramework.Message("Message16"))
		testFramework.WaitMessagesBooked()
		tipManager.AddTip(testFramework.Message("Message16"))
		assert.Equal(t, 8, tipManager.TipCount())
	}
	// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////

	// Message 17
	{
		testFramework.CreateMessage(
			"Message17",
			WithStrongParents("Message16"),
			WithOutput("OUT17", 8),
			WithInputs("Q", "R", "S", "T", "U", "V", "X", "Y"),
		)
		parents, err := tipManager.Tips(testFramework.Message("Message17").Payload(), 4)
		assert.NoError(t, err)
		assert.Equal(t, parents, NewMessageIDs(
			testFramework.Message("Message6").ID(),
			testFramework.Message("Message7").ID(),
			testFramework.Message("Message8").ID(),
			testFramework.Message("Message9").ID(),
			testFramework.Message("Message10").ID(),
			testFramework.Message("Message11").ID(),
			testFramework.Message("Message12").ID(),
			testFramework.Message("Message13").ID(),
		))
	}

	// Message 18
	{
		testFramework.CreateMessage(
			"Message18",
			WithStrongParents("Message16"),
			WithOutput("OUT18", 6),
			WithInputs("Q", "R", "S", "T", "U", "V"),
		)

		parents, err := tipManager.Tips(testFramework.Message("Message18").Payload(), 4)

		assert.NoError(t, err)
		// there are possible parents to be selected, however, since the directly referenced messages are tips as well
		// there is a chance that these are doubly selected, resulting 6 to 8 parents
		assert.GreaterOrEqual(t, len(parents), 6)
		assert.LessOrEqual(t, len(parents), 8)
		assert.Contains(t, parents,
			testFramework.Message("Message6").ID(),
			testFramework.Message("Message7").ID(),
			testFramework.Message("Message8").ID(),
			testFramework.Message("Message9").ID(),
			testFramework.Message("Message10").ID(),
			testFramework.Message("Message11").ID(),
		)
	}

	// Message 19
	{
		testFramework.CreateMessage(
			"Message19",
			WithStrongParents("Message16"),
			WithOutput("OUT19", 3),
			WithInputs("B", "V", "Z"),
		)

		parents, err := tipManager.Tips(testFramework.Message("Message19").Payload(), 4)
		assert.NoError(t, err)

		// we reference 11, 14 directly. 1 is too old and should not be directly referenced
		assert.GreaterOrEqual(t, len(parents), 4)
		assert.LessOrEqual(t, len(parents), 8)
		assert.Contains(t, parents,
			testFramework.Message("Message11").ID(),
			testFramework.Message("Message14").ID(),
		)
		assert.NotContains(t, parents,
			testFramework.Message("Message1").ID(),
		)
	}

	// Message 20
	{

		testFramework.CreateMessage(
			"Message20",
			WithStrongParents("Message16"),
			WithOutput("OUT20", 9),
			WithInputs("Q", "R", "S", "T", "U", "V", "X", "Y", "Z"),
		)

		parents, err := tipManager.Tips(testFramework.Message("Message20").Payload(), 4)
		assert.NoError(t, err)

		// there are 9 inputs to be directly referenced -> we need to reference them via tips (8 tips available)
		// due to the tips' nature they contain all transactions in the past cone
		assert.Len(t, parents, 8)
	}
}

// Test based on packages/tangle/images/TSC_test_scenario.png except nothing is confirmed.
func TestTipManager_TimeSinceConfirmation_Unconfirmed(t *testing.T) {
	t.Skip("Skip this test.")
	tangle := NewTestTangle()
	tangle.Booker.MarkersManager.Manager = markers.NewManager(markers.WithCacheTime(0), markers.WithMaxPastMarkerDistance(10))
	defer tangle.Shutdown()

	tipManager := tangle.TipManager

	testFramework := NewMessageTestFramework(
		tangle,
	)

	tangle.Setup()

	createTestTangleTSC(t, testFramework)
	var confirmedMessageIDsString []string
	confirmedMessageIDs := prepareConfirmedMessageIDs(testFramework, confirmedMessageIDsString)
	confirmedMarkers := markers.NewMarkers()

	tangle.ConfirmationOracle = &MockConfirmationOracleTipManagerTest{confirmedMessageIDs: confirmedMessageIDs, confirmedMarkers: confirmedMarkers}
	tangle.TimeManager.updateTime(testFramework.Message("Marker-2/3").ID())

	// Even without any confirmations, it should be possible to attach to genesis.
	assert.True(t, tipManager.isPastConeTimestampCorrect(EmptyMessageID))

	// case 0 - only one message can attach to genesis, so there should not be two subtangles starting from the genesis, but TSC allows using such tip.
	assert.True(t, tipManager.isPastConeTimestampCorrect(testFramework.Message("0/0_2").ID()))
	// case #1
	assert.False(t, tipManager.isPastConeTimestampCorrect(testFramework.Message("0/3_4").ID()))
	// case #2
	assert.False(t, tipManager.isPastConeTimestampCorrect(testFramework.Message("1/3_4").ID()))
	// case #3
	assert.False(t, tipManager.isPastConeTimestampCorrect(testFramework.Message("2/3_4").ID()))
	// case #4 (marker message)
	assert.False(t, tipManager.isPastConeTimestampCorrect(testFramework.Message("Marker-1/2").ID()))
	// case #5
	assert.False(t, tipManager.isPastConeTimestampCorrect(testFramework.Message("2/5_4").ID()))
	// case #6 (attach to unconfirmed message older than TSC)
	assert.False(t, tipManager.isPastConeTimestampCorrect(testFramework.Message("0/1-preTSC_2").ID()))
	// case #7
	assert.False(t, tipManager.isPastConeTimestampCorrect(testFramework.Message("3/2_4").ID()))
	// case #8
	assert.False(t, tipManager.isPastConeTimestampCorrect(testFramework.Message("2/3+0/4_3").ID()))
	// case #9
	assert.False(t, tipManager.isPastConeTimestampCorrect(testFramework.Message("Marker-4/5").ID()))
	// case #10 (attach to confirmed message older than TSC)
	assert.False(t, tipManager.isPastConeTimestampCorrect(testFramework.Message("0/1-preTSCSeq2_2").ID()))
	// case #11
	assert.False(t, tipManager.isPastConeTimestampCorrect(testFramework.Message("5/2_4").ID()))
}

// Test based on packages/tangle/images/TSC_test_scenario.png.
func TestTipManager_TimeSinceConfirmation_Confirmed(t *testing.T) {
	tangle := NewTestTangle()
	tangle.Booker.MarkersManager.Manager = markers.NewManager(markers.WithCacheTime(0), markers.WithMaxPastMarkerDistance(10))

	defer tangle.Shutdown()

	tipManager := tangle.TipManager

	testFramework := NewMessageTestFramework(
		tangle,
	)

	tangle.Setup()

	createTestTangleTSC(t, testFramework)
	confirmedMessageIDsString := []string{"Marker-0/1", "0/1-preTSCSeq1_0", "0/1-preTSCSeq1_1", "0/1-preTSCSeq1_2", "0/1-postTSCSeq1_0", "0/1-postTSCSeq1_1", "0/1-postTSCSeq1_2", "0/1-postTSCSeq1_3", "0/1-postTSCSeq1_4", "0/1-postTSCSeq1_5", "Marker-1/2", "0/1-preTSCSeq2_0", "0/1-preTSCSeq2_1", "0/1-preTSCSeq2_2", "0/1-postTSCSeq2_0", "0/1-postTSCSeq2_1", "0/1-postTSCSeq2_2", "0/1-postTSCSeq2_3", "0/1-postTSCSeq2_4", "0/1-postTSCSeq2_5", "Marker-2/2", "2/2_0", "2/2_1", "2/2_2", "2/2_3", "2/2_4", "Marker-2/3"}
	confirmedMessageIDs := prepareConfirmedMessageIDs(testFramework, confirmedMessageIDsString)
	confirmedMarkers := markers.NewMarkers(markers.NewMarker(0, 1), markers.NewMarker(1, 2), markers.NewMarker(2, 3))

	tangle.ConfirmationOracle = &MockConfirmationOracleTipManagerTest{confirmedMessageIDs: confirmedMessageIDs, confirmedMarkers: confirmedMarkers}
	tangle.TimeManager.updateTime(testFramework.Message("Marker-2/3").ID())

	// Even without any confirmations, it should be possible to attach to genesis.
	assert.True(t, tipManager.isPastConeTimestampCorrect(EmptyMessageID))

	// case 0 - only one message can attach to genesis, so there should not be two subtangles starting from the genesis, but TSC allows using such tip.
	assert.True(t, tipManager.isPastConeTimestampCorrect(testFramework.Message("0/0_2").ID()))
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
	// case #6 (attach to unconfirmed message older than TSC)
	assert.False(t, tipManager.isPastConeTimestampCorrect(testFramework.Message("0/1-preTSC_2").ID()))
	// // case #7
	assert.True(t, tipManager.isPastConeTimestampCorrect(testFramework.Message("3/2_4").ID()))
	// case #8
	assert.False(t, tipManager.isPastConeTimestampCorrect(testFramework.Message("2/3+0/4_3").ID()))
	// case #9
	assert.False(t, tipManager.isPastConeTimestampCorrect(testFramework.Message("Marker-4/5").ID()))
	// case #10 (attach to confirmed message older than TSC)
	assert.True(t, tipManager.isPastConeTimestampCorrect(testFramework.Message("0/1-preTSCSeq2_2").ID()))
	// case #11
	assert.False(t, tipManager.isPastConeTimestampCorrect(testFramework.Message("5/2_4").ID()))
}

func createTestTangleTSC(t *testing.T, testFramework *MessageTestFramework) {
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
		_ = issueMessages(testFramework, "0/4", 5, []string{"Marker-0/4"}, 0)
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

	// SEQUENCE 0 (without markers)
	{
		_ = issueMessages(testFramework, "0/0", 3, []string{"Genesis"}, time.Minute)

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"0/0_0": markers.NewMarkers(markers.NewMarker(0, 0)),
			"0/0_1": markers.NewMarkers(markers.NewMarker(0, 0)),
			"0/0_2": markers.NewMarkers(markers.NewMarker(0, 0)),
		})
	}
	// SEQUENCE 1
	{ //nolint:dupl
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
		_ = issueMessages(testFramework, "1/3", 5, []string{"Marker-1/3"}, 0)
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
	{ //nolint:dupl
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
		_ = issueMessages(testFramework, "2/3", 5, []string{"Marker-2/3"}, 0)
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
		_ = issueMessages(testFramework, "2/5", 5, []string{"Marker-2/5"}, 0)
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
		_ = issueMessages(testFramework, "3/2", 5, []string{"Marker-3/2"}, 0)
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
		_ = issueMessages(testFramework, "5/2", 5, []string{"Marker-5/2"}, 0)
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
}

func prepareConfirmedMessageIDs(testFramework *MessageTestFramework, confirmedIDs []string) MessageIDs {
	confirmedMessageIDs := NewMessageIDs()
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

func bookMessage(t *testing.T, tangle *Tangle, message *Message) {
	// TODO: CheckTransaction should be removed here once the booker passes on errors
	if message.Payload().Type() == ledgerstate.TransactionType {
		err := tangle.LedgerState.UTXODAG.CheckTransaction(message.Payload().(*ledgerstate.Transaction))
		require.NoError(t, err)
	}
	err := tangle.Booker.BookMessage(message.ID())
	require.NoError(t, err)

	tangle.Storage.MessageMetadata(message.ID()).Consume(func(messageMetadata *MessageMetadata) {
		// make sure that everything was booked into master branch
		require.True(t, messageMetadata.IsBooked())
		messageBranchIDs, err := tangle.Booker.MessageBranchIDs(message.ID())
		assert.NoError(t, err)
		require.Equal(t, ledgerstate.NewBranchIDs(ledgerstate.MasterBranchID), messageBranchIDs)
		messageMetadata.StructureDetails()
	})
}

func createAndStoreParentsDataMessageInMasterBranch(tangle *Tangle, strongParents, weakParents MessageIDs) (message *Message) {
	parents := ParentMessageIDs{
		StrongParentType: strongParents,
	}
	if len(weakParents) > 0 {
		parents[WeakParentType] = weakParents
	}

	message = newTestParentsDataMessage("testmessage", parents)
	tangle.Storage.StoreMessage(message)

	return
}

type MockConfirmationOracleTipManagerTest struct {
	confirmedMessageIDs MessageIDs
	confirmedMarkers    *markers.Markers

	MockConfirmationOracle
}

// IsMessageConfirmed mocks its interface function.
func (m *MockConfirmationOracleTipManagerTest) IsMessageConfirmed(msgID MessageID) bool {
	return m.confirmedMessageIDs.Contains(msgID)
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
	if marker == nil || m.confirmedMarkers == nil || m.confirmedMarkers.Size() == 0 {
		return false
	}
	confirmedMarkerIndex, exists := m.confirmedMarkers.Get(marker.SequenceID())
	if !exists {
		return false
	}
	return marker.Index() <= confirmedMarkerIndex
}
