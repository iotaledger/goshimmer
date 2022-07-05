package acceptance

import (
	"fmt"
	"runtime/debug"
	"testing"

	"github.com/iotaledger/hive.go/generics/event"
	"github.com/iotaledger/hive.go/types/confirmation"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/conflictdag"
	"github.com/iotaledger/goshimmer/packages/ledger"
	"github.com/iotaledger/goshimmer/packages/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/ledger/vm/devnetvm"
	"github.com/iotaledger/goshimmer/packages/tangle"
)

type EventHandlerMock struct {
	mock.Mock
}

const (
	testingAcceptanceThreshold = 0.5
)

var (
	TestBranchGoFTranslation BranchThresholdTranslation = func(branchID utxo.TransactionID, aw float64) confirmation.State {
		if aw >= testingAcceptanceThreshold {
			return confirmation.Accepted
		}

		return confirmation.Pending
	}

	TestMessageGoFTranslation MessageThresholdTranslation = func(aw float64) confirmation.State {
		if aw >= testingAcceptanceThreshold {
			return confirmation.Accepted
		}

		return confirmation.Pending
	}
)

func (handler *EventHandlerMock) MessageAccepted(msgID tangle.MessageID) {
	handler.Called(msgID)
}

func (handler *EventHandlerMock) BranchAccepted(branchID utxo.TransactionID) {
	handler.Called(branchID)
}

func (handler *EventHandlerMock) TransactionAccepted(txID utxo.TransactionID) {
	handler.Called(txID)
}

func (handler *EventHandlerMock) WireUpFinalityGadget(ag *Gadget, tangleInstance *tangle.Tangle) {
	ag.Events().MessageAccepted.Hook(event.NewClosure(func(event *tangle.MessageAcceptedEvent) { handler.MessageAccepted(event.Message.ID()) }))
	tangleInstance.Ledger.ConflictDAG.Events.BranchAccepted.Hook(event.NewClosure(func(event *conflictdag.BranchAcceptedEvent[utxo.TransactionID]) {
		handler.BranchAccepted(event.ID)
	}))
	tangleInstance.Ledger.Events.TransactionAccepted.Hook(event.NewClosure(func(event *ledger.TransactionAcceptedEvent) { handler.TransactionAccepted(event.TransactionID) }))
}

func TestSimpleFinalityGadget(t *testing.T) {
	processMsgScenario := tangle.ProcessMessageScenario(t, tangle.WithConflictDAGOptions(conflictdag.WithMergeToMaster(false)))
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

	testOpts := []Option{
		WithBranchThresholdTranslation(TestBranchGoFTranslation),
		WithMessageThresholdTranslation(TestMessageGoFTranslation),
	}

	sfg := NewSimpleFinalityGadget(processMsgScenario.Tangle, testOpts...)
	wireUpEvents(t, processMsgScenario.Tangle, sfg)

	eventHandlerMock := &EventHandlerMock{}
	eventHandlerMock.WireUpFinalityGadget(sfg, processMsgScenario.Tangle)

	prePostSteps := []*tangle.PrePostStepTuple{
		// Message1
		{
			Post: func(t *testing.T, testFramework *tangle.MessageTestFramework, testEventMock *tangle.EventMock, nodes tangle.NodeIdentities) {
				assertMsgsConfirmationState(t, testFramework, map[confirmation.State][]string{
					confirmation.Pending: {"Message1"},
				})
				eventHandlerMock.AssertExpectations(t)
			},
		},
		// Message2
		{
			Post: func(t *testing.T, testFramework *tangle.MessageTestFramework, testEventMock *tangle.EventMock, nodes tangle.NodeIdentities) {
				assertMsgsConfirmationState(t, testFramework, map[confirmation.State][]string{
					confirmation.Pending: {"Message1", "Message2"},
				})
				eventHandlerMock.AssertExpectations(t)
			},
		},
		// Message3
		{
			Pre: func(t *testing.T, testFramework *tangle.MessageTestFramework, testEventMock *tangle.EventMock, nodes tangle.NodeIdentities) {
				eventHandlerMock.On("MessageAccepted", testFramework.Message("Message1").ID())
			},
			Post: func(t *testing.T, testFramework *tangle.MessageTestFramework, testEventMock *tangle.EventMock, nodes tangle.NodeIdentities) {
				assertMsgsConfirmationState(t, testFramework, map[confirmation.State][]string{
					confirmation.Accepted: {"Message1"},
					confirmation.Pending:  {"Message2", "Message3"},
				})
				eventHandlerMock.AssertExpectations(t)
			},
		},
		// Message4
		{
			Pre: func(t *testing.T, testFramework *tangle.MessageTestFramework, testEventMock *tangle.EventMock, nodes tangle.NodeIdentities) {
				eventHandlerMock.On("MessageAccepted", testFramework.Message("Message2").ID())
			},
			Post: func(t *testing.T, testFramework *tangle.MessageTestFramework, testEventMock *tangle.EventMock, nodes tangle.NodeIdentities) {
				assertMsgsConfirmationState(t, testFramework, map[confirmation.State][]string{
					confirmation.Accepted: {"Message1", "Message2"},
					confirmation.Pending:  {"Message3", "Message4"},
				})
				eventHandlerMock.AssertExpectations(t)
			},
		},
		// Message5
		{
			Pre: func(t *testing.T, testFramework *tangle.MessageTestFramework, testEventMock *tangle.EventMock, nodes tangle.NodeIdentities) {
				eventHandlerMock.On("MessageAccepted", testFramework.Message("Message3").ID())
				eventHandlerMock.On("MessageAccepted", testFramework.Message("Message4").ID())
			},
			Post: func(t *testing.T, testFramework *tangle.MessageTestFramework, testEventMock *tangle.EventMock, nodes tangle.NodeIdentities) {
				assertMsgsConfirmationState(t, testFramework, map[confirmation.State][]string{
					confirmation.Accepted: {"Message1", "Message2", "Message3", "Message4"},
					confirmation.Pending:  {"Message5"},
				})
				eventHandlerMock.AssertExpectations(t)
			},
		},
		// Message6
		{
			Post: func(t *testing.T, testFramework *tangle.MessageTestFramework, testEventMock *tangle.EventMock, nodes tangle.NodeIdentities) {
				assertMsgsConfirmationState(t, testFramework, map[confirmation.State][]string{
					confirmation.Accepted: {"Message1", "Message2", "Message3", "Message4"},
					confirmation.Pending:  {"Message5", "Message6"},
				})
				assertBranchesConfirmationState(t, testFramework, map[confirmation.State][]string{
					confirmation.Pending: {"Message5", "Message6"},
				})
				assertTxsConfirmationState(t, testFramework, map[confirmation.State][]string{
					confirmation.Pending: {"Message5", "Message6"},
				})
			},
		},
		// Message7
		{
			Pre: func(t *testing.T, testFramework *tangle.MessageTestFramework, testEventMock *tangle.EventMock, nodes tangle.NodeIdentities) {
				eventHandlerMock.On("MessageAccepted", testFramework.Message("Message5").ID())
				eventHandlerMock.On("TransactionAccepted", testFramework.TransactionID("Message5"))
				eventHandlerMock.On("BranchAccepted", testFramework.BranchIDFromMessage("Message5"))
			},
			Post: func(t *testing.T, testFramework *tangle.MessageTestFramework, testEventMock *tangle.EventMock, nodes tangle.NodeIdentities) {
				assertMsgsConfirmationState(t, testFramework, map[confirmation.State][]string{
					confirmation.Accepted: {"Message1", "Message2", "Message3", "Message4", "Message5"},
					confirmation.Pending:  {"Message7", "Message6"},
				})
				assertBranchesConfirmationState(t, testFramework, map[confirmation.State][]string{
					confirmation.Accepted: {"Message5"},
					confirmation.Rejected: {"Message6"},
				})
				assertTxsConfirmationState(t, testFramework, map[confirmation.State][]string{
					confirmation.Accepted: {"Message5"},
					confirmation.Rejected: {"Message6"},
					confirmation.Pending:  {"Message7"},
				})
				eventHandlerMock.AssertExpectations(t)
			},
		},
		// Message7.1
		{
			Pre: func(t *testing.T, testFramework *tangle.MessageTestFramework, testEventMock *tangle.EventMock, nodes tangle.NodeIdentities) {
				eventHandlerMock.On("MessageAccepted", testFramework.Message("Message7").ID())
				eventHandlerMock.On("TransactionAccepted", testFramework.TransactionID("Message7"))
			},
			Post: func(t *testing.T, testFramework *tangle.MessageTestFramework, testEventMock *tangle.EventMock, nodes tangle.NodeIdentities) {
				assertMsgsConfirmationState(t, testFramework, map[confirmation.State][]string{
					confirmation.Accepted: {"Message1", "Message2", "Message3", "Message4", "Message5", "Message7"},
					confirmation.Pending:  {"Message7.1", "Message6"},
				})
				assertBranchesConfirmationState(t, testFramework, map[confirmation.State][]string{
					confirmation.Accepted: {"Message5"},
					confirmation.Rejected: {"Message6"},
				})
				assertTxsConfirmationState(t, testFramework, map[confirmation.State][]string{
					confirmation.Accepted: {"Message5", "Message7"},
					confirmation.Rejected: {"Message6"},
				})
				eventHandlerMock.AssertExpectations(t)
			},
		},
		// Message8
		{
			Post: func(t *testing.T, testFramework *tangle.MessageTestFramework, testEventMock *tangle.EventMock, nodes tangle.NodeIdentities) {
				assertMsgsConfirmationState(t, testFramework, map[confirmation.State][]string{
					confirmation.Accepted: {"Message1", "Message2", "Message3", "Message4", "Message5", "Message7"},
					confirmation.Pending:  {"Message7.1", "Message6", "Message8"},
				})
				assertBranchesConfirmationState(t, testFramework, map[confirmation.State][]string{
					confirmation.Accepted: {"Message5"},
					confirmation.Rejected: {"Message6"},
				})
				assertTxsConfirmationState(t, testFramework, map[confirmation.State][]string{
					confirmation.Accepted: {"Message5", "Message7"},
					confirmation.Rejected: {"Message6"},
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
}

func TestWeakVsStrongParentWalk(t *testing.T) {
	processMsgScenario := tangle.ProcessMessageScenario2(t, tangle.WithConflictDAGOptions(conflictdag.WithMergeToMaster(false)))
	defer func(processMsgScenario *tangle.TestScenario, t *testing.T) {
		if err := processMsgScenario.Cleanup(t); err != nil {
			require.NoError(t, err)
		}
	}(processMsgScenario, t)

	testOpts := []Option{
		WithBranchThresholdTranslation(TestBranchGoFTranslation),
		WithMessageThresholdTranslation(TestMessageGoFTranslation),
	}

	sfg := NewSimpleFinalityGadget(processMsgScenario.Tangle, testOpts...)
	wireUpEvents(t, processMsgScenario.Tangle, sfg)

	prePostSteps := []*tangle.PrePostStepTuple{
		// Message0
		{
			Post: func(t *testing.T, testFramework *tangle.MessageTestFramework, testEventMock *tangle.EventMock, nodes tangle.NodeIdentities) {
				assertMsgsConfirmationState(t, testFramework, map[confirmation.State][]string{
					confirmation.Pending: {"Message0"},
				})
			},
		},
		// Message1
		{
			Post: func(t *testing.T, testFramework *tangle.MessageTestFramework, testEventMock *tangle.EventMock, nodes tangle.NodeIdentities) {
				assertMsgsConfirmationState(t, testFramework, map[confirmation.State][]string{
					confirmation.Pending: {"Message1"},
				})
			},
		},
		// Message2
		{
			Post: func(t *testing.T, testFramework *tangle.MessageTestFramework, testEventMock *tangle.EventMock, nodes tangle.NodeIdentities) {
				assertMsgsConfirmationState(t, testFramework, map[confirmation.State][]string{
					confirmation.Pending: {"Message1", "Message2"},
				})
			},
		},
		// Message3
		{
			Post: func(t *testing.T, testFramework *tangle.MessageTestFramework, testEventMock *tangle.EventMock, nodes tangle.NodeIdentities) {
				assertMsgsConfirmationState(t, testFramework, map[confirmation.State][]string{
					confirmation.Accepted: {},
					confirmation.Pending:  {"Message1", "Message2", "Message3"},
				})
			},
		},
		// Message4
		{
			Post: func(t *testing.T, testFramework *tangle.MessageTestFramework, testEventMock *tangle.EventMock, nodes tangle.NodeIdentities) {
				sfg.propagateGoFToMessagePastCone(testFramework.Message("Message4").ID(), confirmation.Accepted)
				assertMsgsConfirmationState(t, testFramework, map[confirmation.State][]string{
					confirmation.Accepted: {"Message1", "Message2", "Message3", "Message4"},
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
}

func assertMsgsConfirmationState(t *testing.T, testFramework *tangle.MessageTestFramework, expected map[confirmation.State][]string) {
	for expectedGoF, msgAliases := range expected {
		for _, msgAlias := range msgAliases {
			actualConfirmationState := testFramework.MessageMetadata(msgAlias).ConfirmationState()
			assert.Equal(t, expectedGoF, actualConfirmationState, "expected msg %s ConfirmationState to be %s but is %s", msgAlias, expectedGoF, actualConfirmationState)
		}
	}
}

func assertTxsConfirmationState(t *testing.T, testFramework *tangle.MessageTestFramework, expected map[confirmation.State][]string) {
	for expectedGoF, msgAliases := range expected {
		for _, msgAlias := range msgAliases {
			txMeta := testFramework.TransactionMetadata(msgAlias)
			actualConfirmationState := txMeta.ConfirmationState()
			assert.Equal(t, expectedGoF, actualConfirmationState, "expected tx %s (via msg %s) ConfirmationState to be %s but is %s", txMeta.ID(), msgAlias, expectedGoF, actualConfirmationState)
			// auto. also check outputs
			for _, output := range testFramework.Transaction(msgAlias).(*devnetvm.Transaction).Essence().Outputs() {
				outputGoF := testFramework.OutputMetadata(output.ID()).ConfirmationState()
				assert.Equal(t, expectedGoF, outputGoF, "expected also tx output %s (via msg %s) ConfirmationState to be %s but is %s", output.ID(), msgAlias, expectedGoF, outputGoF)
			}
		}
	}
}

func assertBranchesConfirmationState(t *testing.T, testFramework *tangle.MessageTestFramework, expected map[confirmation.State][]string) {
	for expectedConfirmationState, msgAliases := range expected {
		for _, msgAlias := range msgAliases {
			branch := testFramework.Branch(msgAlias)
			actualConfirmationState := testFramework.TransactionMetadata(msgAlias).ConfirmationState()
			assert.Equal(t, expectedConfirmationState, actualConfirmationState, "expected branch %s (via msg %s) ConfirmationState to be %s but is %s", branch.ID(), msgAlias, expectedConfirmationState, actualConfirmationState)
		}
	}
}

func wireUpEvents(t *testing.T, testTangle *tangle.Tangle, ag *Gadget) {
	testTangle.ApprovalWeightManager.Events.MarkerWeightChanged.Hook(event.NewClosure(func(e *tangle.MarkerWeightChangedEvent) {
		if err := ag.HandleMarker(e.Marker, e.Weight); err != nil {
			t.Log(err)
		}
	}))
	testTangle.ApprovalWeightManager.Events.BranchWeightChanged.Hook(event.NewClosure(func(e *tangle.BranchWeightChangedEvent) {
		if err := ag.HandleBranch(e.BranchID, e.Weight); err != nil {
			t.Log(err)
		}
	}))
}
