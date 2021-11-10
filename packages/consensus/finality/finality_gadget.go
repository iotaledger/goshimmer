package finality

import (
	"fmt"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/datastructure/walker"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/types"

	"github.com/iotaledger/goshimmer/packages/consensus/gof"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/markers"
	"github.com/iotaledger/goshimmer/packages/tangle"
)

// Gadget is an interface that describes the finality gadget.
type Gadget interface {
	HandleMarker(marker *markers.Marker, aw float64) (err error)
	HandleBranch(branchID ledgerstate.BranchID, aw float64) (err error)
	tangle.ConfirmationOracle
}

// MessageThresholdTranslation is a function which translates approval weight to a gof.GradeOfFinality.
type MessageThresholdTranslation func(aw float64) gof.GradeOfFinality

// BranchThresholdTranslation is a function which translates approval weight to a gof.GradeOfFinality.
type BranchThresholdTranslation func(branchID ledgerstate.BranchID, aw float64) gof.GradeOfFinality

const (
	lowLowerBound    = 0.25
	mediumLowerBound = 0.45
	highLowerBound   = 0.67
)

var (
	// DefaultBranchGoFTranslation is the default function to translate the approval weight to gof.GradeOfFinality of a branch.
	DefaultBranchGoFTranslation BranchThresholdTranslation = func(branchID ledgerstate.BranchID, aw float64) gof.GradeOfFinality {
		switch {
		case aw >= lowLowerBound && aw < mediumLowerBound:
			return gof.Low
		case aw >= mediumLowerBound && aw < highLowerBound:
			return gof.Medium
		case aw >= highLowerBound:
			return gof.High
		default:
			return gof.None
		}
	}

	// DefaultMessageGoFTranslation is the default function to translate the approval weight to gof.GradeOfFinality of a message.
	DefaultMessageGoFTranslation MessageThresholdTranslation = func(aw float64) gof.GradeOfFinality {
		switch {
		case aw >= lowLowerBound && aw < mediumLowerBound:
			return gof.Low
		case aw >= mediumLowerBound && aw < highLowerBound:
			return gof.Medium
		case aw >= highLowerBound:
			return gof.High
		default:
			return gof.None
		}
	}

	// ErrUnsupportedBranchType is returned when an operation is tried on an unsupported branch type.
	ErrUnsupportedBranchType = errors.New("unsupported branch type")
)

// Option is a function setting an option on an Options struct.
type Option func(*Options)

// Options defines the options for a SimpleFinalityGadget.
type Options struct {
	BranchTransFunc        BranchThresholdTranslation
	MessageTransFunc       MessageThresholdTranslation
	BranchGoFReachedLevel  gof.GradeOfFinality
	MessageGoFReachedLevel gof.GradeOfFinality
}

var defaultOpts = []Option{
	WithBranchThresholdTranslation(DefaultBranchGoFTranslation),
	WithMessageThresholdTranslation(DefaultMessageGoFTranslation),
	WithBranchGoFReachedLevel(gof.High),
	WithMessageGoFReachedLevel(gof.High),
}

// WithMessageThresholdTranslation returns an Option setting the MessageThresholdTranslation.
func WithMessageThresholdTranslation(f MessageThresholdTranslation) Option {
	return func(opts *Options) {
		opts.MessageTransFunc = f
	}
}

// WithBranchThresholdTranslation returns an Option setting the BranchThresholdTranslation.
func WithBranchThresholdTranslation(f BranchThresholdTranslation) Option {
	return func(opts *Options) {
		opts.BranchTransFunc = f
	}
}

// WithBranchGoFReachedLevel returns an Option setting the branch reached grade of finality level.
func WithBranchGoFReachedLevel(branchGradeOfFinality gof.GradeOfFinality) Option {
	return func(opts *Options) {
		opts.BranchGoFReachedLevel = branchGradeOfFinality
	}
}

// WithMessageGoFReachedLevel returns an Option setting the message reached grade of finality level.
func WithMessageGoFReachedLevel(msgGradeOfFinality gof.GradeOfFinality) Option {
	return func(opts *Options) {
		opts.MessageGoFReachedLevel = msgGradeOfFinality
	}
}

// SimpleFinalityGadget is a Gadget which simply translates approval weight down to gof.GradeOfFinality
// and then applies it to messages, branches, transactions and outputs.
type SimpleFinalityGadget struct {
	tangle *tangle.Tangle
	opts   *Options
	events *tangle.ConfirmationEvents
}

// NewSimpleFinalityGadget creates a new SimpleFinalityGadget.
func NewSimpleFinalityGadget(t *tangle.Tangle, opts ...Option) *SimpleFinalityGadget {
	sfg := &SimpleFinalityGadget{
		tangle: t,
		opts:   &Options{},
		events: &tangle.ConfirmationEvents{
			MessageConfirmed:     events.NewEvent(tangle.MessageIDCaller),
			TransactionConfirmed: events.NewEvent(ledgerstate.TransactionIDEventHandler),
			BranchConfirmed:      events.NewEvent(ledgerstate.BranchIDEventHandler),
		},
	}

	for _, defOpt := range defaultOpts {
		defOpt(sfg.opts)
	}
	for _, opt := range opts {
		opt(sfg.opts)
	}

	return sfg
}

// Events returns the events this gadget exposes.
func (s *SimpleFinalityGadget) Events() *tangle.ConfirmationEvents {
	return s.events
}

// IsMarkerConfirmed returns whether the given marker is confirmed.
func (s *SimpleFinalityGadget) IsMarkerConfirmed(marker *markers.Marker) (confirmed bool) {
	messageID := s.tangle.Booker.MarkersManager.MessageID(marker)
	if messageID == tangle.EmptyMessageID {
		return false
	}

	s.tangle.Storage.MessageMetadata(messageID).Consume(func(messageMetadata *tangle.MessageMetadata) {
		if messageMetadata.GradeOfFinality() >= s.opts.MessageGoFReachedLevel {
			confirmed = true
		}
	})
	return
}

// IsMessageConfirmed returns whether the given message is confirmed.
func (s *SimpleFinalityGadget) IsMessageConfirmed(msgID tangle.MessageID) (confirmed bool) {
	s.tangle.Storage.MessageMetadata(msgID).Consume(func(messageMetadata *tangle.MessageMetadata) {
		if messageMetadata.GradeOfFinality() >= s.opts.MessageGoFReachedLevel {
			confirmed = true
		}
	})
	return
}

// IsBranchConfirmed returns whether the given branch is confirmed.
func (s *SimpleFinalityGadget) IsBranchConfirmed(branchID ledgerstate.BranchID) (confirmed bool) {
	// TODO: HANDLE ERRORS INSTEAD?
	branchGoF, _ := s.tangle.LedgerState.UTXODAG.BranchGradeOfFinality(branchID)

	return branchGoF >= s.opts.BranchGoFReachedLevel
}

// IsTransactionConfirmed returns whether the given transaction is confirmed.
func (s *SimpleFinalityGadget) IsTransactionConfirmed(transactionID ledgerstate.TransactionID) (confirmed bool) {
	s.tangle.LedgerState.TransactionMetadata(transactionID).Consume(func(transactionMetadata *ledgerstate.TransactionMetadata) {
		if transactionMetadata.GradeOfFinality() >= s.opts.MessageGoFReachedLevel {
			confirmed = true
		}
	})
	return
}

// IsOutputConfirmed returns whether the given output is confirmed.
func (s *SimpleFinalityGadget) IsOutputConfirmed(outputID ledgerstate.OutputID) (confirmed bool) {
	s.tangle.LedgerState.CachedOutputMetadata(outputID).Consume(func(outputMetadata *ledgerstate.OutputMetadata) {
		if outputMetadata.GradeOfFinality() >= s.opts.BranchGoFReachedLevel {
			confirmed = true
		}
	})
	return
}

// HandleMarker receives a marker and its current approval weight. It propagates the GoF according to AW to its past cone.
func (s *SimpleFinalityGadget) HandleMarker(marker *markers.Marker, aw float64) (err error) {
	gradeOfFinality := s.opts.MessageTransFunc(aw)
	if gradeOfFinality == gof.None {
		return nil
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
		// stop walking to past cone if reach a message with a higher or equal grade of finality
		if messageMetadata.GradeOfFinality() >= gradeOfFinality {
			return
		}

		s.setMessageGoF(messageMetadata, gradeOfFinality)

		// TODO: revisit weak parents
		// mark weak parents as finalized but not propagate finalized flag to its past cone
		//message.ForEachParentByType(tangle.WeakParentType, func(parentID tangle.MessageID) {
		//	Tangle().Storage.MessageMetadata(parentID).Consume(func(messageMetadata *tangle.MessageMetadata) {
		//		setMessageGoF(messageMetadata)
		//	})
		//})

		// propagate GoF to strong and like parents
		message.ForEachParentByType(tangle.StrongParentType, func(parentID tangle.MessageID) {
			w.Push(parentID)
		})
		message.ForEachParentByType(tangle.LikeParentType, func(parentID tangle.MessageID) {
			w.Push(parentID)
		})
	}

	s.tangle.Utils.WalkMessageAndMetadata(propagateGoF, tangle.MessageIDs{messageID}, false)

	return err
}

// HandleBranch receives a branchID and its approval weight. It propagates the GoF according to AW to transactions
// in the branch (UTXO future cone) and their outputs.
func (s *SimpleFinalityGadget) HandleBranch(branchID ledgerstate.BranchID, aw float64) (err error) {
	newGradeOfFinality := s.opts.BranchTransFunc(branchID, aw)

	// update GoF of txs within the same branch
	txGoFPropWalker := walker.New()
	s.tangle.LedgerState.UTXODAG.CachedTransactionMetadata(branchID.TransactionID()).Consume(func(transactionMetadata *ledgerstate.TransactionMetadata) {
		s.updateTransactionGoF(transactionMetadata, newGradeOfFinality, txGoFPropWalker)
	})
	for txGoFPropWalker.HasNext() {
		s.forwardPropagateBranchGoFToTxs(txGoFPropWalker.Next().(ledgerstate.TransactionID), branchID, newGradeOfFinality, txGoFPropWalker)
	}

	if newGradeOfFinality >= s.opts.BranchGoFReachedLevel {
		s.events.BranchConfirmed.Trigger(branchID)
	}

	return err
}

func (s *SimpleFinalityGadget) forwardPropagateBranchGoFToTxs(candidateTxID ledgerstate.TransactionID, candidateBranchID ledgerstate.BranchID, newGradeOfFinality gof.GradeOfFinality, txGoFPropWalker *walker.Walker) bool {
	return s.tangle.LedgerState.UTXODAG.CachedTransactionMetadata(candidateTxID).Consume(func(transactionMetadata *ledgerstate.TransactionMetadata) {
		// we stop if we walk outside our branch
		if transactionMetadata.BranchID() != candidateBranchID {
			return
		}

		var maxAttachmentGoF gof.GradeOfFinality
		s.tangle.Storage.Attachments(transactionMetadata.ID()).Consume(func(attachment *tangle.Attachment) {
			s.tangle.Storage.MessageMetadata(attachment.MessageID()).Consume(func(messageMetadata *tangle.MessageMetadata) {
				if maxAttachmentGoF < messageMetadata.GradeOfFinality() {
					maxAttachmentGoF = messageMetadata.GradeOfFinality()
				}
			})
		})

		// only adjust tx GoF if attachments have at least GoF derived from UTXO parents
		if maxAttachmentGoF < newGradeOfFinality {
			return
		}

		s.updateTransactionGoF(transactionMetadata, newGradeOfFinality, txGoFPropWalker)
	})
}

func (s *SimpleFinalityGadget) updateTransactionGoF(transactionMetadata *ledgerstate.TransactionMetadata, newGradeOfFinality gof.GradeOfFinality, txGoFPropWalker *walker.Walker) {
	// abort if the grade of finality did not change
	if !transactionMetadata.SetGradeOfFinality(newGradeOfFinality) {
		return
	}

	s.tangle.LedgerState.UTXODAG.CachedTransaction(transactionMetadata.ID()).Consume(func(transaction *ledgerstate.Transaction) {
		// we use a set of consumer txs as our candidate tx can consume multiple outputs from the same txs,
		// but we want to add such tx only once to the walker
		consumerTxs := make(ledgerstate.TransactionIDs)

		// adjust output GoF and add its consumer txs to the walker
		for _, output := range transaction.Essence().Outputs() {
			s.adjustOutputGoF(output, newGradeOfFinality, consumerTxs, txGoFPropWalker)
		}
	})
	if transactionMetadata.GradeOfFinality() >= s.opts.BranchGoFReachedLevel {
		s.events.TransactionConfirmed.Trigger(transactionMetadata.ID())
	}
}

func (s *SimpleFinalityGadget) adjustOutputGoF(output ledgerstate.Output, newGradeOfFinality gof.GradeOfFinality, consumerTxs ledgerstate.TransactionIDs, txGoFPropWalker *walker.Walker) bool {
	return s.tangle.LedgerState.UTXODAG.CachedOutputMetadata(output.ID()).Consume(func(outputMetadata *ledgerstate.OutputMetadata) {
		outputMetadata.SetGradeOfFinality(newGradeOfFinality)
		s.tangle.LedgerState.Consumers(output.ID()).Consume(func(consumer *ledgerstate.Consumer) {
			if _, has := consumerTxs[consumer.TransactionID()]; !has {
				consumerTxs[consumer.TransactionID()] = types.Empty{}
				txGoFPropWalker.Push(consumer.TransactionID())
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

	if gradeOfFinality >= s.opts.MessageGoFReachedLevel {
		s.Events().MessageConfirmed.Trigger(messageMetadata.ID())
	}

	return modified
}

func (s *SimpleFinalityGadget) setPayloadGoF(messageID tangle.MessageID, gradeOfFinality gof.GradeOfFinality) {
	s.tangle.Utils.ComputeIfTransaction(messageID, func(transactionID ledgerstate.TransactionID) {
		s.tangle.LedgerState.TransactionMetadata(transactionID).Consume(func(transactionMetadata *ledgerstate.TransactionMetadata) {
			// A transaction can't have a higher GoF than its branch, thus we need to evaluate based on min(branchGoF,max(messageGoF,transactionGoF)).
			// This also works for transactions in MasterBranch since it has gof.High and we then use max(messageGoF,transactionGoF).
			// max(messageGoF,transactionGoF) gets the max GoF of any possible reattachment (which has set the transaction's GoF before).
			transactionGoF := transactionMetadata.GradeOfFinality()
			if transactionGoF > gradeOfFinality {
				gradeOfFinality = transactionMetadata.GradeOfFinality()
			}

			branchGoF, err := s.tangle.LedgerState.UTXODAG.BranchGradeOfFinality(transactionMetadata.BranchID())
			if err != nil {
				// TODO: properly handle error
				panic(err)
			}
			// This is an invalid invariant and should never happen.
			if transactionGoF > branchGoF {
				panic(fmt.Sprintf("%s GoF (%s) is bigger than its branch %s GoF (%s)", transactionID, transactionGoF, transactionMetadata.BranchID(), branchGoF))
			}

			if branchGoF < gradeOfFinality {
				gradeOfFinality = branchGoF
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
			})

			if gradeOfFinality >= s.opts.BranchGoFReachedLevel {
				s.Events().TransactionConfirmed.Trigger(transactionID)
			}
		})
	})
}
