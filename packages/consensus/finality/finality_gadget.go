package finality

import (
	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/datastructure/walker"
	"github.com/iotaledger/hive.go/types"

	"github.com/iotaledger/goshimmer/packages/consensus/gof"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/markers"
	"github.com/iotaledger/goshimmer/packages/tangle"
)

type Gadget interface {
	HandleMarker(marker *markers.Marker, aw float64) (err error)
	HandleBranch(branchID ledgerstate.BranchID, aw float64) (err error)
	IsMarkerConfirmed(marker *markers.Marker) (confirmed bool)
	tangle.ConfirmationOracle
}

type MessageThresholdTranslation func(aw float64) gof.GradeOfFinality
type BranchThresholdTranslation func(branchID ledgerstate.BranchID, aw float64) gof.GradeOfFinality

// region SimpleFinalityGadget /////////////////////////////////////////////////////////////////////////////////////////
var (
	// DefaultBranchGoFTranslation is the default function to translate the approval weight to gof.GradeOfFinality of a branch.
	DefaultBranchGoFTranslation BranchThresholdTranslation = func(branchID ledgerstate.BranchID, aw float64) gof.GradeOfFinality {
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
	}
	// DefaultMessageGoFTranslation is the default function to translate the approval weight to gof.GradeOfFinality of a message.
	DefaultMessageGoFTranslation MessageThresholdTranslation = func(aw float64) gof.GradeOfFinality {
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
	}

	// ErrUnsupportedBranchType is returned when an operation is tried on an unsupported branch type.
	ErrUnsupportedBranchType = errors.New("unsupported branch type")
)

type SimpleFinalityGadget struct {
	tangle *tangle.Tangle

	branchGoF             BranchThresholdTranslation
	branchGoFReachedLevel gof.GradeOfFinality

	messageGoF             MessageThresholdTranslation
	messageGoFReachedLevel gof.GradeOfFinality

	events *tangle.ConfirmationEvents
}

// NewSimpleFinalityGadget creates a new SimpleFinalityGadget.
func NewSimpleFinalityGadget(t *tangle.Tangle) *SimpleFinalityGadget {
	return &SimpleFinalityGadget{
		tangle:                 t,
		branchGoFReachedLevel:  gof.High,
		messageGoFReachedLevel: gof.High,
		events:                 &tangle.ConfirmationEvents{
			//MessageGoFReached:     events.NewEvent(),
			//TransactionGoFReached: events.NewEvent(),
		},
		branchGoF:  DefaultBranchGoFTranslation,
		messageGoF: DefaultMessageGoFTranslation,
	}
}

// Events returns the events this gadget exposes.
func (s *SimpleFinalityGadget) Events() *tangle.ConfirmationEvents {
	return s.events
}

func (s *SimpleFinalityGadget) IsMarkerConfirmed(marker *markers.Marker) (confirmed bool) {
	messageID := s.tangle.Booker.MarkersManager.MessageID(marker)
	if messageID == tangle.EmptyMessageID {
		return false
	}

	s.tangle.Storage.MessageMetadata(messageID).Consume(func(messageMetadata *tangle.MessageMetadata) {
		if messageMetadata.GradeOfFinality() >= s.messageGoFReachedLevel {
			confirmed = true
		}
	})
	return
}

// IsMessageConfirmed returns whether the given message is confirmed.
func (s *SimpleFinalityGadget) IsMessageConfirmed(msgId tangle.MessageID) (confirmed bool) {
	s.tangle.Storage.MessageMetadata(msgId).Consume(func(messageMetadata *tangle.MessageMetadata) {
		if messageMetadata.GradeOfFinality() >= s.messageGoFReachedLevel {
			confirmed = true
		}
	})
	return
}

// IsBranchConfirmed returns whether the given branch is confirmed.
func (s *SimpleFinalityGadget) IsBranchConfirmed(branchId ledgerstate.BranchID) (confirmed bool) {
	s.tangle.LedgerState.BranchDAG.Branch(branchId).Consume(func(branch ledgerstate.Branch) {
		if branch.GradeOfFinality() >= s.messageGoFReachedLevel {
			confirmed = true
		}
	})
	return
}

func (s *SimpleFinalityGadget) HandleMarker(marker *markers.Marker, aw float64) (err error) {
	gradeOfFinality := s.messageGoF(aw)
	if gradeOfFinality == gof.None {
		return
	}

	// get message ID of marker
	messageID := s.tangle.Booker.MarkersManager.MessageID(marker)

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
	newGradeOfFinality := s.branchGoF(branchID, aw)
	var isAggrBranch bool
	var gradeOfFinalityChanged bool
	s.tangle.LedgerState.BranchDAG.Branch(branchID).Consume(func(branch ledgerstate.Branch) {
		if branch.Type() == ledgerstate.AggregatedBranchType {
			isAggrBranch = true
			return
		}
		gradeOfFinalityChanged = branch.SetGradeOfFinality(newGradeOfFinality)
	})

	// we don't do anything if the branch is an aggr. one,
	// as this function should be called with the parent branches at some point.
	if isAggrBranch {
		return errors.Errorf("%w: can not translate approval weight of an aggregated branch %s", ErrUnsupportedBranchType, branchID)
	}

	if !gradeOfFinalityChanged {
		return
	}

	// update GoF of txs within the same branch
	txGoFPropWalker := walker.New()
	txGoFPropWalker.Push(branchID.TransactionID())
	for txGoFPropWalker.HasNext() {
		s.forwardPropagateBranchGoFToTxs(txGoFPropWalker.Next().(ledgerstate.TransactionID), branchID, newGradeOfFinality, txGoFPropWalker)
	}

	return
}

func (s *SimpleFinalityGadget) forwardPropagateBranchGoFToTxs(candidateTxID ledgerstate.TransactionID, candidateBranchID ledgerstate.BranchID, newGradeOfFinality gof.GradeOfFinality, txGoFPropWalker *walker.Walker) bool {
	return s.tangle.LedgerState.UTXODAG.CachedTransactionMetadata(candidateTxID).Consume(func(transactionMetadata *ledgerstate.TransactionMetadata) {
		// we stop if we walk outside our branch
		if transactionMetadata.BranchID() != candidateBranchID {
			return
		}

		transactionMetadata.SetGradeOfFinality(newGradeOfFinality)
		s.tangle.LedgerState.UTXODAG.CachedTransaction(candidateBranchID.TransactionID()).Consume(func(transaction *ledgerstate.Transaction) {
			// we use a set of consumer txs as our candidate tx can consume multiple outputs from the same txs,
			// but we want to add such tx only once to the walker
			consumerTxs := make(map[ledgerstate.TransactionID]types.Empty)

			// adjust output GoF and add its consumer txs to the walker
			for _, output := range transaction.Essence().Outputs() {
				s.tangle.LedgerState.UTXODAG.CachedOutputMetadata(output.ID()).Consume(func(outputMetadata *ledgerstate.OutputMetadata) {
					outputMetadata.SetGradeOfFinality(newGradeOfFinality)
					s.tangle.LedgerState.Consumers(output.ID()).Consume(func(consumer *ledgerstate.Consumer) {
						if _, has := consumerTxs[consumer.TransactionID()]; !has {
							consumerTxs[consumer.TransactionID()] = types.Empty{}
							txGoFPropWalker.Push(consumer.TransactionID())
						}
					})
				})
			}
		})
	})
}

func (s *SimpleFinalityGadget) setMessageGoF(messageMetadata *tangle.MessageMetadata, gradeOfFinality gof.GradeOfFinality) (modified bool) {
	// abort if message has GoF already set
	if modified = messageMetadata.SetGradeOfFinality(gradeOfFinality); !modified {
		return
	}

	// set GoF of payload (applicable only to transactions)
	s.setPayloadGoF(messageMetadata.ID(), gradeOfFinality)

	if gradeOfFinality >= s.messageGoFReachedLevel {
		s.Events().MessageConfirmed.Trigger(messageMetadata.ID())
	}

	return modified
}

func (s *SimpleFinalityGadget) setPayloadGoF(messageID tangle.MessageID, gradeOfFinality gof.GradeOfFinality) {
	s.tangle.Utils.ComputeIfTransaction(messageID, func(transactionID ledgerstate.TransactionID) {
		s.tangle.LedgerState.TransactionMetadata(transactionID).Consume(func(transactionMetadata *ledgerstate.TransactionMetadata) {
			// if the transaction is part of a conflicting branch then we need to evaluate based on branch AW
			if s.tangle.LedgerState.TransactionConflicting(transactionID) {
				branchID := ledgerstate.NewBranchID(transactionID)
				gradeOfFinality = s.branchGoF(branchID, s.tangle.ApprovalWeightManager.WeightOfBranch(branchID))
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
				s.Events().TransactionConfirmed.Trigger(transactionID)
			}
		})
	})
}

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
