package finality

import (
	"fmt"
	"runtime/debug"
	"testing"

	"github.com/iotaledger/hive.go/generics/event"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/consensus/gof"
	"github.com/iotaledger/goshimmer/packages/ledger"
	"github.com/iotaledger/goshimmer/packages/ledger/branchdag"
	"github.com/iotaledger/goshimmer/packages/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/ledger/vm/devnetvm"
	"github.com/iotaledger/goshimmer/packages/tangle"
)

type EventHandlerMock struct {
	mock.Mock
}

const (
	testingLowBound    = 0.2
	testingMediumBound = 0.3
	testingHighBound   = 0.5
)

var (
	TestBranchGoFTranslation BranchThresholdTranslation = func(branchID branchdag.BranchID, aw float64) gof.GradeOfFinality {
		switch {
		case aw >= testingLowBound && aw < testingMediumBound:
			return gof.Low
		case aw >= testingMediumBound && aw < testingHighBound:
			return gof.Medium
		case aw >= testingHighBound:
			return gof.High
		default:
			return gof.None
		}
	}

	TestMessageGoFTranslation MessageThresholdTranslation = func(aw float64) gof.GradeOfFinality {
		switch {
		case aw >= testingLowBound && aw < testingMediumBound:
			return gof.Low
		case aw >= testingMediumBound && aw < testingHighBound:
			return gof.Medium
		case aw >= testingHighBound:
			return gof.High
		default:
			return gof.None
		}
	}
)

func (handler *EventHandlerMock) MessageConfirmed(msgID tangle.MessageID) {
	handler.Called(msgID)
}

func (handler *EventHandlerMock) BranchConfirmed(branchID branchdag.BranchID) {
	handler.Called(branchID)
}

func (handler *EventHandlerMock) TransactionConfirmed(txID utxo.TransactionID) {
	handler.Called(txID)
}

func (handler *EventHandlerMock) WireUpFinalityGadget(fg Gadget, tangleInstance *tangle.Tangle) {
	fg.Events().MessageConfirmed.Hook(event.NewClosure(func(event *tangle.MessageConfirmedEvent) { handler.MessageConfirmed(event.Message.ID()) }))
	tangleInstance.Ledger.BranchDAG.Events.BranchConfirmed.Hook(event.NewClosure(func(event *branchdag.BranchConfirmedEvent) { handler.BranchConfirmed(event.BranchID) }))
	tangleInstance.Ledger.Events.TransactionConfirmed.Hook(event.NewClosure(func(event *ledger.TransactionConfirmedEvent) { handler.TransactionConfirmed(event.TransactionID) }))
}

func TestSimpleFinalityGadget(t *testing.T) {
	processMsgScenario := tangle.ProcessMessageScenario(t, tangle.WithBranchDAGOptions(branchdag.WithMergeToMaster(false)))
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
				assertMsgsGoFs(t, testFramework, map[gof.GradeOfFinality][]string{
					gof.Medium: {"Message1"},
				})
				eventHandlerMock.AssertExpectations(t)
			},
		},
		// Message2
		{
			Post: func(t *testing.T, testFramework *tangle.MessageTestFramework, testEventMock *tangle.EventMock, nodes tangle.NodeIdentities) {
				assertMsgsGoFs(t, testFramework, map[gof.GradeOfFinality][]string{
					gof.Medium: {"Message1"},
					gof.None:   {"Message2"},
				})
				eventHandlerMock.AssertExpectations(t)
			},
		},
		// Message3
		{
			Pre: func(t *testing.T, testFramework *tangle.MessageTestFramework, testEventMock *tangle.EventMock, nodes tangle.NodeIdentities) {
				eventHandlerMock.On("MessageConfirmed", testFramework.Message("Message1").ID())
			},
			Post: func(t *testing.T, testFramework *tangle.MessageTestFramework, testEventMock *tangle.EventMock, nodes tangle.NodeIdentities) {
				assertMsgsGoFs(t, testFramework, map[gof.GradeOfFinality][]string{
					gof.High:   {"Message1"},
					gof.Medium: {"Message2"},
					gof.Low:    {"Message3"},
				})
				eventHandlerMock.AssertExpectations(t)
			},
		},
		// Message4
		{
			Pre: func(t *testing.T, testFramework *tangle.MessageTestFramework, testEventMock *tangle.EventMock, nodes tangle.NodeIdentities) {
				eventHandlerMock.On("MessageConfirmed", testFramework.Message("Message2").ID())
			},
			Post: func(t *testing.T, testFramework *tangle.MessageTestFramework, testEventMock *tangle.EventMock, nodes tangle.NodeIdentities) {
				assertMsgsGoFs(t, testFramework, map[gof.GradeOfFinality][]string{
					gof.High:   {"Message1", "Message2"},
					gof.Medium: {"Message3"},
					gof.Low:    {"Message4"},
				})
				eventHandlerMock.AssertExpectations(t)
			},
		},
		// Message5
		{
			Pre: func(t *testing.T, testFramework *tangle.MessageTestFramework, testEventMock *tangle.EventMock, nodes tangle.NodeIdentities) {
				eventHandlerMock.On("MessageConfirmed", testFramework.Message("Message3").ID())
				eventHandlerMock.On("MessageConfirmed", testFramework.Message("Message4").ID())
			},
			Post: func(t *testing.T, testFramework *tangle.MessageTestFramework, testEventMock *tangle.EventMock, nodes tangle.NodeIdentities) {
				assertMsgsGoFs(t, testFramework, map[gof.GradeOfFinality][]string{
					gof.High:   {"Message1", "Message2", "Message3", "Message4"},
					gof.Medium: {"Message5"},
				})
				eventHandlerMock.AssertExpectations(t)
			},
		},
		// Message6
		{
			Post: func(t *testing.T, testFramework *tangle.MessageTestFramework, testEventMock *tangle.EventMock, nodes tangle.NodeIdentities) {
				assertMsgsGoFs(t, testFramework, map[gof.GradeOfFinality][]string{
					gof.High:   {"Message1", "Message2", "Message3", "Message4"},
					gof.Medium: {"Message5"},
					gof.None:   {"Message6"},
				})
				assertBranchsGoFs(t, testFramework, map[gof.GradeOfFinality][]string{
					gof.None: {"Message5", "Message6"},
				})
				assertTxsGoFs(t, testFramework, map[gof.GradeOfFinality][]string{
					gof.None: {"Message5", "Message6"},
				})
			},
		},
		// Message7
		{
			Pre: func(t *testing.T, testFramework *tangle.MessageTestFramework, testEventMock *tangle.EventMock, nodes tangle.NodeIdentities) {
				eventHandlerMock.On("MessageConfirmed", testFramework.Message("Message5").ID())
				eventHandlerMock.On("TransactionConfirmed", testFramework.TransactionID("Message5"))
				eventHandlerMock.On("BranchConfirmed", testFramework.BranchIDFromMessage("Message5"))
			},
			Post: func(t *testing.T, testFramework *tangle.MessageTestFramework, testEventMock *tangle.EventMock, nodes tangle.NodeIdentities) {
				assertMsgsGoFs(t, testFramework, map[gof.GradeOfFinality][]string{
					gof.High: {"Message1", "Message2", "Message3", "Message4", "Message5"},
					gof.Low:  {"Message7"},
					gof.None: {"Message6"},
				})
				assertBranchsGoFs(t, testFramework, map[gof.GradeOfFinality][]string{
					gof.High: {"Message5"},
					gof.None: {"Message6"},
				})
				assertTxsGoFs(t, testFramework, map[gof.GradeOfFinality][]string{
					gof.High: {"Message5"},
					gof.None: {"Message6", "Message7"},
				})
				eventHandlerMock.AssertExpectations(t)
			},
		},
		// Message7.1
		{
			Pre: func(t *testing.T, testFramework *tangle.MessageTestFramework, testEventMock *tangle.EventMock, nodes tangle.NodeIdentities) {
				eventHandlerMock.On("MessageConfirmed", testFramework.Message("Message7").ID())
				eventHandlerMock.On("TransactionConfirmed", testFramework.TransactionID("Message7"))
			},
			Post: func(t *testing.T, testFramework *tangle.MessageTestFramework, testEventMock *tangle.EventMock, nodes tangle.NodeIdentities) {
				assertMsgsGoFs(t, testFramework, map[gof.GradeOfFinality][]string{
					gof.High:   {"Message1", "Message2", "Message3", "Message4", "Message5", "Message7"},
					gof.Medium: {"Message7.1"},
					gof.None:   {"Message6"},
				})
				assertBranchsGoFs(t, testFramework, map[gof.GradeOfFinality][]string{
					gof.High: {"Message5"},
					gof.None: {"Message6"},
				})
				assertTxsGoFs(t, testFramework, map[gof.GradeOfFinality][]string{
					gof.High: {"Message5", "Message7"},
					gof.None: {"Message6"},
				})
				eventHandlerMock.AssertExpectations(t)
			},
		},
		// Message8
		{
			Post: func(t *testing.T, testFramework *tangle.MessageTestFramework, testEventMock *tangle.EventMock, nodes tangle.NodeIdentities) {
				assertMsgsGoFs(t, testFramework, map[gof.GradeOfFinality][]string{
					gof.High:   {"Message1", "Message2", "Message3", "Message4", "Message5", "Message7"},
					gof.Medium: {"Message7.1"},
					gof.Low:    {},
					gof.None:   {"Message6", "Message8"},
				})
				assertBranchsGoFs(t, testFramework, map[gof.GradeOfFinality][]string{
					gof.High: {"Message5"},
					gof.None: {"Message6"},
				})
				assertTxsGoFs(t, testFramework, map[gof.GradeOfFinality][]string{
					gof.High: {"Message5", "Message7"},
					gof.None: {"Message6"},
				})
			},
		},
		// Message9
		{
			Pre: func(t *testing.T, testFramework *tangle.MessageTestFramework, testEventMock *tangle.EventMock, nodes tangle.NodeIdentities) {
				eventHandlerMock.On("TransactionConfirmed", testFramework.TransactionID("Message6"))
				eventHandlerMock.On("BranchConfirmed", testFramework.BranchIDFromMessage("Message6"))
			},
			Post: func(t *testing.T, testFramework *tangle.MessageTestFramework, testEventMock *tangle.EventMock, nodes tangle.NodeIdentities) {
				assertMsgsGoFs(t, testFramework, map[gof.GradeOfFinality][]string{
					gof.High:   {"Message1", "Message2", "Message3", "Message4", "Message5", "Message7"},
					gof.Medium: {"Message7.1", "Message9", "Message6", "Message8"},
					gof.Low:    {},
					gof.None:   {},
				})
				assertBranchsGoFs(t, testFramework, map[gof.GradeOfFinality][]string{
					gof.High: {"Message6"},
					gof.None: {"Message5"},
				})
				assertTxsGoFs(t, testFramework, map[gof.GradeOfFinality][]string{
					gof.High: {"Message6"},
					gof.None: {"Message5", "Message7"},
				})
				eventHandlerMock.AssertExpectations(t)
			},
		},
		// Message10
		{
			Post: func(t *testing.T, testFramework *tangle.MessageTestFramework, testEventMock *tangle.EventMock, nodes tangle.NodeIdentities) {
				assertMsgsGoFs(t, testFramework, map[gof.GradeOfFinality][]string{
					gof.High:   {"Message1", "Message2", "Message3", "Message4", "Message5", "Message7"},
					gof.Medium: {"Message7.1", "Message9", "Message6", "Message8"},
					gof.Low:    {},
					gof.None:   {"Message10"},
				})
				assertBranchsGoFs(t, testFramework, map[gof.GradeOfFinality][]string{
					gof.High: {"Message6"},
					gof.None: {"Message5"},
				})
				assertTxsGoFs(t, testFramework, map[gof.GradeOfFinality][]string{
					gof.High: {"Message6"},
					gof.None: {"Message5", "Message7"},
				})
			},
		},
		// Message11
		{
			Post: func(t *testing.T, testFramework *tangle.MessageTestFramework, testEventMock *tangle.EventMock, nodes tangle.NodeIdentities) {
				assertMsgsGoFs(t, testFramework, map[gof.GradeOfFinality][]string{
					gof.High:   {"Message1", "Message2", "Message3", "Message4", "Message5", "Message7"},
					gof.Medium: {"Message7.1", "Message9", "Message6", "Message8"},
					gof.Low:    {},
					gof.None:   {"Message10", "Message11"},
				})
				// AW swaps back from msg6's branch to 5's, msg 7/11 (pun intended) new conflict set
				assertBranchsGoFs(t, testFramework, map[gof.GradeOfFinality][]string{
					gof.High: {"Message5"},
					gof.None: {"Message6", "Message7", "Message11"},
				})
				assertTxsGoFs(t, testFramework, map[gof.GradeOfFinality][]string{
					gof.High: {"Message5"},
					gof.None: {"Message6", "Message7", "Message11"},
				})
			},
		},
		// Message12
		{
			Pre: func(t *testing.T, testFramework *tangle.MessageTestFramework, testEventMock *tangle.EventMock, nodes tangle.NodeIdentities) {
				eventHandlerMock.On("TransactionConfirmed", testFramework.TransactionID("Message11"))
				eventHandlerMock.On("BranchConfirmed", testFramework.BranchIDFromMessage("Message11"))
			},
			Post: func(t *testing.T, testFramework *tangle.MessageTestFramework, testEventMock *tangle.EventMock, nodes tangle.NodeIdentities) {
				assertMsgsGoFs(t, testFramework, map[gof.GradeOfFinality][]string{
					gof.High:   {"Message1", "Message2", "Message3", "Message4", "Message5", "Message7"},
					gof.Medium: {"Message7.1", "Message9", "Message6", "Message8"},
					gof.Low:    {},
					gof.None:   {"Message10", "Message11", "Message12"},
				})
				assertBranchsGoFs(t, testFramework, map[gof.GradeOfFinality][]string{
					gof.High: {"Message5", "Message11"},
					gof.None: {"Message7", "Message6"},
				})
				assertTxsGoFs(t, testFramework, map[gof.GradeOfFinality][]string{
					gof.High: {"Message5", "Message11"},
					gof.None: {"Message7", "Message6"},
				})
				eventHandlerMock.AssertExpectations(t)
			},
		},
		// Message13
		{
			Post: func(t *testing.T, testFramework *tangle.MessageTestFramework, testEventMock *tangle.EventMock, nodes tangle.NodeIdentities) {
				assertMsgsGoFs(t, testFramework, map[gof.GradeOfFinality][]string{
					gof.High:   {"Message1", "Message2", "Message3", "Message4", "Message5", "Message7"},
					gof.Medium: {"Message7.1", "Message9", "Message6", "Message8"},
					gof.Low:    {},
					gof.None:   {"Message10", "Message11", "Message12", "Message13"},
				})
				assertBranchsGoFs(t, testFramework, map[gof.GradeOfFinality][]string{
					gof.High: {"Message5", "Message11"},
					gof.None: {"Message6", "Message7"},
				})
				assertTxsGoFs(t, testFramework, map[gof.GradeOfFinality][]string{
					gof.High: {"Message5", "Message11"},
					gof.None: {"Message6", "Message7"},
				})
			},
		},
		// Message14
		{
			Post: func(t *testing.T, testFramework *tangle.MessageTestFramework, testEventMock *tangle.EventMock, nodes tangle.NodeIdentities) {
				assertMsgsGoFs(t, testFramework, map[gof.GradeOfFinality][]string{
					gof.High:   {"Message1", "Message2", "Message3", "Message4", "Message5", "Message7"},
					gof.Medium: {"Message7.1", "Message9", "Message6", "Message8"},
					gof.Low:    {"Message13", "Message11", "Message12"},
					gof.None:   {"Message10", "Message14"},
				})
				assertBranchsGoFs(t, testFramework, map[gof.GradeOfFinality][]string{
					gof.High: {"Message5", "Message11"},
					gof.None: {"Message6", "Message7"},
				})
				assertTxsGoFs(t, testFramework, map[gof.GradeOfFinality][]string{
					gof.High: {"Message5", "Message11"},
					gof.None: {"Message6", "Message7"},
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
	processMsgScenario := tangle.ProcessMessageScenario2(t, tangle.WithBranchDAGOptions(branchdag.WithMergeToMaster(false)))
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
				assertMsgsGoFs(t, testFramework, map[gof.GradeOfFinality][]string{
					gof.Medium: {"Message0"},
				})
			},
		},
		// Message1
		{
			Post: func(t *testing.T, testFramework *tangle.MessageTestFramework, testEventMock *tangle.EventMock, nodes tangle.NodeIdentities) {
				assertMsgsGoFs(t, testFramework, map[gof.GradeOfFinality][]string{
					gof.None: {"Message1"},
				})
			},
		},
		// Message2
		{
			Post: func(t *testing.T, testFramework *tangle.MessageTestFramework, testEventMock *tangle.EventMock, nodes tangle.NodeIdentities) {
				assertMsgsGoFs(t, testFramework, map[gof.GradeOfFinality][]string{
					gof.Medium: {},
					gof.None:   {"Message1", "Message2"},
				})
			},
		},
		// Message3
		{
			Post: func(t *testing.T, testFramework *tangle.MessageTestFramework, testEventMock *tangle.EventMock, nodes tangle.NodeIdentities) {
				assertMsgsGoFs(t, testFramework, map[gof.GradeOfFinality][]string{
					gof.High:   {},
					gof.Medium: {},
					gof.None:   {"Message1", "Message2", "Message3"},
				})
			},
		},
		// Message4
		{
			Post: func(t *testing.T, testFramework *tangle.MessageTestFramework, testEventMock *tangle.EventMock, nodes tangle.NodeIdentities) {
				sfg.propagateGoFToMessagePastCone(testFramework.Message("Message4").ID(), gof.High)
				assertMsgsGoFs(t, testFramework, map[gof.GradeOfFinality][]string{
					gof.High:   {"Message1", "Message2", "Message3", "Message4"},
					gof.Medium: {},
					gof.Low:    {},
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

func assertMsgsGoFs(t *testing.T, testFramework *tangle.MessageTestFramework, expected map[gof.GradeOfFinality][]string) {
	for expectedGoF, msgAliases := range expected {
		for _, msgAlias := range msgAliases {
			actualGradeOfFinality := testFramework.MessageMetadata(msgAlias).GradeOfFinality()
			assert.Equal(t, expectedGoF, actualGradeOfFinality, "expected msg %s GoF to be %s but is %s", msgAlias, expectedGoF, actualGradeOfFinality)
		}
	}
}

func assertTxsGoFs(t *testing.T, testFramework *tangle.MessageTestFramework, expected map[gof.GradeOfFinality][]string) {
	for expectedGoF, msgAliases := range expected {
		for _, msgAlias := range msgAliases {
			txMeta := testFramework.TransactionMetadata(msgAlias)
			actualGradeOfFinality := txMeta.GradeOfFinality()
			assert.Equal(t, expectedGoF, actualGradeOfFinality, "expected tx %s (via msg %s) GoF to be %s but is %s", txMeta.ID(), msgAlias, expectedGoF, actualGradeOfFinality)
			// auto. also check outputs
			for _, output := range testFramework.Transaction(msgAlias).(*devnetvm.Transaction).Essence().Outputs() {
				outputGoF := testFramework.OutputMetadata(output.ID()).GradeOfFinality()
				assert.Equal(t, expectedGoF, outputGoF, "expected also tx output %s (via msg %s) GoF to be %s but is %s", output.ID(), msgAlias, expectedGoF, outputGoF)
			}
		}
	}
}

func assertBranchsGoFs(t *testing.T, testFramework *tangle.MessageTestFramework, expected map[gof.GradeOfFinality][]string) {
	for expectedGoF, msgAliases := range expected {
		for _, msgAlias := range msgAliases {
			branch := testFramework.Branch(msgAlias)
			actualGradeOfFinality := testFramework.TransactionMetadata(msgAlias).GradeOfFinality()
			assert.Equal(t, expectedGoF, actualGradeOfFinality, "expected branch %s (via msg %s) GoF to be %s but is %s", branch.ID(), msgAlias, expectedGoF, actualGradeOfFinality)
		}
	}
}

func wireUpEvents(t *testing.T, testTangle *tangle.Tangle, fg Gadget) {
	testTangle.ApprovalWeightManager.Events.MarkerWeightChanged.Hook(event.NewClosure(func(e *tangle.MarkerWeightChangedEvent) {
		if err := fg.HandleMarker(e.Marker, e.Weight); err != nil {
			t.Log(err)
		}
	}))
	testTangle.ApprovalWeightManager.Events.BranchWeightChanged.Hook(event.NewClosure(func(e *tangle.BranchWeightChangedEvent) {
		if err := fg.HandleBranch(e.BranchID, e.Weight); err != nil {
			t.Log(err)
		}
	}))
}
