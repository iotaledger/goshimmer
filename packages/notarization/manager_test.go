package notarization

import (
	"fmt"
	"runtime/debug"
	"testing"
	"time"

	"github.com/iotaledger/goshimmer/packages/conflictdag"
	"github.com/iotaledger/goshimmer/packages/consensus/finality"
	"github.com/iotaledger/goshimmer/packages/epoch"
	"github.com/iotaledger/goshimmer/packages/ledger"
	"github.com/iotaledger/goshimmer/packages/tangle"

	"github.com/iotaledger/hive.go/generics/event"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewManager(t *testing.T) {
	testTangle := tangle.NewTestTangle()
	m := NewManager(NewEpochManager(), NewEpochCommitmentFactory(testTangle.Options.Store, testTangle, 1), testTangle)
	assert.NotNil(t, m)
}

func TestManager_pendingConflictsCounters(t *testing.T) {
	testTangle := tangle.NewTestTangle()
	m := NewManager(NewEpochManager(), NewEpochCommitmentFactory(testTangle.Options.Store, testTangle, 1), testTangle)
	m.pendingConflictsCounters[3] = 3
	assert.Equal(t, uint64(3), m.pendingConflictsCounters[3])
}

func TestManager_IsCommittable(t *testing.T) {
	m := testNotarizationManager()
	Index := epoch.Index(5)
	m.pendingConflictsCounters[Index] = 0
	// not old enough
	assert.False(t, m.isCommittable(Index))

	Index = epoch.Index(1)
	m.pendingConflictsCounters[Index] = 1
	// old enough but pbc > 0
	assert.False(t, m.isCommittable(Index))
	m.pendingConflictsCounters[Index] = 0
	// old enough and pbc > 0
	assert.True(t, m.isCommittable(Index))
}

func TestManager_GetLatestEC(t *testing.T) {
	m := testNotarizationManager()
	// epoch ages (in mins) since genesis [25,20,15,10,5]
	for i := 0; i <= 5; i++ {
		m.pendingConflictsCounters[epoch.Index(i)] = uint64(i)
		err := m.epochCommitmentFactory.insertTangleLeaf(epoch.Index(i), tangle.EmptyMessageID)
		require.NoError(t, err)
	}

	commitment, err := m.GetLatestEC()
	assert.NoError(t, err)
	// only epoch 0 has pbc = 0
	assert.Equal(t, epoch.Index(0), commitment.EI)

	m.pendingConflictsCounters[4] = 0
	commitment, err = m.GetLatestEC()
	assert.NoError(t, err)
	// epoch 4 has pbc = 0 but is not old enough
	assert.Equal(t, epoch.Index(0), commitment.EI)

	m.pendingConflictsCounters[2] = 0
	commitment, err = m.GetLatestEC()
	assert.NoError(t, err)
	// epoch 2 has pbc=0 and is old enough
	assert.Equal(t, epoch.Index(2), commitment.EI)
}

func TestManager_UpdateTangleTree(t *testing.T) {
	var epochInterval = 1 * time.Second

	processMsgScenario := tangle.NotarizationMessageScenario(t, tangle.WithConflictDAGOptions(conflictdag.WithMergeToMaster(false)))
	eventHandlerMock, notarizationMgr := setupFramework(t, processMsgScenario, epochInterval)
	defer func(processMsgScenario *tangle.TestScenario, t *testing.T) {
		if err := recover(); err != nil {
			t.Error(err)
			fmt.Println(string(debug.Stack()))
			return
		}

		if err := processMsgScenario.Cleanup(t); err != nil {
			require.NoError(t, err)
		}
	}(processMsgScenario, t)

	prePostSteps := []*tangle.PrePostStepTuple{
		// Message1, issuing time epoch 1
		{
			Pre: func(t *testing.T, testFramework *tangle.MessageTestFramework, testEventMock *tangle.EventMock, nodes tangle.NodeIdentities) {
				time.Sleep(epochInterval)
				eventHandlerMock.Expect("EpochCommitted", epoch.Index(0))
			},
			Post: func(t *testing.T, testFramework *tangle.MessageTestFramework, testEventMock *tangle.EventMock, nodes tangle.NodeIdentities) {
				msg := testFramework.Message("Message1")
				assert.Equal(t, epoch.Index(0), msg.EI())
			},
		},
		// Message2, issuing time epoch 2
		{
			Pre: func(t *testing.T, testFramework *tangle.MessageTestFramework, testEventMock *tangle.EventMock, nodes tangle.NodeIdentities) {
				time.Sleep(epochInterval)
				eventHandlerMock.Expect("EpochCommitted", epoch.Index(0))
				fmt.Println("message 2")
			},
			Post: func(t *testing.T, testFramework *tangle.MessageTestFramework, testEventMock *tangle.EventMock, nodes tangle.NodeIdentities) {
				msg := testFramework.Message("Message2")
				assert.Equal(t, epoch.Index(0), msg.EI())
			},
		},
		// Message3, issuing time epoch 3
		{
			Pre: func(t *testing.T, testFramework *tangle.MessageTestFramework, testEventMock *tangle.EventMock, nodes tangle.NodeIdentities) {
				time.Sleep(epochInterval)
				eventHandlerMock.Expect("EpochCommitted", epoch.Index(1))
				fmt.Println("message 3")
			},
			Post: func(t *testing.T, testFramework *tangle.MessageTestFramework, testEventMock *tangle.EventMock, nodes tangle.NodeIdentities) {
				msg := testFramework.Message("Message3")
				assert.Equal(t, epoch.Index(1), msg.EI())
			},
		},
		// Message4, issuing time epoch 4
		{
			Pre: func(t *testing.T, testFramework *tangle.MessageTestFramework, testEventMock *tangle.EventMock, nodes tangle.NodeIdentities) {
				time.Sleep(epochInterval)
				eventHandlerMock.Expect("EpochCommitted", epoch.Index(2))
				fmt.Println("message 4")
			},
			Post: func(t *testing.T, testFramework *tangle.MessageTestFramework, testEventMock *tangle.EventMock, nodes tangle.NodeIdentities) {
				msg := testFramework.Message("Message4")
				assert.Equal(t, epoch.Index(2), msg.EI())
			},
		},
		// Message5, issuing time epoch 5
		{
			Pre: func(t *testing.T, testFramework *tangle.MessageTestFramework, testEventMock *tangle.EventMock, nodes tangle.NodeIdentities) {
				time.Sleep(epochInterval)
				eventHandlerMock.Expect("EpochCommitted", epoch.Index(3))
				fmt.Println("message 5")
			},

			Post: func(t *testing.T, testFramework *tangle.MessageTestFramework, testEventMock *tangle.EventMock, nodes tangle.NodeIdentities) {
				msg := testFramework.Message("Message5")
				assert.Equal(t, epoch.Index(3), msg.EI())
				assertExistenceOfBlock(t, testFramework, notarizationMgr, map[string]bool{
					"Message1": true,
					"Message2": true,
					"Message3": true,
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

func TestManager_UpdateStateMutationTree(t *testing.T) {
	var epochInterval = 1 * time.Second

	processMsgScenario := tangle.NotarizationTxScenario(t, tangle.WithConflictDAGOptions(conflictdag.WithMergeToMaster(false)))
	eventHandlerMock, notarizationMgr := setupFramework(t, processMsgScenario, epochInterval)
	defer func(processMsgScenario *tangle.TestScenario, t *testing.T) {
		if err := recover(); err != nil {
			t.Error(err)
			fmt.Println(string(debug.Stack()))
			return
		}

		if err := processMsgScenario.Cleanup(t); err != nil {
			require.NoError(t, err)
		}
	}(processMsgScenario, t)

	prePostSteps := []*tangle.PrePostStepTuple{
		// Message1, issuing time epoch 1
		{
			Pre: func(t *testing.T, testFramework *tangle.MessageTestFramework, testEventMock *tangle.EventMock, nodes tangle.NodeIdentities) {
				time.Sleep(epochInterval)
				eventHandlerMock.Expect("EpochCommitted", epoch.Index(0))
			},
			Post: func(t *testing.T, testFramework *tangle.MessageTestFramework, testEventMock *tangle.EventMock, nodes tangle.NodeIdentities) {
				msg := testFramework.Message("Message1")
				assert.Equal(t, epoch.Index(0), msg.EI())
			},
		},
		// Message2, issuing time epoch 2
		{
			Pre: func(t *testing.T, testFramework *tangle.MessageTestFramework, testEventMock *tangle.EventMock, nodes tangle.NodeIdentities) {
				time.Sleep(epochInterval)
				fmt.Println("message 2")
				eventHandlerMock.Expect("EpochCommitted", epoch.Index(0))
			},
			Post: func(t *testing.T, testFramework *tangle.MessageTestFramework, testEventMock *tangle.EventMock, nodes tangle.NodeIdentities) {
				msg := testFramework.Message("Message2")
				assert.Equal(t, epoch.Index(0), msg.EI())
			},
		},
		// Message3, issuing time epoch 3
		{
			Pre: func(t *testing.T, testFramework *tangle.MessageTestFramework, testEventMock *tangle.EventMock, nodes tangle.NodeIdentities) {
				time.Sleep(epochInterval)
				eventHandlerMock.Expect("EpochCommitted", epoch.Index(1))
				fmt.Println("message 3")
			},
			Post: func(t *testing.T, testFramework *tangle.MessageTestFramework, testEventMock *tangle.EventMock, nodes tangle.NodeIdentities) {
				msg := testFramework.Message("Message3")
				assert.Equal(t, epoch.Index(1), msg.EI())
			},
		},
		// Message4, issuing time epoch 4
		{
			Pre: func(t *testing.T, testFramework *tangle.MessageTestFramework, testEventMock *tangle.EventMock, nodes tangle.NodeIdentities) {
				time.Sleep(epochInterval)
				eventHandlerMock.Expect("EpochCommitted", epoch.Index(2))
				fmt.Println("message 4")
			},
			Post: func(t *testing.T, testFramework *tangle.MessageTestFramework, testEventMock *tangle.EventMock, nodes tangle.NodeIdentities) {
				msg := testFramework.Message("Message4")
				assert.Equal(t, epoch.Index(2), msg.EI())
			},
		},
		// Message5 TX1, issuing time epoch 5
		{
			Pre: func(t *testing.T, testFramework *tangle.MessageTestFramework, testEventMock *tangle.EventMock, nodes tangle.NodeIdentities) {
				time.Sleep(epochInterval)
				eventHandlerMock.Expect("EpochCommitted", epoch.Index(3))
				fmt.Println("message 5")
			},

			Post: func(t *testing.T, testFramework *tangle.MessageTestFramework, testEventMock *tangle.EventMock, nodes tangle.NodeIdentities) {
				msg := testFramework.Message("Message5")
				assert.Equal(t, epoch.Index(3), msg.EI())
			},
		},
		// Message6 TX2, issuing time epoch 5
		{
			Pre: func(t *testing.T, testFramework *tangle.MessageTestFramework, testEventMock *tangle.EventMock, nodes tangle.NodeIdentities) {
				eventHandlerMock.Expect("EpochCommitted", epoch.Index(3))
				fmt.Println("message 6")
			},

			Post: func(t *testing.T, testFramework *tangle.MessageTestFramework, testEventMock *tangle.EventMock, nodes tangle.NodeIdentities) {
				msg := testFramework.Message("Message6")
				assert.Equal(t, epoch.Index(3), msg.EI())
			},
		},
		// Message7, issuing time epoch 6
		{
			Pre: func(t *testing.T, testFramework *tangle.MessageTestFramework, testEventMock *tangle.EventMock, nodes tangle.NodeIdentities) {
				time.Sleep(epochInterval)
				eventHandlerMock.Expect("EpochCommitted", epoch.Index(4))
				fmt.Println("message 7")
			},

			Post: func(t *testing.T, testFramework *tangle.MessageTestFramework, testEventMock *tangle.EventMock, nodes tangle.NodeIdentities) {
				msg := testFramework.Message("Message7")
				assert.Equal(t, epoch.Index(4), msg.EI())
			},
		},
		// Message8, issuing time epoch 6
		{
			Pre: func(t *testing.T, testFramework *tangle.MessageTestFramework, testEventMock *tangle.EventMock, nodes tangle.NodeIdentities) {
				eventHandlerMock.Expect("EpochCommitted", epoch.Index(4))
				fmt.Println("message 8")
			},

			Post: func(t *testing.T, testFramework *tangle.MessageTestFramework, testEventMock *tangle.EventMock, nodes tangle.NodeIdentities) {
				msg := testFramework.Message("Message8")
				assert.Equal(t, epoch.Index(4), msg.EI())
				assertExistenceOfTransaction(t, testFramework, notarizationMgr, map[string]bool{
					"Message5": true,
					"Message6": true,
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

func TestManager_UpdateStateMutationTreeWithConflict(t *testing.T) {
	var epochInterval = 1 * time.Second

	processMsgScenario := tangle.NotarizationConflictTxScenario(t, tangle.WithConflictDAGOptions(conflictdag.WithMergeToMaster(false)))
	eventHandlerMock, notarizationMgr := setupFramework(t, processMsgScenario, epochInterval)
	defer func(processMsgScenario *tangle.TestScenario, t *testing.T) {
		if err := recover(); err != nil {
			t.Error(err)
			fmt.Println(string(debug.Stack()))
			return
		}

		if err := processMsgScenario.Cleanup(t); err != nil {
			require.NoError(t, err)
		}
	}(processMsgScenario, t)

	prePostSteps := []*tangle.PrePostStepTuple{
		// Message1, issuing time epoch 1
		{
			Pre: func(t *testing.T, testFramework *tangle.MessageTestFramework, testEventMock *tangle.EventMock, nodes tangle.NodeIdentities) {
				time.Sleep(epochInterval)
				eventHandlerMock.Expect("EpochCommitted", epoch.Index(0))
			},
			Post: func(t *testing.T, testFramework *tangle.MessageTestFramework, testEventMock *tangle.EventMock, nodes tangle.NodeIdentities) {
				msg := testFramework.Message("Message1")
				assert.Equal(t, epoch.Index(0), msg.EI())
			},
		},
		// Message2, issuing time epoch 1
		{
			Pre: func(t *testing.T, testFramework *tangle.MessageTestFramework, testEventMock *tangle.EventMock, nodes tangle.NodeIdentities) {
				fmt.Println("message 2")
				eventHandlerMock.Expect("EpochCommitted", epoch.Index(0))
			},
			Post: func(t *testing.T, testFramework *tangle.MessageTestFramework, testEventMock *tangle.EventMock, nodes tangle.NodeIdentities) {
				msg := testFramework.Message("Message2")
				assert.Equal(t, epoch.Index(0), msg.EI())
			},
		},
		// Message3, issuing time epoch 1
		{
			Pre: func(t *testing.T, testFramework *tangle.MessageTestFramework, testEventMock *tangle.EventMock, nodes tangle.NodeIdentities) {
				eventHandlerMock.Expect("EpochCommitted", epoch.Index(0))
				fmt.Println("message 3")
			},
			Post: func(t *testing.T, testFramework *tangle.MessageTestFramework, testEventMock *tangle.EventMock, nodes tangle.NodeIdentities) {
				msg := testFramework.Message("Message3")
				assert.Equal(t, epoch.Index(0), msg.EI())
			},
		},
		// Message4, issuing time epoch 1
		{
			Pre: func(t *testing.T, testFramework *tangle.MessageTestFramework, testEventMock *tangle.EventMock, nodes tangle.NodeIdentities) {
				eventHandlerMock.Expect("EpochCommitted", epoch.Index(0))
				fmt.Println("message 4")
			},
			Post: func(t *testing.T, testFramework *tangle.MessageTestFramework, testEventMock *tangle.EventMock, nodes tangle.NodeIdentities) {
				msg := testFramework.Message("Message4")
				assert.Equal(t, epoch.Index(0), msg.EI())
			},
		},
		// Message5 TX1, issuing time epoch 2
		{
			Pre: func(t *testing.T, testFramework *tangle.MessageTestFramework, testEventMock *tangle.EventMock, nodes tangle.NodeIdentities) {
				time.Sleep(epochInterval)
				eventHandlerMock.Expect("EpochCommitted", epoch.Index(0))
				fmt.Println("message 5")
			},

			Post: func(t *testing.T, testFramework *tangle.MessageTestFramework, testEventMock *tangle.EventMock, nodes tangle.NodeIdentities) {
				msg := testFramework.Message("Message5")
				assert.Equal(t, epoch.Index(0), msg.EI())
			},
		},
		// Message6 TX2, issuing time epoch 2
		{
			Pre: func(t *testing.T, testFramework *tangle.MessageTestFramework, testEventMock *tangle.EventMock, nodes tangle.NodeIdentities) {
				eventHandlerMock.Expect("EpochCommitted", epoch.Index(0))
				fmt.Println("message 6")
			},

			Post: func(t *testing.T, testFramework *tangle.MessageTestFramework, testEventMock *tangle.EventMock, nodes tangle.NodeIdentities) {
				msg := testFramework.Message("Message6")
				assert.Equal(t, epoch.Index(0), msg.EI())
			},
		},
		// Message7, issuing time epoch 3
		{
			Pre: func(t *testing.T, testFramework *tangle.MessageTestFramework, testEventMock *tangle.EventMock, nodes tangle.NodeIdentities) {
				time.Sleep(epochInterval)
				eventHandlerMock.Expect("EpochCommitted", epoch.Index(1))
				fmt.Println("message 7")
			},

			Post: func(t *testing.T, testFramework *tangle.MessageTestFramework, testEventMock *tangle.EventMock, nodes tangle.NodeIdentities) {
				msg := testFramework.Message("Message7")
				assert.Equal(t, epoch.Index(1), msg.EI())
			},
		},
		// Message8, issuing time epoch 4
		{
			Pre: func(t *testing.T, testFramework *tangle.MessageTestFramework, testEventMock *tangle.EventMock, nodes tangle.NodeIdentities) {
				time.Sleep(epochInterval)
				eventHandlerMock.Expect("EpochCommitted", epoch.Index(2))
				fmt.Println("message 8")
			},

			Post: func(t *testing.T, testFramework *tangle.MessageTestFramework, testEventMock *tangle.EventMock, nodes tangle.NodeIdentities) {
				msg := testFramework.Message("Message8")
				assert.Equal(t, epoch.Index(2), msg.EI())
				assertExistenceOfBlock(t, testFramework, notarizationMgr, map[string]bool{
					"Message1": true,
					"Message2": true,
					"Message3": true,
					"Message4": true,
					"Message5": true,
					"Message7": true,
					"Message6": false,
					"Message8": false,
				})
				assertExistenceOfTransaction(t, testFramework, notarizationMgr, map[string]bool{
					"Message5": true,
					"Message7": true,
					"Message6": false,
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

func setupFramework(t *testing.T, scenario *tangle.TestScenario, epochInterval time.Duration) (eventMock *EventMock, m *Manager) {
	var minCommittable time.Duration = 2 * epochInterval
	genesisTime := time.Now()
	testTangle := scenario.Tangle

	// set up finality gadget
	testOpts := []finality.Option{
		finality.WithBranchThresholdTranslation(TestBranchGoFTranslation),
		finality.WithMessageThresholdTranslation(TestMessageGoFTranslation),
	}
	sfg := finality.NewSimpleFinalityGadget(testTangle, testOpts...)
	testTangle.ConfirmationOracle = sfg

	// set up notarization manager
	ecFactory := NewEpochCommitmentFactory(testTangle.Options.Store, testTangle, 0)
	epochMgr := NewEpochManager(Duration(epochInterval), GenesisTime(genesisTime.Unix()))
	m = NewManager(epochMgr, ecFactory, testTangle, MinCommittableEpochAge(minCommittable))

	commitmentFunc := func() (ecRecord *epoch.ECRecord, latestConfirmedEpoch epoch.Index, err error) {
		ecRecord, err = m.GetLatestEC()
		require.NoError(t, err)
		latestConfirmedEpoch, err = m.LatestConfirmedEpochIndex()
		require.NoError(t, err)
		return ecRecord, latestConfirmedEpoch, nil
	}
	testTangle.Options.CommitmentFunc = commitmentFunc
	testTangle.Setup()

	registerToTangleEvents(sfg, testTangle)
	loadSnapshot(m, scenario.TestFramework)

	eventMock = NewEventMock(t, m, ecFactory)

	return eventMock, m
}

func assertExistenceOfBlock(t *testing.T, testFramework *tangle.MessageTestFramework, m *Manager, results map[string]bool) {
	for alias, result := range results {
		p, err := m.GetBlockInclusionProof(testFramework.Message(alias).ID())
		require.NoError(t, err)
		valid := m.epochCommitmentFactory.VerifyTangleRoot(*p, testFramework.Message(alias).ID())
		assert.Equal(t, result, valid)
	}
}

func assertExistenceOfTransaction(t *testing.T, testFramework *tangle.MessageTestFramework, m *Manager, results map[string]bool) {
	for alias, result := range results {
		p, err := m.GetTransactionInclusionProof(testFramework.Transaction(alias).ID())
		require.NoError(t, err)
		valid := m.epochCommitmentFactory.VerifyStateMutationRoot(*p, testFramework.TransactionID(alias))
		assert.Equal(t, result, valid)
	}
}

func testNotarizationManager() *Manager {
	t := time.Now().Add(-25 * time.Minute).Unix()
	testTangle := tangle.NewTestTangle()
	interval := 5 * time.Minute
	return NewManager(NewEpochManager(GenesisTime(t), Duration(interval)), NewEpochCommitmentFactory(testTangle.Options.Store, testTangle, 0), testTangle, MinCommittableEpochAge(10*time.Minute))
}

func loadSnapshot(m *Manager, testFramework *tangle.MessageTestFramework) {
	snapshot := testFramework.Snapshot()
	snapshot.DiffEpochIndex = epoch.Index(0)
	snapshot.FullEpochIndex = epoch.Index(0)

	var createMetadata []*ledger.OutputWithMetadata
	for _, metadata := range snapshot.OutputsWithMetadata {
		createMetadata = append(createMetadata, metadata)
	}
	snapshot.EpochDiffs = make(map[epoch.Index]*ledger.EpochDiff)
	snapshot.EpochDiffs[epoch.Index(0)] = ledger.NewEpochDiff([]*ledger.OutputWithMetadata{}, createMetadata)

	ecRecord := epoch.NewECRecord(snapshot.FullEpochIndex)
	ecRecord.SetECR(epoch.MerkleRoot{})
	ecRecord.SetPrevEC(epoch.MerkleRoot{})
	snapshot.LatestECRecord = ecRecord

	m.LoadSnapshot(snapshot)
}

func registerToTangleEvents(sfg *finality.SimpleFinalityGadget, testTangle *tangle.Tangle) {
	testTangle.ApprovalWeightManager.Events.MarkerWeightChanged.Hook(event.NewClosure(func(e *tangle.MarkerWeightChangedEvent) {
		sfg.HandleMarker(e.Marker, e.Weight)
	}))
	testTangle.ApprovalWeightManager.Events.BranchWeightChanged.Hook(event.NewClosure(func(e *tangle.BranchWeightChangedEvent) {
		sfg.HandleBranch(e.BranchID, e.Weight)
	}))
}
