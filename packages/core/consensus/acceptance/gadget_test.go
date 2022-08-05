package acceptance

import (
	"fmt"
	"runtime/debug"
	"testing"

	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/types/confirmation"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/core/conflictdag"

	"github.com/iotaledger/goshimmer/packages/core/ledger"
	"github.com/iotaledger/goshimmer/packages/core/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/core/ledger/vm/devnetvm"
	"github.com/iotaledger/goshimmer/packages/core/tangleold"
)

type EventHandlerMock struct {
	mock.Mock
}

const (
	testingAcceptanceThreshold = 0.5
)

var (
	TestConflictTranslation ConflictThresholdTranslation = func(conflictID utxo.TransactionID, aw float64) confirmation.State {
		if aw >= testingAcceptanceThreshold {
			return confirmation.Accepted
		}

		return confirmation.Pending
	}

	TestBlockTranslation BlockThresholdTranslation = func(aw float64) confirmation.State {
		if aw >= testingAcceptanceThreshold {
			return confirmation.Accepted
		}

		return confirmation.Pending
	}
)

func (handler *EventHandlerMock) BlockAccepted(blkID tangleold.BlockID) {
	handler.Called(blkID)
}

func (handler *EventHandlerMock) ConflictAccepted(conflictID utxo.TransactionID) {
	handler.Called(conflictID)
}

func (handler *EventHandlerMock) TransactionAccepted(txID utxo.TransactionID) {
	handler.Called(txID)
}

func (handler *EventHandlerMock) WireUpFinalityGadget(ag *Gadget, tangleInstance *tangleold.Tangle) {
	ag.Events().BlockAccepted.Hook(event.NewClosure(func(event *tangleold.BlockAcceptedEvent) { handler.BlockAccepted(event.Block.ID()) }))
	tangleInstance.Ledger.ConflictDAG.Events.ConflictAccepted.Hook(event.NewClosure(func(event *conflictdag.ConflictAcceptedEvent[utxo.TransactionID]) {
		handler.ConflictAccepted(event.ID)
	}))
	tangleInstance.Ledger.Events.TransactionAccepted.Hook(event.NewClosure(func(event *ledger.TransactionAcceptedEvent) { handler.TransactionAccepted(event.TransactionID) }))
}

func TestSimpleFinalityGadget(t *testing.T) {
	processBlkScenario := tangleold.ProcessBlockScenario(t, tangleold.WithConflictDAGOptions(conflictdag.WithMergeToMaster(false)))
	defer func(processBlkScenario *tangleold.TestScenario, t *testing.T) {
		if err := recover(); err != nil {
			t.Error(err)
			fmt.Println(string(debug.Stack()))
			return
		}

		if err := processBlkScenario.Cleanup(t); err != nil {
			require.NoError(t, err)
		}
	}(processBlkScenario, t)

	testOpts := []Option{
		WithConflictThresholdTranslation(TestConflictTranslation),
		WithBlockThresholdTranslation(TestBlockTranslation),
	}

	sfg := NewSimpleFinalityGadget(processBlkScenario.Tangle, testOpts...)
	wireUpEvents(t, processBlkScenario.Tangle, sfg)

	eventHandlerMock := &EventHandlerMock{}
	eventHandlerMock.WireUpFinalityGadget(sfg, processBlkScenario.Tangle)

	prePostSteps := []*tangleold.PrePostStepTuple{
		// Block1
		{
			Post: func(t *testing.T, testFramework *tangleold.BlockTestFramework, testEventMock *tangleold.EventMock, nodes tangleold.NodeIdentities) {
				assertBlksConfirmationState(t, testFramework, map[confirmation.State][]string{
					confirmation.Pending: {"Block1"},
				})
				eventHandlerMock.AssertExpectations(t)
			},
		},
		// Block2
		{
			Post: func(t *testing.T, testFramework *tangleold.BlockTestFramework, testEventMock *tangleold.EventMock, nodes tangleold.NodeIdentities) {
				assertBlksConfirmationState(t, testFramework, map[confirmation.State][]string{
					confirmation.Pending: {"Block1", "Block2"},
				})
				eventHandlerMock.AssertExpectations(t)
			},
		},
		// Block3
		{
			Pre: func(t *testing.T, testFramework *tangleold.BlockTestFramework, testEventMock *tangleold.EventMock, nodes tangleold.NodeIdentities) {
				eventHandlerMock.On("BlockAccepted", testFramework.Block("Block1").ID())
			},
			Post: func(t *testing.T, testFramework *tangleold.BlockTestFramework, testEventMock *tangleold.EventMock, nodes tangleold.NodeIdentities) {
				assertBlksConfirmationState(t, testFramework, map[confirmation.State][]string{
					confirmation.Accepted: {"Block1"},
					confirmation.Pending:  {"Block2", "Block3"},
				})
				eventHandlerMock.AssertExpectations(t)
			},
		},
		// Block4
		{
			Pre: func(t *testing.T, testFramework *tangleold.BlockTestFramework, testEventMock *tangleold.EventMock, nodes tangleold.NodeIdentities) {
				eventHandlerMock.On("BlockAccepted", testFramework.Block("Block2").ID())
			},
			Post: func(t *testing.T, testFramework *tangleold.BlockTestFramework, testEventMock *tangleold.EventMock, nodes tangleold.NodeIdentities) {
				assertBlksConfirmationState(t, testFramework, map[confirmation.State][]string{
					confirmation.Accepted: {"Block1", "Block2"},
					confirmation.Pending:  {"Block3", "Block4"},
				})
				eventHandlerMock.AssertExpectations(t)
			},
		},
		// Block5
		{
			Pre: func(t *testing.T, testFramework *tangleold.BlockTestFramework, testEventMock *tangleold.EventMock, nodes tangleold.NodeIdentities) {
				eventHandlerMock.On("BlockAccepted", testFramework.Block("Block3").ID())
				eventHandlerMock.On("BlockAccepted", testFramework.Block("Block4").ID())
			},
			Post: func(t *testing.T, testFramework *tangleold.BlockTestFramework, testEventMock *tangleold.EventMock, nodes tangleold.NodeIdentities) {
				assertBlksConfirmationState(t, testFramework, map[confirmation.State][]string{
					confirmation.Accepted: {"Block1", "Block2", "Block3", "Block4"},
					confirmation.Pending:  {"Block5"},
				})
				eventHandlerMock.AssertExpectations(t)
			},
		},
		// Block6
		{
			Post: func(t *testing.T, testFramework *tangleold.BlockTestFramework, testEventMock *tangleold.EventMock, nodes tangleold.NodeIdentities) {
				assertBlksConfirmationState(t, testFramework, map[confirmation.State][]string{
					confirmation.Accepted: {"Block1", "Block2", "Block3", "Block4"},
					confirmation.Pending:  {"Block5", "Block6"},
				})
				assertConflictsConfirmationState(t, testFramework, map[confirmation.State][]string{
					confirmation.Pending: {"Block5", "Block6"},
				})
				assertTxsConfirmationState(t, testFramework, map[confirmation.State][]string{
					confirmation.Pending: {"Block5", "Block6"},
				})
			},
		},
		// Block7
		{
			Pre: func(t *testing.T, testFramework *tangleold.BlockTestFramework, testEventMock *tangleold.EventMock, nodes tangleold.NodeIdentities) {
				eventHandlerMock.On("BlockAccepted", testFramework.Block("Block5").ID())
				eventHandlerMock.On("TransactionAccepted", testFramework.TransactionID("Block5"))
				eventHandlerMock.On("ConflictAccepted", testFramework.ConflictIDFromBlock("Block5"))
			},
			Post: func(t *testing.T, testFramework *tangleold.BlockTestFramework, testEventMock *tangleold.EventMock, nodes tangleold.NodeIdentities) {
				assertBlksConfirmationState(t, testFramework, map[confirmation.State][]string{
					confirmation.Accepted: {"Block1", "Block2", "Block3", "Block4", "Block5"},
					confirmation.Pending:  {"Block7", "Block6"},
				})
				assertConflictsConfirmationState(t, testFramework, map[confirmation.State][]string{
					confirmation.Accepted: {"Block5"},
					confirmation.Rejected: {"Block6"},
				})
				assertTxsConfirmationState(t, testFramework, map[confirmation.State][]string{
					confirmation.Accepted: {"Block5"},
					confirmation.Rejected: {"Block6"},
					confirmation.Pending:  {"Block7"},
				})
				eventHandlerMock.AssertExpectations(t)
			},
		},
		// Block7.1
		{
			Pre: func(t *testing.T, testFramework *tangleold.BlockTestFramework, testEventMock *tangleold.EventMock, nodes tangleold.NodeIdentities) {
				eventHandlerMock.On("BlockAccepted", testFramework.Block("Block7").ID())
				eventHandlerMock.On("TransactionAccepted", testFramework.TransactionID("Block7"))
			},
			Post: func(t *testing.T, testFramework *tangleold.BlockTestFramework, testEventMock *tangleold.EventMock, nodes tangleold.NodeIdentities) {
				assertBlksConfirmationState(t, testFramework, map[confirmation.State][]string{
					confirmation.Accepted: {"Block1", "Block2", "Block3", "Block4", "Block5", "Block7"},
					confirmation.Pending:  {"Block7.1", "Block6"},
				})
				assertConflictsConfirmationState(t, testFramework, map[confirmation.State][]string{
					confirmation.Accepted: {"Block5"},
					confirmation.Rejected: {"Block6"},
				})
				assertTxsConfirmationState(t, testFramework, map[confirmation.State][]string{
					confirmation.Accepted: {"Block5", "Block7"},
					confirmation.Rejected: {"Block6"},
				})
				eventHandlerMock.AssertExpectations(t)
			},
		},
		// Block8
		{
			Post: func(t *testing.T, testFramework *tangleold.BlockTestFramework, testEventMock *tangleold.EventMock, nodes tangleold.NodeIdentities) {
				assertBlksConfirmationState(t, testFramework, map[confirmation.State][]string{
					confirmation.Accepted: {"Block1", "Block2", "Block3", "Block4", "Block5", "Block7"},
					confirmation.Pending:  {"Block7.1", "Block6", "Block8"},
				})
				assertConflictsConfirmationState(t, testFramework, map[confirmation.State][]string{
					confirmation.Accepted: {"Block5"},
					confirmation.Rejected: {"Block6"},
				})
				assertTxsConfirmationState(t, testFramework, map[confirmation.State][]string{
					confirmation.Accepted: {"Block5", "Block7"},
					confirmation.Rejected: {"Block6"},
				})
			},
		},
	}
	for i := 0; processBlkScenario.HasNext(); i++ {
		if len(prePostSteps)-1 < i {
			processBlkScenario.Next(nil)
			continue
		}
		processBlkScenario.Next(prePostSteps[i])
	}
}

func TestWeakVsStrongParentWalk(t *testing.T) {
	processBlkScenario := tangleold.ProcessBlockScenario2(t, tangleold.WithConflictDAGOptions(conflictdag.WithMergeToMaster(false)))
	defer func(processBlkScenario *tangleold.TestScenario, t *testing.T) {
		if err := processBlkScenario.Cleanup(t); err != nil {
			require.NoError(t, err)
		}
	}(processBlkScenario, t)

	testOpts := []Option{
		WithConflictThresholdTranslation(TestConflictTranslation),
		WithBlockThresholdTranslation(TestBlockTranslation),
	}

	sfg := NewSimpleFinalityGadget(processBlkScenario.Tangle, testOpts...)
	wireUpEvents(t, processBlkScenario.Tangle, sfg)

	prePostSteps := []*tangleold.PrePostStepTuple{
		// Block0
		{
			Post: func(t *testing.T, testFramework *tangleold.BlockTestFramework, testEventMock *tangleold.EventMock, nodes tangleold.NodeIdentities) {
				assertBlksConfirmationState(t, testFramework, map[confirmation.State][]string{
					confirmation.Pending: {"Block0"},
				})
			},
		},
		// Block1
		{
			Post: func(t *testing.T, testFramework *tangleold.BlockTestFramework, testEventMock *tangleold.EventMock, nodes tangleold.NodeIdentities) {
				assertBlksConfirmationState(t, testFramework, map[confirmation.State][]string{
					confirmation.Pending: {"Block1"},
				})
			},
		},
		// Block2
		{
			Post: func(t *testing.T, testFramework *tangleold.BlockTestFramework, testEventMock *tangleold.EventMock, nodes tangleold.NodeIdentities) {
				assertBlksConfirmationState(t, testFramework, map[confirmation.State][]string{
					confirmation.Pending: {"Block1", "Block2"},
				})
			},
		},
		// Block3
		{
			Post: func(t *testing.T, testFramework *tangleold.BlockTestFramework, testEventMock *tangleold.EventMock, nodes tangleold.NodeIdentities) {
				assertBlksConfirmationState(t, testFramework, map[confirmation.State][]string{
					confirmation.Accepted: {},
					confirmation.Pending:  {"Block1", "Block2", "Block3"},
				})
			},
		},
		// Block4
		{
			Post: func(t *testing.T, testFramework *tangleold.BlockTestFramework, testEventMock *tangleold.EventMock, nodes tangleold.NodeIdentities) {
				sfg.propagateConfirmationStateToBlockPastCone(testFramework.Block("Block4").ID(), confirmation.Accepted)
				assertBlksConfirmationState(t, testFramework, map[confirmation.State][]string{
					confirmation.Accepted: {"Block1", "Block2", "Block3", "Block4"},
				})
			},
		},
	}

	for i := 0; processBlkScenario.HasNext(); i++ {
		if len(prePostSteps)-1 < i {
			processBlkScenario.Next(nil)
			continue
		}
		processBlkScenario.Next(prePostSteps[i])
	}
}

func assertBlksConfirmationState(t *testing.T, testFramework *tangleold.BlockTestFramework, expected map[confirmation.State][]string) {
	for expectedConfirmationState, blkAliases := range expected {
		for _, blkAlias := range blkAliases {
			actualConfirmationState := testFramework.BlockMetadata(blkAlias).ConfirmationState()
			assert.Equal(t, expectedConfirmationState, actualConfirmationState, "expected blk %s ConfirmationState to be %s but is %s", blkAlias, expectedConfirmationState, actualConfirmationState)
		}
	}
}

func assertTxsConfirmationState(t *testing.T, testFramework *tangleold.BlockTestFramework, expected map[confirmation.State][]string) {
	for expectedConfirmationState, blkAliases := range expected {
		for _, blkAlias := range blkAliases {
			txMeta := testFramework.TransactionMetadata(blkAlias)
			actualConfirmationState := txMeta.ConfirmationState()
			assert.Equal(t, expectedConfirmationState, actualConfirmationState, "expected tx %s (via blk %s) ConfirmationState to be %s but is %s", txMeta.ID(), blkAlias, expectedConfirmationState, actualConfirmationState)
			// auto. also check outputs
			for _, output := range testFramework.Transaction(blkAlias).(*devnetvm.Transaction).Essence().Outputs() {
				outputConfirmationState := testFramework.OutputMetadata(output.ID()).ConfirmationState()
				assert.Equal(t, expectedConfirmationState, outputConfirmationState, "expected also tx output %s (via blk %s) ConfirmationState to be %s but is %s", output.ID(), blkAlias, expectedConfirmationState, outputConfirmationState)
			}
		}
	}
}

func assertConflictsConfirmationState(t *testing.T, testFramework *tangleold.BlockTestFramework, expected map[confirmation.State][]string) {
	for expectedConfirmationState, blkAliases := range expected {
		for _, blkAlias := range blkAliases {
			conflict := testFramework.Conflict(blkAlias)
			actualConfirmationState := testFramework.TransactionMetadata(blkAlias).ConfirmationState()
			assert.Equal(t, expectedConfirmationState, actualConfirmationState, "expected conflict %s (via blk %s) ConfirmationState to be %s but is %s", conflict.ID(), blkAlias, expectedConfirmationState, actualConfirmationState)
		}
	}
}

func wireUpEvents(t *testing.T, testTangle *tangleold.Tangle, ag *Gadget) {
	testTangle.ApprovalWeightManager.Events.MarkerWeightChanged.Hook(event.NewClosure(func(e *tangleold.MarkerWeightChangedEvent) {
		if err := ag.HandleMarker(e.Marker, e.Weight); err != nil {
			t.Log(err)
		}
	}))
	testTangle.ApprovalWeightManager.Events.ConflictWeightChanged.Hook(event.NewClosure(func(e *tangleold.ConflictWeightChangedEvent) {
		if err := ag.HandleConflict(e.ConflictID, e.Weight); err != nil {
			t.Log(err)
		}
	}))
}
