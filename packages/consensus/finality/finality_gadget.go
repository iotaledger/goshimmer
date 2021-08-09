package finality

import (
	"github.com/iotaledger/hive.go/datastructure/walker"
	"github.com/iotaledger/hive.go/identity"

	"github.com/iotaledger/goshimmer/packages/consensus/gof"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/markers"
	"github.com/iotaledger/goshimmer/packages/tangle"
)

type Gadget interface {
	HandleMarker(marker markers.Marker, aw float64) (err error)
	HandleBranch(branchID ledgerstate.BranchID, aw float64) (err error)
}

type MarkerThresholdTranslation func(aw float64) gof.GradeOfFinality
type BranchThresholdTranslation func(branchID ledgerstate.BranchID, aw float64) gof.GradeOfFinality

// region SimpleFinalityGadget /////////////////////////////////////////////////////////////////////////////////////////

type SimpleFinalityGadget struct {
	tangle    *tangle.Tangle
	branchGoF BranchThresholdTranslation
	markerGoF MarkerThresholdTranslation
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
		markerGoF: func(aw float64) gof.GradeOfFinality {
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
	}
}

func (s *SimpleFinalityGadget) HandleMarker(marker markers.Marker, aw float64) (err error) {
	gradeOfFinality := s.markerGoF(aw)
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

	propagateGoF := func(message *tangle.Message, messageMetadata *tangle.MessageMetadata, finalizedWalker *walker.Walker) {
		// stop walking to past cone if reach a marker
		if messageMetadata.StructureDetails().IsPastMarker && messageMetadata.IsFinalized() {
			return
		}

		// abort if the message is already finalized
		if !s.setMessageFinalized(messageMetadata) {
			return
		}

		// TODO: revisit weak parents
		// mark weak parents as finalized but not propagate finalized flag to its past cone
		//message.ForEachParentByType(tangle.WeakParentType, func(parentID tangle.MessageID) {
		//	Tangle().Storage.MessageMetadata(parentID).Consume(func(messageMetadata *tangle.MessageMetadata) {
		//		setMessageFinalized(messageMetadata)
		//	})
		//})

		// propagate finalized to strong parents
		message.ForEachParentByType(tangle.StrongParentType, func(parentID tangle.MessageID) {
			finalizedWalker.Push(parentID)
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

func (s *SimpleFinalityGadget) setMessageFinalized(messageMetadata *tangle.MessageMetadata) (modified bool) {
	// abort if the message is already finalized
	if modified = messageMetadata.SetFinalized(true); !modified {
		return
	}

	// TODO: is this still necessary?
	// set issuer of message as active node
	s.tangle.Storage.Message(messageMetadata.ID()).Consume(func(message *tangle.Message) {
		s.tangle.WeightProvider.Update(message.IssuingTime(), identity.NewID(message.IssuerPublicKey()))
	})

	//s.setPayloadFinalized(messageMetadata.ID())

	// TODO: introduce message finalized event? could be used for TangleTime
	//Tangle().ApprovalWeightManager.Events.MessageFinalized.Trigger(messageMetadata.ID())

	return modified
}

func (s *SimpleFinalityGadget) setPayloadFinalized(messageID tangle.MessageID) {
	s.tangle.Utils.ComputeIfTransaction(messageID, func(transactionID ledgerstate.TransactionID) {
		// TODO: evaluate like switch, intersect with branch supporters
		//if err := s.tangle.LedgerState.UTXODAG.SetTransactionConfirmed(transactionID); err != nil {
		//	panic(err)
		//}
	})
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

type Option func(s *SimpleFinalityGadget)

func WithMarkerThresholdTranslation(t MarkerThresholdTranslation) Option {
	return func(s *SimpleFinalityGadget) {
		s.markerGoF = t
	}
}

func WithBranchThresholdTranslation(t BranchThresholdTranslation) Option {
	return func(s *SimpleFinalityGadget) {
		s.branchGoF = t
	}
}
