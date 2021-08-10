package finality

import (
	"github.com/iotaledger/hive.go/datastructure/walker"
	"github.com/iotaledger/hive.go/events"

	"github.com/iotaledger/goshimmer/packages/consensus/gof"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/markers"
	"github.com/iotaledger/goshimmer/packages/tangle"
)

type Gadget interface {
	HandleMarker(marker markers.Marker, aw float64) (err error)
	HandleBranch(branchID ledgerstate.BranchID, aw float64) (err error)
	Events() Events
}

type MessageThresholdTranslation func(aw float64) gof.GradeOfFinality
type BranchThresholdTranslation func(branchID ledgerstate.BranchID, aw float64) gof.GradeOfFinality

type Events struct {
	MessageGoFReached     events.Event
	TransactionGoFReached events.Event
}

// region SimpleFinalityGadget /////////////////////////////////////////////////////////////////////////////////////////

type SimpleFinalityGadget struct {
	tangle *tangle.Tangle

	branchGoF             BranchThresholdTranslation
	branchGoFReachedLevel gof.GradeOfFinality

	messageGoF             MessageThresholdTranslation
	messageGoFReachedLevel gof.GradeOfFinality

	events *Events
}

func NewSimpleFinalityGadget(tangle *tangle.Tangle) *SimpleFinalityGadget {
	return &SimpleFinalityGadget{
		tangle: tangle,
		branchGoF: func(branchID ledgerstate.BranchID, aw float64) gof.GradeOfFinality {
			switch {
			case aw >= 0.2 && aw < 0.3:
				return gof.Low
			case aw >= 0.3 && aw < 0.6:
				return gof.Middle
			case aw >= 0.6:
				return gof.High
			default:
				return gof.None
			}
		},
		branchGoFReachedLevel: gof.High,
		messageGoF: func(aw float64) gof.GradeOfFinality {
			switch {
			case aw >= 0.2 && aw < 0.3:
				return gof.Low
			case aw >= 0.3 && aw < 0.6:
				return gof.Middle
			case aw >= 0.6:
				return gof.High
			default:
				return gof.None
			}
		},
		messageGoFReachedLevel: gof.High,

		events: &Events{
			//MessageGoFReached:     events.NewEvent(),
			//TransactionGoFReached: events.NewEvent(),
		},
	}
}

func (s *SimpleFinalityGadget) Events() *Events {
	return s.events
}

func (s *SimpleFinalityGadget) HandleMarker(marker markers.Marker, aw float64) (err error) {
	gradeOfFinality := s.messageGoF(aw)
	if gradeOfFinality == gof.None {
		return
	}

	// get message ID of marker
	messageID := s.tangle.Booker.MarkersManager.MessageID(&marker)

	// check that we're updating the GoF
	var gofIncreased bool
	s.tangle.Storage.MessageMetadata(messageID).Consume(func(messageMetadata *tangle.MessageMetadata) {
		if gradeOfFinality > messageMetadata.GradeOfFinality() {
			gofIncreased = true
		}
	})
	if !gofIncreased {
		return
	}

	propagateGoF := func(message *tangle.Message, messageMetadata *tangle.MessageMetadata, w *walker.Walker) {
		// stop walking to past cone if reach a marker
		if messageMetadata.StructureDetails().IsPastMarker && messageMetadata.GradeOfFinality() == gradeOfFinality {
			return
		}

		// abort if message has GoF already set
		if !s.setMessageGoF(messageMetadata, gradeOfFinality) {
			return
		}

		// TODO: revisit weak parents
		// mark weak parents as finalized but not propagate finalized flag to its past cone
		//message.ForEachParentByType(tangle.WeakParentType, func(parentID tangle.MessageID) {
		//	Tangle().Storage.MessageMetadata(parentID).Consume(func(messageMetadata *tangle.MessageMetadata) {
		//		setMessageGoF(messageMetadata)
		//	})
		//})

		// propagate GoF to strong parents
		message.ForEachParentByType(tangle.StrongParentType, func(parentID tangle.MessageID) {
			w.Push(parentID)
		})
	}

	s.tangle.Utils.WalkMessageAndMetadata(propagateGoF, tangle.MessageIDs{messageID}, false)

	return
}

func (s *SimpleFinalityGadget) HandleBranch(branchID ledgerstate.BranchID, aw float64) (err error) {
	// TODO:
	//   1. determine GoF
	//   2. set GoF to branch
	//   3. propagate through branch DAG
	//   4. finalize corresponding transaction
	return
}

func (s *SimpleFinalityGadget) setMessageGoF(messageMetadata *tangle.MessageMetadata, gradeOfFinality gof.GradeOfFinality) (modified bool) {
	// abort if message has GoF already set
	if modified = messageMetadata.SetGradeOfFinality(gradeOfFinality); !modified {
		return
	}

	// set GoF of payload (applicable only to transactions)
	s.setPayloadGoF(messageMetadata.ID(), gradeOfFinality)

	if gradeOfFinality >= s.messageGoFReachedLevel {
		s.Events().MessageGoFReached.Trigger(messageMetadata.ID())
	}

	return modified
}

func (s *SimpleFinalityGadget) setPayloadGoF(messageID tangle.MessageID, gradeOfFinality gof.GradeOfFinality) {
	s.tangle.Utils.ComputeIfTransaction(messageID, func(transactionID ledgerstate.TransactionID) {
		s.tangle.LedgerState.TransactionMetadata(transactionID).Consume(func(transactionMetadata *ledgerstate.TransactionMetadata) {
			// TODO: check if conflicting, if yes then we need to evaluate based on branch
			if !s.tangle.LedgerState.TransactionConflicting(transactionID) {

			}

			// abort if transaction has GoF already set
			if !transactionMetadata.SetGradeOfFinality(gradeOfFinality) {
				return
			}

			// set GoF in outputs
			s.tangle.LedgerState.Transaction(transactionID).Consume(func(transaction *ledgerstate.Transaction) {
				for _, output := range transaction.Essence().Outputs() {
					s.tangle.LedgerState.CachedOutputMetadata(output.ID()).Consume(func(outputMetadata *ledgerstate.OutputMetadata) {
						outputMetadata.SetGradeOfFinality(gradeOfFinality)
					})
				}

				//for _, input := range transaction.Essence().Inputs() {
				//	referencedOutputID := input.(*UTXOInput).ReferencedOutputID()
				//	// TODO: do we still need this?
				//	u.CachedOutputMetadata(referencedOutputID).Consume(func(outputMetadata *OutputMetadata) {
				//		outputMetadata.SetConfirmedConsumer(*transaction.id)
				//	})
				//}
			})

			if gradeOfFinality >= s.branchGoFReachedLevel {
				s.Events().TransactionGoFReached.Trigger(transactionID)
			}
		})
	})
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

type Option func(s *SimpleFinalityGadget)

func WithMessageThresholdTranslation(t MessageThresholdTranslation) Option {
	return func(s *SimpleFinalityGadget) {
		s.messageGoF = t
	}
}

func WithBranchThresholdTranslation(t BranchThresholdTranslation) Option {
	return func(s *SimpleFinalityGadget) {
		s.branchGoF = t
	}
}
