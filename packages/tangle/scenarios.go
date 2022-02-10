package tangle

import (
	"math"
	"testing"
	"time"

	"github.com/iotaledger/hive.go/identity"
	"github.com/stretchr/testify/assert"

	"github.com/iotaledger/goshimmer/packages/markers"
)

// TestStep defines a test scenario step.
type TestStep func(t *testing.T, testFramework *MessageTestFramework, testEventMock *EventMock, nodes NodeIdentities)

// NodeIdentities defines a set of node identities mapped through an alias.
type NodeIdentities map[string]*identity.Identity

// TestScenario is a sequence of steps applied onto test scenario.
type TestScenario struct {
	Steps         []TestStep
	PostStep      TestStep
	Tangle        *Tangle
	stepIndex     int
	t             *testing.T
	nodes         NodeIdentities
	testFramework *MessageTestFramework
	testEventMock *EventMock
}

// Setup sets up the scenario.
func (s *TestScenario) Setup(t *testing.T) error {
	return nil
}

// Cleanup cleans up the scenario.
func (s *TestScenario) Cleanup(t *testing.T) error {
	s.Tangle.Shutdown()
	return nil
}

// HasNext returns whether the scenario has a next step.
func (s *TestScenario) HasNext() bool {
	return len(s.Steps) != s.stepIndex
}

// PrePostStepTuple is a tuple of TestStep(s) called before and after the actual test step is called.
type PrePostStepTuple struct {
	Pre  TestStep
	Post TestStep
}

// Next returns the next step or panics if non is available.
func (s *TestScenario) Next(prePostStepTuple *PrePostStepTuple) {
	step := s.Steps[s.stepIndex]

	if prePostStepTuple != nil && prePostStepTuple.Pre != nil {
		prePostStepTuple.Pre(s.t, s.testFramework, s.testEventMock, s.nodes)
	}

	step(s.t, s.testFramework, s.testEventMock, s.nodes)

	if prePostStepTuple != nil && prePostStepTuple.Post != nil {
		prePostStepTuple.Post(s.t, s.testFramework, s.testEventMock, s.nodes)
	}
	s.stepIndex++
}

// ProcessMessageScenario the approval weight and voter adjustments.
//nolint:gomnd
func ProcessMessageScenario(t *testing.T) *TestScenario {
	s := &TestScenario{t: t}
	s.nodes = make(map[string]*identity.Identity)
	for _, node := range []string{"A", "B", "C", "D", "E"} {
		s.nodes[node] = identity.GenerateIdentity()
	}

	var weightProvider *CManaWeightProvider
	manaRetrieverMock := func() map[identity.ID]float64 {
		for _, node := range s.nodes {
			weightProvider.Update(time.Now(), node.ID())
		}
		return map[identity.ID]float64{
			s.nodes["A"].ID(): 30,
			s.nodes["B"].ID(): 15,
			s.nodes["C"].ID(): 25,
			s.nodes["D"].ID(): 20,
			s.nodes["E"].ID(): 10,
		}
	}
	weightProvider = NewCManaWeightProvider(manaRetrieverMock, time.Now)

	s.Tangle = NewTestTangle(ApprovalWeights(weightProvider))
	s.Tangle.Setup()

	s.Tangle.Booker.MarkersManager.Options.MaxPastMarkerDistance = 3

	s.testEventMock = NewEventMock(t, s.Tangle.ApprovalWeightManager)
	s.testFramework = NewMessageTestFramework(s.Tangle, WithGenesisOutput("A", 500))
	s.Steps = []TestStep{
		// ISSUE Message1
		func(t *testing.T, testFramework *MessageTestFramework, testEventMock *EventMock, nodes NodeIdentities) {
			testFramework.CreateMessage("Message1", WithStrongParents("Genesis"), WithIssuer(nodes["A"].PublicKey()))

			testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(0, 1), 0.3)

			IssueAndValidateMessageApproval(t, "Message1", testEventMock, testFramework, map[string]float64{}, map[markers.Marker]float64{
				*markers.NewMarker(0, 1): 0.30,
			})
		},
		// ISSUE Message2
		func(t *testing.T, testFramework *MessageTestFramework, testEventMock *EventMock, nodes NodeIdentities) {
			testFramework.CreateMessage("Message2", WithStrongParents("Message1"), WithIssuer(nodes["B"].PublicKey()))

			testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(0, 1), 0.45)
			testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(0, 2), 0.15)

			IssueAndValidateMessageApproval(t, "Message2", testEventMock, testFramework, map[string]float64{}, map[markers.Marker]float64{
				*markers.NewMarker(0, 1): 0.45,
				*markers.NewMarker(0, 2): 0.15,
			})
		},
		// ISSUE Message3
		func(t *testing.T, testFramework *MessageTestFramework, testEventMock *EventMock, nodes NodeIdentities) {
			testFramework.CreateMessage("Message3", WithStrongParents("Message2"), WithIssuer(nodes["C"].PublicKey()))

			testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(0, 1), 0.70)
			testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(0, 2), 0.40)
			testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(0, 3), 0.25)

			IssueAndValidateMessageApproval(t, "Message3", testEventMock, testFramework, map[string]float64{}, map[markers.Marker]float64{
				*markers.NewMarker(0, 1): 0.70,
				*markers.NewMarker(0, 2): 0.40,
				*markers.NewMarker(0, 3): 0.25,
			})
		},
		// ISSUE Message4
		func(t *testing.T, testFramework *MessageTestFramework, testEventMock *EventMock, nodes NodeIdentities) {
			testFramework.CreateMessage("Message4", WithStrongParents("Message3"), WithIssuer(nodes["D"].PublicKey()))

			testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(0, 1), 0.90)
			testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(0, 2), 0.60)
			testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(0, 3), 0.45)
			testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(0, 4), 0.20)

			IssueAndValidateMessageApproval(t, "Message4", testEventMock, testFramework, map[string]float64{}, map[markers.Marker]float64{
				*markers.NewMarker(0, 1): 0.90,
				*markers.NewMarker(0, 2): 0.60,
				*markers.NewMarker(0, 3): 0.45,
				*markers.NewMarker(0, 4): 0.20,
			})
		},
		// ISSUE Message5
		func(t *testing.T, testFramework *MessageTestFramework, testEventMock *EventMock, nodes NodeIdentities) {
			testFramework.CreateMessage("Message5", WithStrongParents("Message4"), WithIssuer(nodes["A"].PublicKey()), WithInputs("A"), WithOutput("B", 500))
			testFramework.RegisterBranchID("Branch1", "Message5")

			testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(0, 2), 0.90)
			testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(0, 3), 0.75)
			testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(0, 4), 0.50)
			testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(0, 5), 0.30)

			IssueAndValidateMessageApproval(t, "Message5", testEventMock, testFramework, map[string]float64{}, map[markers.Marker]float64{
				*markers.NewMarker(0, 1): 0.90,
				*markers.NewMarker(0, 2): 0.90,
				*markers.NewMarker(0, 3): 0.75,
				*markers.NewMarker(0, 4): 0.50,
				*markers.NewMarker(0, 5): 0.30,
			})
		},
		// ISSUE Message6
		func(t *testing.T, testFramework *MessageTestFramework, testEventMock *EventMock, nodes NodeIdentities) {
			testFramework.CreateMessage("Message6", WithStrongParents("Message4"), WithIssuer(nodes["E"].PublicKey()), WithInputs("A"), WithOutput("C", 500))
			testFramework.RegisterBranchID("Branch2", "Message6")

			testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(0, 1), 1.0)
			testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(0, 2), 1.0)
			testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(0, 3), 0.85)
			testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(0, 4), 0.60)

			testEventMock.Expect("BranchWeightChanged", testFramework.BranchID("Branch1"), 0.30)
			testEventMock.Expect("BranchWeightChanged", testFramework.BranchID("Branch2"), 0.10)

			IssueAndValidateMessageApproval(t, "Message6", testEventMock, testFramework, map[string]float64{
				"Branch1": 0.3,
				"Branch2": 0.1,
			}, map[markers.Marker]float64{
				*markers.NewMarker(0, 1): 1,
				*markers.NewMarker(0, 2): 1,
				*markers.NewMarker(0, 3): 0.85,
				*markers.NewMarker(0, 4): 0.60,
				*markers.NewMarker(0, 5): 0.30,
			})
		},
		// ISSUE Message7
		func(t *testing.T, testFramework *MessageTestFramework, testEventMock *EventMock, nodes NodeIdentities) {
			testFramework.CreateMessage("Message7", WithStrongParents("Message5"), WithIssuer(nodes["C"].PublicKey()), WithInputs("B"), WithOutput("E", 500))

			testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(0, 4), 0.85)
			testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(0, 5), 0.55)
			testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(0, 6), 0.25)

			testEventMock.Expect("BranchWeightChanged", testFramework.BranchID("Branch1"), 0.55)

			IssueAndValidateMessageApproval(t, "Message7", testEventMock, testFramework, map[string]float64{
				"Branch1": 0.55,
				"Branch2": 0.1,
			}, map[markers.Marker]float64{
				*markers.NewMarker(0, 1): 1,
				*markers.NewMarker(0, 2): 1,
				*markers.NewMarker(0, 3): 0.85,
				*markers.NewMarker(0, 4): 0.85,
				*markers.NewMarker(0, 5): 0.55,
				*markers.NewMarker(0, 6): 0.25,
			})
		},
		// ISSUE Message7.1
		func(t *testing.T, testFramework *MessageTestFramework, testEventMock *EventMock, nodes NodeIdentities) {
			testFramework.CreateMessage("Message7.1", WithStrongParents("Message7"), WithIssuer(nodes["A"].PublicKey()))

			testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(0, 6), 0.55)
			testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(0, 7), 0.30)

			IssueAndValidateMessageApproval(t, "Message7.1", testEventMock, testFramework, map[string]float64{
				"Branch1": 0.55,
				"Branch2": 0.1,
			}, map[markers.Marker]float64{
				*markers.NewMarker(0, 1): 1,
				*markers.NewMarker(0, 2): 1,
				*markers.NewMarker(0, 3): 0.85,
				*markers.NewMarker(0, 4): 0.85,
				*markers.NewMarker(0, 5): 0.55,
				*markers.NewMarker(0, 6): 0.55,
				*markers.NewMarker(0, 7): 0.30,
			})
		},
		// ISSUE Message8
		func(t *testing.T, testFramework *MessageTestFramework, testEventMock *EventMock, nodes NodeIdentities) {
			testFramework.CreateMessage("Message8", WithStrongParents("Message6"), WithIssuer(nodes["D"].PublicKey()))

			testEventMock.Expect("BranchWeightChanged", testFramework.BranchID("Branch2"), 0.3)

			IssueAndValidateMessageApproval(t, "Message8", testEventMock, testFramework, map[string]float64{
				"Branch1": 0.55,
				"Branch2": 0.30,
			}, map[markers.Marker]float64{
				*markers.NewMarker(0, 1): 1,
				*markers.NewMarker(0, 2): 1,
				*markers.NewMarker(0, 3): 0.85,
				*markers.NewMarker(0, 4): 0.85,
				*markers.NewMarker(0, 5): 0.55,
				*markers.NewMarker(0, 6): 0.55,
				*markers.NewMarker(0, 7): 0.30,
			})
		},
		// ISSUE Message9
		func(t *testing.T, testFramework *MessageTestFramework, testEventMock *EventMock, nodes NodeIdentities) {
			testFramework.CreateMessage("Message9", WithStrongParents("Message8"), WithIssuer(nodes["A"].PublicKey()))

			testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(1, 5), 0.30)

			testEventMock.Expect("BranchWeightChanged", testFramework.BranchID("Branch1"), 0.25)
			testEventMock.Expect("BranchWeightChanged", testFramework.BranchID("Branch2"), 0.60)

			IssueAndValidateMessageApproval(t, "Message9", testEventMock, testFramework, map[string]float64{
				"Branch1": 0.25,
				"Branch2": 0.60,
			}, map[markers.Marker]float64{
				*markers.NewMarker(0, 1): 1,
				*markers.NewMarker(0, 2): 1,
				*markers.NewMarker(0, 3): 0.85,
				*markers.NewMarker(0, 4): 0.85,
				*markers.NewMarker(0, 5): 0.55,
				*markers.NewMarker(0, 6): 0.55,
				*markers.NewMarker(0, 7): 0.30,
				*markers.NewMarker(1, 5): 0.30,
			})
		},
		// ISSUE Message10
		func(t *testing.T, testFramework *MessageTestFramework, testEventMock *EventMock, nodes NodeIdentities) {
			testFramework.CreateMessage("Message10", WithStrongParents("Message9"), WithIssuer(nodes["B"].PublicKey()))

			testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(0, 3), 1.0)
			testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(0, 4), 1.0)
			testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(1, 5), 0.45)
			testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(1, 6), 0.15)

			testEventMock.Expect("BranchWeightChanged", testFramework.BranchID("Branch2"), 0.75)

			IssueAndValidateMessageApproval(t, "Message10", testEventMock, testFramework, map[string]float64{
				"Branch1": 0.25,
				"Branch2": 0.75,
			}, map[markers.Marker]float64{
				*markers.NewMarker(0, 1): 1,
				*markers.NewMarker(0, 2): 1,
				*markers.NewMarker(0, 3): 1,
				*markers.NewMarker(0, 4): 1,
				*markers.NewMarker(0, 5): 0.55,
				*markers.NewMarker(0, 6): 0.55,
				*markers.NewMarker(0, 7): 0.30,
				*markers.NewMarker(1, 5): 0.45,
				*markers.NewMarker(1, 6): 0.15,
			})
		},
		// ISSUE Message11
		func(t *testing.T, testFramework *MessageTestFramework, testEventMock *EventMock, nodes NodeIdentities) {
			testFramework.CreateMessage("Message11", WithStrongParents("Message5"), WithIssuer(nodes["A"].PublicKey()), WithInputs("B"), WithOutput("D", 500))
			testFramework.RegisterBranchID("Branch3", "Message7")
			testFramework.RegisterBranchID("Branch4", "Message11")

			testEventMock.Expect("BranchWeightChanged", testFramework.BranchID("Branch1"), 0.55)
			testEventMock.Expect("BranchWeightChanged", testFramework.BranchID("Branch2"), 0.45)
			testEventMock.Expect("BranchWeightChanged", testFramework.BranchID("Branch3"), 0.25)
			testEventMock.Expect("BranchWeightChanged", testFramework.BranchID("Branch4"), 0.30)

			IssueAndValidateMessageApproval(t, "Message11", testEventMock, testFramework, map[string]float64{
				"Branch1": 0.55,
				"Branch2": 0.45,
				"Branch3": 0.25,
				"Branch4": 0.30,
			}, map[markers.Marker]float64{
				*markers.NewMarker(0, 1): 1,
				*markers.NewMarker(0, 2): 1,
				*markers.NewMarker(0, 3): 1,
				*markers.NewMarker(0, 4): 1,
				*markers.NewMarker(0, 5): 0.55,
				*markers.NewMarker(0, 6): 0.55,
				*markers.NewMarker(0, 7): 0.30,
				*markers.NewMarker(1, 5): 0.45,
				*markers.NewMarker(1, 6): 0.15,
			})
		},
		// ISSUE Message12
		func(t *testing.T, testFramework *MessageTestFramework, testEventMock *EventMock, nodes NodeIdentities) {
			testFramework.CreateMessage("Message12", WithStrongParents("Message11"), WithIssuer(nodes["D"].PublicKey()))

			testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(0, 5), 0.75)

			testEventMock.Expect("BranchWeightChanged", testFramework.BranchID("Branch1"), 0.75)
			testEventMock.Expect("BranchWeightChanged", testFramework.BranchID("Branch2"), 0.25)
			testEventMock.Expect("BranchWeightChanged", testFramework.BranchID("Branch4"), 0.50)

			IssueAndValidateMessageApproval(t, "Message12", testEventMock, testFramework, map[string]float64{
				"Branch1": 0.75,
				"Branch2": 0.25,
				"Branch3": 0.25,
				"Branch4": 0.50,
			}, map[markers.Marker]float64{
				*markers.NewMarker(0, 1): 1,
				*markers.NewMarker(0, 2): 1,
				*markers.NewMarker(0, 3): 1,
				*markers.NewMarker(0, 4): 1,
				*markers.NewMarker(0, 5): 0.75,
				*markers.NewMarker(0, 6): 0.55,
				*markers.NewMarker(0, 7): 0.30,
				*markers.NewMarker(1, 5): 0.45,
				*markers.NewMarker(1, 6): 0.15,
			})
		},
		// ISSUE Message13
		func(t *testing.T, testFramework *MessageTestFramework, testEventMock *EventMock, nodes NodeIdentities) {
			testFramework.CreateMessage("Message13", WithStrongParents("Message12"), WithIssuer(nodes["E"].PublicKey()))

			testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(0, 5), 0.85)
			testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(2, 6), 0.10)

			testEventMock.Expect("BranchWeightChanged", testFramework.BranchID("Branch1"), 0.85)
			testEventMock.Expect("BranchWeightChanged", testFramework.BranchID("Branch2"), 0.15)
			testEventMock.Expect("BranchWeightChanged", testFramework.BranchID("Branch4"), 0.60)

			IssueAndValidateMessageApproval(t, "Message13", testEventMock, testFramework, map[string]float64{
				"Branch1": 0.85,
				"Branch2": 0.15,
				"Branch3": 0.25,
				"Branch4": 0.60,
			}, map[markers.Marker]float64{
				*markers.NewMarker(0, 1): 1,
				*markers.NewMarker(0, 2): 1,
				*markers.NewMarker(0, 3): 1,
				*markers.NewMarker(0, 4): 1,
				*markers.NewMarker(0, 5): 0.85,
				*markers.NewMarker(0, 6): 0.55,
				*markers.NewMarker(0, 7): 0.30,
				*markers.NewMarker(1, 5): 0.45,
				*markers.NewMarker(1, 6): 0.15,
				*markers.NewMarker(2, 6): 0.10,
			})
		},
		// ISSUE Message14
		func(t *testing.T, testFramework *MessageTestFramework, testEventMock *EventMock, nodes NodeIdentities) {
			testFramework.CreateMessage("Message14", WithStrongParents("Message13"), WithIssuer(nodes["B"].PublicKey()))

			testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(0, 5), 1.00)
			testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(2, 6), 0.25)
			testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(2, 7), 0.15)

			testEventMock.Expect("BranchWeightChanged", testFramework.BranchID("Branch1"), 1.0)
			testEventMock.Expect("BranchWeightChanged", testFramework.BranchID("Branch2"), 0.0)
			testEventMock.Expect("BranchWeightChanged", testFramework.BranchID("Branch4"), 0.75)

			IssueAndValidateMessageApproval(t, "Message14", testEventMock, testFramework, map[string]float64{
				"Branch1": 1,
				"Branch2": 0,
				"Branch3": 0.25,
				"Branch4": 0.75,
			}, map[markers.Marker]float64{
				*markers.NewMarker(0, 1): 1,
				*markers.NewMarker(0, 2): 1,
				*markers.NewMarker(0, 3): 1,
				*markers.NewMarker(0, 4): 1,
				*markers.NewMarker(0, 5): 1,
				*markers.NewMarker(0, 6): 0.55,
				*markers.NewMarker(0, 7): 0.30,
				*markers.NewMarker(1, 5): 0.45,
				*markers.NewMarker(1, 6): 0.15,
				*markers.NewMarker(2, 6): 0.25,
				*markers.NewMarker(2, 7): 0.15,
			})
		},
	}
	return s
}

// ProcessMessageScenario2 creates a scenario useful to validate strong / weak propagation paths.
//nolint:gomnd
func ProcessMessageScenario2(t *testing.T) *TestScenario {
	s := &TestScenario{t: t}
	s.nodes = make(map[string]*identity.Identity)
	for _, node := range []string{"A", "B", "C", "D", "E"} {
		s.nodes[node] = identity.GenerateIdentity()
	}

	var weightProvider *CManaWeightProvider
	manaRetrieverMock := func() map[identity.ID]float64 {
		for _, node := range s.nodes {
			weightProvider.Update(time.Now(), node.ID())
		}
		return map[identity.ID]float64{
			s.nodes["A"].ID(): 30,
			s.nodes["B"].ID(): 15,
			s.nodes["C"].ID(): 25,
			s.nodes["D"].ID(): 20,
			s.nodes["E"].ID(): 10,
		}
	}
	weightProvider = NewCManaWeightProvider(manaRetrieverMock, time.Now)

	s.Tangle = NewTestTangle(ApprovalWeights(weightProvider))
	s.Tangle.Booker.MarkersManager.Options.MaxPastMarkerDistance = 2
	s.Tangle.Setup()

	s.testEventMock = NewEventMock(t, s.Tangle.ApprovalWeightManager)
	s.testFramework = NewMessageTestFramework(s.Tangle, WithGenesisOutput("A", 500))
	s.Steps = []TestStep{
		// ISSUE Message0
		func(t *testing.T, testFramework *MessageTestFramework, testEventMock *EventMock, nodes NodeIdentities) {
			testFramework.CreateMessage("Message0", WithStrongParents("Genesis"), WithIssuer(nodes["A"].PublicKey()))

			testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(0, 1), 0.30)

			IssueAndValidateMessageApproval(t, "Message0", testEventMock, testFramework, map[string]float64{}, map[markers.Marker]float64{
				*markers.NewMarker(0, 1): 0.30,
			})
		},
		// ISSUE Message1
		func(t *testing.T, testFramework *MessageTestFramework, testEventMock *EventMock, nodes NodeIdentities) {
			testFramework.CreateMessage("Message1", WithStrongParents("Genesis"), WithIssuer(nodes["A"].PublicKey()))

			IssueAndValidateMessageApproval(t, "Message1", testEventMock, testFramework, map[string]float64{}, map[markers.Marker]float64{
				*markers.NewMarker(0, 1): 0.30,
			})
		},
		// ISSUE Message2
		func(t *testing.T, testFramework *MessageTestFramework, testEventMock *EventMock, nodes NodeIdentities) {
			testFramework.CreateMessage("Message2", WithStrongParents("Message1"), WithIssuer(nodes["B"].PublicKey()))

			testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(1, 1), 0.15)

			IssueAndValidateMessageApproval(t, "Message2", testEventMock, testFramework, map[string]float64{}, map[markers.Marker]float64{
				*markers.NewMarker(0, 1): 0.30,
				*markers.NewMarker(1, 1): 0.15,
			})
		},
		// ISSUE Message3
		func(t *testing.T, testFramework *MessageTestFramework, testEventMock *EventMock, nodes NodeIdentities) {
			testFramework.CreateMessage("Message3", WithStrongParents("Message2"), WithIssuer(nodes["B"].PublicKey()))

			testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(1, 2), 0.15)

			IssueAndValidateMessageApproval(t, "Message3", testEventMock, testFramework, map[string]float64{}, map[markers.Marker]float64{
				*markers.NewMarker(0, 1): 0.30,
				*markers.NewMarker(1, 1): 0.15,
				*markers.NewMarker(1, 2): 0.15,
			})
		},
		// ISSUE Message4
		func(t *testing.T, testFramework *MessageTestFramework, testEventMock *EventMock, nodes NodeIdentities) {
			testFramework.CreateMessage("Message4", WithWeakParents("Message2"), WithStrongParents("Message3"), WithIssuer(nodes["D"].PublicKey()))

			testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(1, 1), 0.35)
			testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(1, 2), 0.35)
			testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(1, 3), 0.20)

			IssueAndValidateMessageApproval(t, "Message4", testEventMock, testFramework, map[string]float64{}, map[markers.Marker]float64{
				*markers.NewMarker(0, 1): 0.30,
				*markers.NewMarker(1, 1): 0.35,
				*markers.NewMarker(1, 2): 0.35,
				*markers.NewMarker(1, 3): 0.20,
			})
		},
	}
	return s
}

// IssueAndValidateMessageApproval issues the msg by the given alias and assets the expected weights.
//nolint:gomnd
func IssueAndValidateMessageApproval(t *testing.T, messageAlias string, eventMock *EventMock, testFramework *MessageTestFramework, expectedBranchWeights map[string]float64, expectedMarkerWeights map[markers.Marker]float64) {
	eventMock.Expect("MessageProcessed", testFramework.Message(messageAlias).ID())

	t.Logf("ISSUE:\tMessageID(%s)", messageAlias)
	testFramework.IssueMessages(messageAlias).WaitApprovalWeightProcessed()

	for branchAlias, expectedWeight := range expectedBranchWeights {
		branchID := testFramework.BranchID(branchAlias)
		actualWeight := testFramework.tangle.ApprovalWeightManager.WeightOfBranch(branchID)
		if expectedWeight != actualWeight {
			assert.True(t, math.Abs(actualWeight-expectedWeight) < 0.001, "weight of %s (%0.2f) not equal to expected value %0.2f", branchID, actualWeight, expectedWeight)
		}
	}

	for marker, expectedWeight := range expectedMarkerWeights {
		actualWeight := testFramework.tangle.ApprovalWeightManager.WeightOfMarker(&marker, testFramework.Message(messageAlias).IssuingTime())
		if expectedWeight != actualWeight {
			assert.True(t, math.Abs(actualWeight-expectedWeight) < 0.001, "weight of %s (%0.2f) not equal to expected value %0.2f", marker.String(), actualWeight, expectedWeight)
		}
	}

	eventMock.AssertExpectations(t)
}
