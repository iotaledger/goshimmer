package notarization

import (
	"fmt"
	"testing"
	"time"

	"github.com/iotaledger/goshimmer/packages/conflictdag"
	"github.com/iotaledger/goshimmer/packages/consensus/finality"
	"github.com/iotaledger/goshimmer/packages/epoch"
	"github.com/iotaledger/goshimmer/packages/ledger"
	"github.com/iotaledger/goshimmer/packages/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/ledger/vm/devnetvm"
	"github.com/iotaledger/goshimmer/packages/tangle"

	"github.com/iotaledger/hive.go/generics/event"
	"github.com/iotaledger/hive.go/generics/orderedmap"
	"github.com/iotaledger/hive.go/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewManager(t *testing.T) {
	testTangle := tangle.NewTestTangle()
	vm := new(devnetvm.VM)
	m := NewManager(NewEpochManager(), NewEpochCommitmentFactory(testTangle.Options.Store, vm, testTangle), testTangle)
	assert.NotNil(t, m)
}

func TestManager_PendingConflictsCount(t *testing.T) {
	testTangle := tangle.NewTestTangle()
	vm := new(devnetvm.VM)
	m := NewManager(NewEpochManager(), NewEpochCommitmentFactory(testTangle.Options.Store, vm, testTangle), testTangle)
	m.pendingConflictsCount[3] = 3
	assert.Equal(t, uint64(3), m.PendingConflictsCount(3))
}

func TestManager_IsCommittable(t *testing.T) {
	m := testNotarizationManager()
	ei := epoch.EI(5)
	m.pendingConflictsCount[ei] = 0
	// not old enough
	assert.False(t, m.IsCommittable(ei))

	ei = epoch.EI(1)
	m.pendingConflictsCount[ei] = 1
	// old enough but pbc > 0
	assert.False(t, m.IsCommittable(ei))
	m.pendingConflictsCount[ei] = 0
	// old enough and pbc > 0
	assert.True(t, m.IsCommittable(ei))
}

func TestManager_GetLatestEC(t *testing.T) {
	m := testNotarizationManager()
	// epoch ages (in mins) since genesis [25,20,15,10,5]
	for i := 0; i <= 5; i++ {
		m.pendingConflictsCount[epoch.EI(i)] = uint64(i)
		err := m.epochCommitmentFactory.InsertTangleLeaf(epoch.EI(i), tangle.EmptyMessageID)
		require.NoError(t, err)
	}

	commitment, err := m.GetLatestEC()
	assert.NoError(t, err)
	// only epoch 0 has pbc = 0
	assert.Equal(t, epoch.EI(0), commitment.EI)

	m.pendingConflictsCount[4] = 0
	commitment, err = m.GetLatestEC()
	assert.NoError(t, err)
	// epoch 4 has pbc = 0 but is not old enough
	assert.Equal(t, epoch.EI(0), commitment.EI)

	m.pendingConflictsCount[2] = 0
	commitment, err = m.GetLatestEC()
	assert.NoError(t, err)
	// epoch 2 has pbc=0 and is old enough
	assert.Equal(t, epoch.EI(2), commitment.EI)
}

func TestManager_UpdateTangleTree(t *testing.T) {
	var epochInterval int64 = 1
	var minCommittable time.Duration = 2 * time.Second
	genesisTime := time.Now()
	var m *Manager

	processMsgScenario := tangle.ProcessMessageScenario3(t, tangle.WithConflictDAGOptions(conflictdag.WithMergeToMaster(false)))
	testTangle := processMsgScenario.Tangle
	vm := new(devnetvm.VM)

	// set up notarization manager
	ecFactory := NewEpochCommitmentFactory(testTangle.Options.Store, vm, testTangle)
	epochMgr := NewEpochManager(Interval(epochInterval), GenesisTime(genesisTime.Unix()))
	m = NewManager(epochMgr, ecFactory, testTangle, MinCommittableEpochAge(minCommittable))

	commitmentFunc := func() (ecRecord *epoch.ECRecord, latestConfirmedEpoch epoch.EI, err error) {
		ecRecord, err = m.GetLatestEC()
		require.NoError(t, err)
		latestConfirmedEpoch, err = m.LatestConfirmedEpochIndex()
		require.NoError(t, err)
		return ecRecord, latestConfirmedEpoch, nil
	}
	testTangle.Options.CommitmentFunc = commitmentFunc

	// set up finality gadget
	testOpts := []finality.Option{
		finality.WithBranchThresholdTranslation(TestBranchGoFTranslation),
		finality.WithMessageThresholdTranslation(TestMessageGoFTranslation),
	}
	sfg := finality.NewSimpleFinalityGadget(processMsgScenario.Tangle, testOpts...)

	registerToTangleEvents(m, sfg, testTangle)
	eventHandlerMock := NewEventMock(t, m, ecFactory)

	loadSnapshot(m, processMsgScenario.TestFramework)

	prePostSteps := []*tangle.PrePostStepTuple{
		// Message1
		{
			Pre: func(t *testing.T, testFramework *tangle.MessageTestFramework, testEventMock *tangle.EventMock, nodes tangle.NodeIdentities) {
				time.Sleep(time.Duration(epochInterval) * time.Second)
			},
		},
		// Message2
		{
			Pre: func(t *testing.T, testFramework *tangle.MessageTestFramework, testEventMock *tangle.EventMock, nodes tangle.NodeIdentities) {
				time.Sleep(time.Duration(epochInterval) * time.Second)
				fmt.Println("message 2")
			},
		},
		// Message3
		{
			Pre: func(t *testing.T, testFramework *tangle.MessageTestFramework, testEventMock *tangle.EventMock, nodes tangle.NodeIdentities) {
				time.Sleep(time.Duration(epochInterval) * time.Second)
				eventHandlerMock.Expect("NewCommitmentTreesCreated", epoch.EI(1))
				eventHandlerMock.Expect("EpochCommitted", epoch.EI(1))
				fmt.Println("message 3")
			},
		},
		// Message4
		{
			Pre: func(t *testing.T, testFramework *tangle.MessageTestFramework, testEventMock *tangle.EventMock, nodes tangle.NodeIdentities) {
				time.Sleep(time.Duration(epochInterval) * time.Second)
				eventHandlerMock.Expect("NewCommitmentTreesCreated", epoch.EI(2))
				eventHandlerMock.Expect("EpochCommitted", epoch.EI(2))
				fmt.Println("message 4")
			},
		},
		// Message5
		{
			Pre: func(t *testing.T, testFramework *tangle.MessageTestFramework, testEventMock *tangle.EventMock, nodes tangle.NodeIdentities) {
				time.Sleep(time.Duration(epochInterval) * time.Second)
				eventHandlerMock.Expect("NewCommitmentTreesCreated", epoch.EI(3))
				eventHandlerMock.Expect("EpochCommitted", epoch.EI(3))
				fmt.Println("message 5")
			},

			Post: func(t *testing.T, testFramework *tangle.MessageTestFramework, testEventMock *tangle.EventMock, nodes tangle.NodeIdentities) {
				assertExistenceOfBlock(t, testFramework, m, []string{
					"Message1",
					"Message2",
					"Message3",
				})
			},
		},
	}

	for i := 0; processMsgScenario.HasNext(); i++ {
		if len(prePostSteps)-1 < i {
			processMsgScenario.Next(nil)
			continue
		}
		processMsgScenario.Next(prePostSteps[i])
	}

	eventHandlerMock.AssertExpectations(t)
}

func assertExistenceOfBlock(t *testing.T, testFramework *tangle.MessageTestFramework, m *Manager, aliasNames []string) {
	for _, alias := range aliasNames {

		_, err := m.GetBlockInclusionProof(testFramework.Message(alias).ID())
		require.NoError(t, err)
		fmt.Println(alias)
	}
}

func testNotarizationManager() *Manager {
	t := time.Now().Add(-25 * time.Minute).Unix()
	testTangle := tangle.NewTestTangle()
	interval := int64(5 * 60)
	vm := new(devnetvm.VM)
	return NewManager(NewEpochManager(GenesisTime(t), Interval(interval)), NewEpochCommitmentFactory(testTangle.Options.Store, vm, testTangle), testTangle, MinCommittableEpochAge(10*time.Minute))
}

func loadSnapshot(m *Manager, testFramework *tangle.MessageTestFramework) {
	snapshot := testFramework.Snapshot()
	snapshot.DiffEpochIndex = epoch.EI(0)
	snapshot.FullEpochIndex = epoch.EI(0)

	diffs := orderedmap.New[epoch.EI, *ledger.EpochDiff]()
	diff := ledger.NewEpochDiff(snapshot.DiffEpochIndex)
	snapshot.Outputs.ForEach(func(output utxo.Output) error {
		diff.AddCreated(output)
		return nil
	})
	diffs.Set(snapshot.DiffEpochIndex, ledger.NewEpochDiff(snapshot.DiffEpochIndex))
	snapshot.EpochDiffs = &ledger.EpochDiffs{*diffs}

	ecRecord := epoch.NewECRecord(snapshot.FullEpochIndex)
	ecRecord.SetECR(&epoch.MerkleRoot{types.NewIdentifier([]byte{})})
	ecRecord.SetPrevEC(&epoch.MerkleRoot{types.NewIdentifier([]byte{})})
	snapshot.LatestECRecord = ecRecord

	m.LoadSnapshot(snapshot)
}

func registerToTangleEvents(m *Manager, sfg *finality.SimpleFinalityGadget, testTangle *tangle.Tangle) {
	testTangle.ApprovalWeightManager.Events.MarkerWeightChanged.Hook(event.NewClosure(func(e *tangle.MarkerWeightChangedEvent) {
		sfg.HandleMarker(e.Marker, e.Weight)
	}))
	testTangle.ApprovalWeightManager.Events.BranchWeightChanged.Hook(event.NewClosure(func(e *tangle.BranchWeightChangedEvent) {
		sfg.HandleBranch(e.BranchID, e.Weight)
	}))
	testTangle.ConfirmationOracle.Events().MessageConfirmed.Attach(event.NewClosure(func(event *tangle.MessageConfirmedEvent) {
		testTangle.Storage.Message(event.Message.ID()).Consume(func(msg *tangle.Message) {
			m.OnMessageConfirmed(msg)
		})
	}))
	testTangle.ConfirmationOracle.Events().MessageOrphaned.Attach(event.NewClosure(func(event *tangle.MessageConfirmedEvent) {
		m.OnMessageOrphaned(event.Message)
	}))
	testTangle.Ledger.Events.TransactionConfirmed.Attach(event.NewClosure(func(event *ledger.TransactionConfirmedEvent) {
		fmt.Println("event triggered")
		testTangle.Ledger.Storage.CachedTransaction(event.TransactionID).Consume(func(t utxo.Transaction) {
			m.OnTransactionConfirmed(t.(*devnetvm.Transaction))
		})
	}))
	testTangle.Ledger.Events.TransactionInclusionUpdated.Attach(event.NewClosure(func(event *ledger.TransactionInclusionUpdatedEvent) {
		m.OnTransactionInclusionUpdated(event)
	}))

	testTangle.Ledger.ConflictDAG.Events.BranchConfirmed.Attach(event.NewClosure(func(event *conflictdag.BranchConfirmedEvent[utxo.TransactionID]) {
		m.OnBranchConfirmed(event.ID)
	}))
	testTangle.Ledger.ConflictDAG.Events.ConflictCreated.Attach(event.NewClosure(func(event *conflictdag.ConflictCreatedEvent[utxo.TransactionID, utxo.OutputID]) {
		m.OnBranchCreated(event.ID)
	}))
	testTangle.Ledger.ConflictDAG.Events.BranchRejected.Attach(event.NewClosure(func(event *conflictdag.BranchRejectedEvent[utxo.TransactionID]) {
		m.OnBranchRejected(event.ID)
	}))
}
