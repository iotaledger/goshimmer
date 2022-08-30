package tangleold

import (
	"math"
	"testing"
	"time"

	"github.com/iotaledger/hive.go/core/identity"
	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/markers"
)

// TestStep defines a test scenario step.
type TestStep func(t *testing.T, testFramework *BlockTestFramework, testEventMock *EventMock, nodes NodeIdentities)

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
	TestFramework *BlockTestFramework
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
		prePostStepTuple.Pre(s.t, s.TestFramework, s.testEventMock, s.nodes)
	}

	step(s.t, s.TestFramework, s.testEventMock, s.nodes)

	if prePostStepTuple != nil && prePostStepTuple.Post != nil {
		prePostStepTuple.Post(s.t, s.TestFramework, s.testEventMock, s.nodes)
	}
	s.stepIndex++
}

// ProcessBlockScenario the approval weight and voter adjustments.
//nolint:gomnd
func ProcessBlockScenario(t *testing.T, options ...Option) *TestScenario {
	s := &TestScenario{t: t}
	s.nodes = make(map[string]*identity.Identity)
	for _, node := range []string{"A", "B", "C", "D", "E"} {
		s.nodes[node] = identity.GenerateIdentity()
	}

	var weightProvider *CManaWeightProvider
	manaRetrieverMock := func() map[identity.ID]float64 {
		for _, node := range s.nodes {
			weightProvider.Update(1, node.ID())
		}
		return map[identity.ID]float64{
			s.nodes["A"].ID(): 30,
			s.nodes["B"].ID(): 15,
			s.nodes["C"].ID(): 25,
			s.nodes["D"].ID(): 20,
			s.nodes["E"].ID(): 10,
		}
	}
	testEpoch := epoch.IndexFromTime(time.Now())
	epochRetrieverFunc := func() epoch.Index { return testEpoch }
	timeProvider := func() time.Time { return epochRetrieverFunc().StartTime() }
	confirmedRetrieverFunc := func() epoch.Index { return 0 }
	weightProvider = NewCManaWeightProvider(manaRetrieverMock, timeProvider, confirmedRetrieverFunc)

	s.Tangle = NewTestTangle(append([]Option{
		ApprovalWeights(weightProvider),
	}, options...)...)
	s.Tangle.Setup()

	s.Tangle.Booker.MarkersManager.Options.MaxPastMarkerDistance = 3

	s.testEventMock = NewEventMock(t, s.Tangle.ApprovalWeightManager)
	s.TestFramework = NewBlockTestFramework(s.Tangle, WithGenesisOutput("A", 500))

	s.Steps = []TestStep{
		// ISSUE Block1
		func(t *testing.T, testFramework *BlockTestFramework, testEventMock *EventMock, nodes NodeIdentities) {
			// Make all nodes active
			for node := range nodes {
				weightProvider.Update(epochRetrieverFunc(), nodes[node].ID())
			}
			testFramework.CreateBlock("Block1", WithStrongParents("Genesis"), WithIssuer(nodes["A"].PublicKey()))

			testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(0, 1), 0.3)

			IssueAndValidateBlockApproval(t, "Block1", testEventMock, testFramework, map[string]float64{}, map[markers.Marker]float64{
				markers.NewMarker(0, 1): 0.30,
			})
		},
		// ISSUE Block2
		func(t *testing.T, testFramework *BlockTestFramework, testEventMock *EventMock, nodes NodeIdentities) {
			testFramework.CreateBlock("Block2", WithStrongParents("Block1"), WithIssuer(nodes["B"].PublicKey()))

			testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(0, 1), 0.45)
			testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(0, 2), 0.15)

			IssueAndValidateBlockApproval(t, "Block2", testEventMock, testFramework, map[string]float64{}, map[markers.Marker]float64{
				markers.NewMarker(0, 1): 0.45,
				markers.NewMarker(0, 2): 0.15,
			})
		},
		// ISSUE Block3
		func(t *testing.T, testFramework *BlockTestFramework, testEventMock *EventMock, nodes NodeIdentities) {
			testFramework.CreateBlock("Block3", WithStrongParents("Block2"), WithIssuer(nodes["C"].PublicKey()))

			testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(0, 1), 0.70)
			testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(0, 2), 0.40)
			testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(0, 3), 0.25)

			IssueAndValidateBlockApproval(t, "Block3", testEventMock, testFramework, map[string]float64{}, map[markers.Marker]float64{
				markers.NewMarker(0, 1): 0.70,
				markers.NewMarker(0, 2): 0.40,
				markers.NewMarker(0, 3): 0.25,
			})
		},
		// ISSUE Block4
		func(t *testing.T, testFramework *BlockTestFramework, testEventMock *EventMock, nodes NodeIdentities) {
			testFramework.CreateBlock("Block4", WithStrongParents("Block3"), WithIssuer(nodes["D"].PublicKey()))

			testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(0, 1), 0.90)
			testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(0, 2), 0.60)
			testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(0, 3), 0.45)
			testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(0, 4), 0.20)

			IssueAndValidateBlockApproval(t, "Block4", testEventMock, testFramework, map[string]float64{}, map[markers.Marker]float64{
				markers.NewMarker(0, 1): 0.90,
				markers.NewMarker(0, 2): 0.60,
				markers.NewMarker(0, 3): 0.45,
				markers.NewMarker(0, 4): 0.20,
			})
		},
		// ISSUE Block5
		func(t *testing.T, testFramework *BlockTestFramework, testEventMock *EventMock, nodes NodeIdentities) {
			testFramework.CreateBlock("Block5", WithStrongParents("Block4"), WithIssuer(nodes["A"].PublicKey()), WithInputs("A"), WithOutput("B", 500))
			testFramework.RegisterConflictID("Conflict1", "Block5")

			testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(0, 2), 0.90)
			testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(0, 3), 0.75)
			testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(0, 4), 0.50)
			testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(0, 5), 0.30)

			IssueAndValidateBlockApproval(t, "Block5", testEventMock, testFramework, map[string]float64{}, map[markers.Marker]float64{
				markers.NewMarker(0, 1): 0.90,
				markers.NewMarker(0, 2): 0.90,
				markers.NewMarker(0, 3): 0.75,
				markers.NewMarker(0, 4): 0.50,
				markers.NewMarker(0, 5): 0.30,
			})
		},
		// ISSUE Block6
		func(t *testing.T, testFramework *BlockTestFramework, testEventMock *EventMock, nodes NodeIdentities) {
			testFramework.CreateBlock("Block6", WithStrongParents("Block4"), WithIssuer(nodes["E"].PublicKey()), WithInputs("A"), WithOutput("C", 500))
			testFramework.RegisterConflictID("Conflict2", "Block6")

			testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(0, 1), 1.0)
			testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(0, 2), 1.0)
			testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(0, 3), 0.85)
			testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(0, 4), 0.60)

			testEventMock.Expect("ConflictWeightChanged", testFramework.ConflictID("Conflict1"), 0.30)
			testEventMock.Expect("ConflictWeightChanged", testFramework.ConflictID("Conflict2"), 0.10)

			IssueAndValidateBlockApproval(t, "Block6", testEventMock, testFramework, map[string]float64{
				"Conflict1": 0.3,
				"Conflict2": 0.1,
			}, map[markers.Marker]float64{
				markers.NewMarker(0, 1): 1,
				markers.NewMarker(0, 2): 1,
				markers.NewMarker(0, 3): 0.85,
				markers.NewMarker(0, 4): 0.60,
				markers.NewMarker(0, 5): 0.30,
			})
		},
		// ISSUE Block7
		func(t *testing.T, testFramework *BlockTestFramework, testEventMock *EventMock, nodes NodeIdentities) {
			testFramework.CreateBlock("Block7", WithStrongParents("Block5"), WithIssuer(nodes["C"].PublicKey()), WithInputs("B"), WithOutput("E", 500))

			testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(0, 4), 0.85)
			testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(0, 5), 0.55)
			testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(0, 6), 0.25)

			testEventMock.Expect("ConflictWeightChanged", testFramework.ConflictID("Conflict1"), 0.55)

			IssueAndValidateBlockApproval(t, "Block7", testEventMock, testFramework, map[string]float64{
				"Conflict1": 0.55,
				"Conflict2": 0.1,
			}, map[markers.Marker]float64{
				markers.NewMarker(0, 1): 1,
				markers.NewMarker(0, 2): 1,
				markers.NewMarker(0, 3): 0.85,
				markers.NewMarker(0, 4): 0.85,
				markers.NewMarker(0, 5): 0.55,
				markers.NewMarker(0, 6): 0.25,
			})
		},
		// ISSUE Block7.1
		func(t *testing.T, testFramework *BlockTestFramework, testEventMock *EventMock, nodes NodeIdentities) {
			testFramework.CreateBlock("Block7.1", WithStrongParents("Block7"), WithIssuer(nodes["A"].PublicKey()))

			testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(0, 6), 0.55)
			testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(0, 7), 0.30)

			IssueAndValidateBlockApproval(t, "Block7.1", testEventMock, testFramework, map[string]float64{
				"Conflict1": 0.55,
				"Conflict2": 0.1,
			}, map[markers.Marker]float64{
				markers.NewMarker(0, 1): 1,
				markers.NewMarker(0, 2): 1,
				markers.NewMarker(0, 3): 0.85,
				markers.NewMarker(0, 4): 0.85,
				markers.NewMarker(0, 5): 0.55,
				markers.NewMarker(0, 6): 0.55,
				markers.NewMarker(0, 7): 0.30,
			})
		},
		// ISSUE Block8
		func(t *testing.T, testFramework *BlockTestFramework, testEventMock *EventMock, nodes NodeIdentities) {
			testFramework.CreateBlock("Block8", WithStrongParents("Block6"), WithIssuer(nodes["D"].PublicKey()))

			testEventMock.Expect("ConflictWeightChanged", testFramework.ConflictID("Conflict2"), 0.3)

			IssueAndValidateBlockApproval(t, "Block8", testEventMock, testFramework, map[string]float64{
				"Conflict1": 0.55,
				"Conflict2": 0.30,
			}, map[markers.Marker]float64{
				markers.NewMarker(0, 1): 1,
				markers.NewMarker(0, 2): 1,
				markers.NewMarker(0, 3): 0.85,
				markers.NewMarker(0, 4): 0.85,
				markers.NewMarker(0, 5): 0.55,
				markers.NewMarker(0, 6): 0.55,
				markers.NewMarker(0, 7): 0.30,
			})
		},
		// TODO: this tests reorg, which is not yet supported
		// ISSUE Block9
		// 	func(t *testing.T, testFramework *BlockTestFramework, testEventMock *EventMock, nodes NodeIdentities) {
		// 		testFramework.CreateBlock("Block9", WithStrongParents("Block8"), WithIssuer(nodes["A"].PublicKey()))
		//
		// 		testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(1, 5), 0.30)
		//
		// 		testEventMock.Expect("ConflictWeightChanged", testFramework.ConflictID("Conflict1"), 0.25)
		// 		testEventMock.Expect("ConflictWeightChanged", testFramework.ConflictID("Conflict2"), 0.60)
		//
		// 		IssueAndValidateBlockApproval(t, "Block9", testEventMock, testFramework, map[string]float64{
		// 			"Conflict1": 0.25,
		// 			"Conflict2": 0.60,
		// 		}, map[markers.Marker]float64{
		// 			markers.NewMarker(0, 1): 1,
		// 			markers.NewMarker(0, 2): 1,
		// 			markers.NewMarker(0, 3): 0.85,
		// 			markers.NewMarker(0, 4): 0.85,
		// 			markers.NewMarker(0, 5): 0.55,
		// 			markers.NewMarker(0, 6): 0.55,
		// 			markers.NewMarker(0, 7): 0.30,
		// 			markers.NewMarker(1, 5): 0.30,
		// 		})
		// 	},
		// 	// ISSUE Block10
		// 	func(t *testing.T, testFramework *BlockTestFramework, testEventMock *EventMock, nodes NodeIdentities) {
		// 		testFramework.CreateBlock("Block10", WithStrongParents("Block9"), WithIssuer(nodes["B"].PublicKey()))
		//
		// 		testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(0, 3), 1.0)
		// 		testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(0, 4), 1.0)
		// 		testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(1, 5), 0.45)
		// 		testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(1, 6), 0.15)
		//
		// 		testEventMock.Expect("ConflictWeightChanged", testFramework.ConflictID("Conflict2"), 0.75)
		//
		// 		IssueAndValidateBlockApproval(t, "Block10", testEventMock, testFramework, map[string]float64{
		// 			"Conflict1": 0.25,
		// 			"Conflict2": 0.75,
		// 		}, map[markers.Marker]float64{
		// 			markers.NewMarker(0, 1): 1,
		// 			markers.NewMarker(0, 2): 1,
		// 			markers.NewMarker(0, 3): 1,
		// 			markers.NewMarker(0, 4): 1,
		// 			markers.NewMarker(0, 5): 0.55,
		// 			markers.NewMarker(0, 6): 0.55,
		// 			markers.NewMarker(0, 7): 0.30,
		// 			markers.NewMarker(1, 5): 0.45,
		// 			markers.NewMarker(1, 6): 0.15,
		// 		})
		// 	},
		// 	// ISSUE Block11
		// 	func(t *testing.T, testFramework *BlockTestFramework, testEventMock *EventMock, nodes NodeIdentities) {
		// 		testFramework.CreateBlock("Block11", WithStrongParents("Block5"), WithIssuer(nodes["A"].PublicKey()), WithInputs("B"), WithOutput("D", 500))
		// 		testFramework.RegisterConflictID("Conflict3", "Block7")
		// 		testFramework.RegisterConflictID("Conflict4", "Block11")
		//
		// 		testEventMock.Expect("ConflictWeightChanged", testFramework.ConflictID("Conflict1"), 0.55)
		// 		testEventMock.Expect("ConflictWeightChanged", testFramework.ConflictID("Conflict2"), 0.45)
		// 		testEventMock.Expect("ConflictWeightChanged", testFramework.ConflictID("Conflict3"), 0.25)
		// 		testEventMock.Expect("ConflictWeightChanged", testFramework.ConflictID("Conflict4"), 0.30)
		//
		// 		IssueAndValidateBlockApproval(t, "Block11", testEventMock, testFramework, map[string]float64{
		// 			"Conflict1": 0.55,
		// 			"Conflict2": 0.45,
		// 			"Conflict3": 0.25,
		// 			"Conflict4": 0.30,
		// 		}, map[markers.Marker]float64{
		// 			markers.NewMarker(0, 1): 1,
		// 			markers.NewMarker(0, 2): 1,
		// 			markers.NewMarker(0, 3): 1,
		// 			markers.NewMarker(0, 4): 1,
		// 			markers.NewMarker(0, 5): 0.55,
		// 			markers.NewMarker(0, 6): 0.55,
		// 			markers.NewMarker(0, 7): 0.30,
		// 			markers.NewMarker(1, 5): 0.45,
		// 			markers.NewMarker(1, 6): 0.15,
		// 		})
		// 	},
		// 	// ISSUE Block12
		// 	func(t *testing.T, testFramework *BlockTestFramework, testEventMock *EventMock, nodes NodeIdentities) {
		// 		testFramework.CreateBlock("Block12", WithStrongParents("Block11"), WithIssuer(nodes["D"].PublicKey()))
		//
		// 		testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(0, 5), 0.75)
		//
		// 		testEventMock.Expect("ConflictWeightChanged", testFramework.ConflictID("Conflict1"), 0.75)
		// 		testEventMock.Expect("ConflictWeightChanged", testFramework.ConflictID("Conflict2"), 0.25)
		// 		testEventMock.Expect("ConflictWeightChanged", testFramework.ConflictID("Conflict4"), 0.50)
		//
		// 		IssueAndValidateBlockApproval(t, "Block12", testEventMock, testFramework, map[string]float64{
		// 			"Conflict1": 0.75,
		// 			"Conflict2": 0.25,
		// 			"Conflict3": 0.25,
		// 			"Conflict4": 0.50,
		// 		}, map[markers.Marker]float64{
		// 			markers.NewMarker(0, 1): 1,
		// 			markers.NewMarker(0, 2): 1,
		// 			markers.NewMarker(0, 3): 1,
		// 			markers.NewMarker(0, 4): 1,
		// 			markers.NewMarker(0, 5): 0.75,
		// 			markers.NewMarker(0, 6): 0.55,
		// 			markers.NewMarker(0, 7): 0.30,
		// 			markers.NewMarker(1, 5): 0.45,
		// 			markers.NewMarker(1, 6): 0.15,
		// 		})
		// 	},
		// 	// ISSUE Block13
		// 	func(t *testing.T, testFramework *BlockTestFramework, testEventMock *EventMock, nodes NodeIdentities) {
		// 		testFramework.CreateBlock("Block13", WithStrongParents("Block12"), WithIssuer(nodes["E"].PublicKey()))
		//
		// 		testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(0, 5), 0.85)
		// 		testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(2, 6), 0.10)
		//
		// 		testEventMock.Expect("ConflictWeightChanged", testFramework.ConflictID("Conflict1"), 0.85)
		// 		testEventMock.Expect("ConflictWeightChanged", testFramework.ConflictID("Conflict2"), 0.15)
		// 		testEventMock.Expect("ConflictWeightChanged", testFramework.ConflictID("Conflict4"), 0.60)
		//
		// 		IssueAndValidateBlockApproval(t, "Block13", testEventMock, testFramework, map[string]float64{
		// 			"Conflict1": 0.85,
		// 			"Conflict2": 0.15,
		// 			"Conflict3": 0.25,
		// 			"Conflict4": 0.60,
		// 		}, map[markers.Marker]float64{
		// 			markers.NewMarker(0, 1): 1,
		// 			markers.NewMarker(0, 2): 1,
		// 			markers.NewMarker(0, 3): 1,
		// 			markers.NewMarker(0, 4): 1,
		// 			markers.NewMarker(0, 5): 0.85,
		// 			markers.NewMarker(0, 6): 0.55,
		// 			markers.NewMarker(0, 7): 0.30,
		// 			markers.NewMarker(1, 5): 0.45,
		// 			markers.NewMarker(1, 6): 0.15,
		// 			markers.NewMarker(2, 6): 0.10,
		// 		})
		// 	},
		// 	// ISSUE Block14
		// 	func(t *testing.T, testFramework *BlockTestFramework, testEventMock *EventMock, nodes NodeIdentities) {
		// 		testFramework.CreateBlock("Block14", WithStrongParents("Block13"), WithIssuer(nodes["B"].PublicKey()))
		//
		// 		testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(0, 5), 1.00)
		// 		testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(2, 6), 0.25)
		// 		testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(2, 7), 0.15)
		//
		// 		testEventMock.Expect("ConflictWeightChanged", testFramework.ConflictID("Conflict1"), 1.0)
		// 		testEventMock.Expect("ConflictWeightChanged", testFramework.ConflictID("Conflict2"), 0.0)
		// 		testEventMock.Expect("ConflictWeightChanged", testFramework.ConflictID("Conflict4"), 0.75)
		//
		// 		IssueAndValidateBlockApproval(t, "Block14", testEventMock, testFramework, map[string]float64{
		// 			"Conflict1": 1,
		// 			"Conflict2": 0,
		// 			"Conflict3": 0.25,
		// 			"Conflict4": 0.75,
		// 		}, map[markers.Marker]float64{
		// 			markers.NewMarker(0, 1): 1,
		// 			markers.NewMarker(0, 2): 1,
		// 			markers.NewMarker(0, 3): 1,
		// 			markers.NewMarker(0, 4): 1,
		// 			markers.NewMarker(0, 5): 1,
		// 			markers.NewMarker(0, 6): 0.55,
		// 			markers.NewMarker(0, 7): 0.30,
		// 			markers.NewMarker(1, 5): 0.45,
		// 			markers.NewMarker(1, 6): 0.15,
		// 			markers.NewMarker(2, 6): 0.25,
		// 			markers.NewMarker(2, 7): 0.15,
		// 		})
		// 	},
	}
	return s
}

// ProcessBlockScenario2 creates a scenario useful to validate strong / weak propagation paths.
//nolint:gomnd
func ProcessBlockScenario2(t *testing.T, options ...Option) *TestScenario {
	s := &TestScenario{t: t}
	s.nodes = make(map[string]*identity.Identity)
	for _, node := range []string{"A", "B", "C", "D", "E"} {
		s.nodes[node] = identity.GenerateIdentity()
	}

	var weightProvider *CManaWeightProvider
	manaRetrieverMock := func() map[identity.ID]float64 {
		for _, node := range s.nodes {
			weightProvider.Update(epoch.Index(1), node.ID())
		}
		return map[identity.ID]float64{
			s.nodes["A"].ID(): 30,
			s.nodes["B"].ID(): 15,
			s.nodes["C"].ID(): 25,
			s.nodes["D"].ID(): 20,
			s.nodes["E"].ID(): 10,
		}
	}
	testEpoch := epoch.IndexFromTime(time.Now())
	epochRetrieverFunc := func() epoch.Index { return testEpoch }
	timeProvider := func() time.Time { return epochRetrieverFunc().StartTime() }
	confirmedRetrieverFunc := func() epoch.Index { return 0 }

	weightProvider = NewCManaWeightProvider(manaRetrieverMock, timeProvider, confirmedRetrieverFunc)

	s.Tangle = NewTestTangle(append([]Option{
		ApprovalWeights(weightProvider),
	}, options...)...)
	s.Tangle.Booker.MarkersManager.Options.MaxPastMarkerDistance = 2
	s.Tangle.Setup()

	s.testEventMock = NewEventMock(t, s.Tangle.ApprovalWeightManager)
	s.TestFramework = NewBlockTestFramework(s.Tangle, WithGenesisOutput("A", 500))
	s.Steps = []TestStep{
		// ISSUE Block0
		func(t *testing.T, testFramework *BlockTestFramework, testEventMock *EventMock, nodes NodeIdentities) {
			// Make all nodes active
			for node := range nodes {
				weightProvider.Update(epochRetrieverFunc(), nodes[node].ID())
			}
			testFramework.CreateBlock("Block0", WithStrongParents("Genesis"), WithIssuer(nodes["A"].PublicKey()))

			testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(0, 1), 0.30)

			IssueAndValidateBlockApproval(t, "Block0", testEventMock, testFramework, map[string]float64{}, map[markers.Marker]float64{
				markers.NewMarker(0, 1): 0.30,
			})
		},
		// ISSUE Block1
		func(t *testing.T, testFramework *BlockTestFramework, testEventMock *EventMock, nodes NodeIdentities) {
			testFramework.CreateBlock("Block1", WithStrongParents("Genesis"), WithIssuer(nodes["A"].PublicKey()))

			IssueAndValidateBlockApproval(t, "Block1", testEventMock, testFramework, map[string]float64{}, map[markers.Marker]float64{
				markers.NewMarker(0, 1): 0.30,
			})
		},
		// ISSUE Block2
		func(t *testing.T, testFramework *BlockTestFramework, testEventMock *EventMock, nodes NodeIdentities) {
			testFramework.CreateBlock("Block2", WithStrongParents("Block1"), WithIssuer(nodes["B"].PublicKey()))

			testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(1, 1), 0.15)

			IssueAndValidateBlockApproval(t, "Block2", testEventMock, testFramework, map[string]float64{}, map[markers.Marker]float64{
				markers.NewMarker(0, 1): 0.30,
				markers.NewMarker(1, 1): 0.15,
			})
		},
		// ISSUE Block3
		func(t *testing.T, testFramework *BlockTestFramework, testEventMock *EventMock, nodes NodeIdentities) {
			testFramework.CreateBlock("Block3", WithStrongParents("Block2"), WithIssuer(nodes["B"].PublicKey()))

			testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(1, 2), 0.15)

			IssueAndValidateBlockApproval(t, "Block3", testEventMock, testFramework, map[string]float64{}, map[markers.Marker]float64{
				markers.NewMarker(0, 1): 0.30,
				markers.NewMarker(1, 1): 0.15,
				markers.NewMarker(1, 2): 0.15,
			})
		},
		// ISSUE Block4
		func(t *testing.T, testFramework *BlockTestFramework, testEventMock *EventMock, nodes NodeIdentities) {
			testFramework.CreateBlock("Block4", WithWeakParents("Block2"), WithStrongParents("Block3"), WithIssuer(nodes["D"].PublicKey()))

			testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(1, 1), 0.35)
			testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(1, 2), 0.35)
			testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(1, 3), 0.20)

			IssueAndValidateBlockApproval(t, "Block4", testEventMock, testFramework, map[string]float64{}, map[markers.Marker]float64{
				markers.NewMarker(0, 1): 0.30,
				markers.NewMarker(1, 1): 0.35,
				markers.NewMarker(1, 2): 0.35,
				markers.NewMarker(1, 3): 0.20,
			})
		},
	}
	return s
}

// IssueAndValidateBlockApproval issues the blk by the given alias and assets the expected weights.
//nolint:gomnd
func IssueAndValidateBlockApproval(t *testing.T, blockAlias string, eventMock *EventMock, testFramework *BlockTestFramework, expectedConflictWeights map[string]float64, expectedMarkerWeights map[markers.Marker]float64) {
	eventMock.Expect("BlockProcessed", testFramework.Block(blockAlias).ID())

	t.Logf("ISSUE:\tBlockID(%s)", blockAlias)
	testFramework.IssueBlocks(blockAlias).WaitUntilAllTasksProcessed()

	for conflictAlias, expectedWeight := range expectedConflictWeights {
		conflictID := testFramework.ConflictID(conflictAlias)
		actualWeight := testFramework.tangle.ApprovalWeightManager.WeightOfConflict(conflictID)
		if expectedWeight != actualWeight {
			require.True(t, math.Abs(actualWeight-expectedWeight) < 0.001, "weight of %s (%0.2f) not equal to expected value %0.2f", conflictID, actualWeight, expectedWeight)
		}
	}

	for marker, expectedWeight := range expectedMarkerWeights {
		actualWeight := testFramework.tangle.ApprovalWeightManager.WeightOfMarker(marker, testFramework.Block(blockAlias).IssuingTime())
		if expectedWeight != actualWeight {
			require.True(t, math.Abs(actualWeight-expectedWeight) < 0.001, "weight of %s (%0.2f) not equal to expected value %0.2f", marker.String(), actualWeight, expectedWeight)
		}
	}

	eventMock.AssertExpectations(t)
}
