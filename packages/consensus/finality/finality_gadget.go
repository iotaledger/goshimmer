package finality

import (
	"fmt"
	"sync"

	"github.com/iotaledger/hive.go/generics/set"
	"github.com/iotaledger/hive.go/generics/walker"
	"github.com/iotaledger/hive.go/types/confirmation"

	"github.com/iotaledger/goshimmer/packages/ledger"
	"github.com/iotaledger/goshimmer/packages/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/ledger/vm/devnetvm"
	"github.com/iotaledger/goshimmer/packages/markers"
	"github.com/iotaledger/goshimmer/packages/tangle"
)

// Gadget is an interface that describes the finality gadget.
type Gadget interface {
	HandleMarker(marker markers.Marker, aw float64) (err error)
	HandleBranch(branchID utxo.TransactionID, aw float64) (err error)
	tangle.ConfirmationOracle
}

// MessageThresholdTranslation is a function which translates approval weight to a confirmation.State.
type MessageThresholdTranslation func(aw float64) confirmation.State

// BranchThresholdTranslation is a function which translates approval weight to a confirmation.State.
type BranchThresholdTranslation func(branchID utxo.TransactionID, aw float64) confirmation.State

const (
	acceptanceThreshold = 0.67
)

var (
	// DefaultBranchGoFTranslation is the default function to translate the approval weight to confirmation.State of a branch.
	DefaultBranchGoFTranslation BranchThresholdTranslation = func(branchID utxo.TransactionID, aw float64) confirmation.State {
		if aw >= acceptanceThreshold {
			return confirmation.Accepted
		}

		return confirmation.Pending
	}

	// DefaultMessageGoFTranslation is the default function to translate the approval weight to confirmation.State of a message.
	DefaultMessageGoFTranslation MessageThresholdTranslation = func(aw float64) confirmation.State {
		if aw >= acceptanceThreshold {
			return confirmation.Accepted
		}

		return confirmation.Pending
	}
)

// Option is a function setting an option on an Options struct.
type Option func(*Options)

// Options defines the options for a SimpleFinalityGadget.
type Options struct {
	BranchTransFunc  BranchThresholdTranslation
	MessageTransFunc MessageThresholdTranslation
}

var defaultOpts = []Option{
	WithBranchThresholdTranslation(DefaultBranchGoFTranslation),
	WithMessageThresholdTranslation(DefaultMessageGoFTranslation),
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

// SimpleFinalityGadget is a Gadget which simply translates approval weight down to confirmation.State
// and then applies it to messages, branches, transactions and outputs.
type SimpleFinalityGadget struct {
	tangle                    *tangle.Tangle
	opts                      *Options
	lastConfirmedMarkers      map[markers.SequenceID]markers.Index
	lastConfirmedMarkersMutex sync.RWMutex
	events                    *tangle.ConfirmationEvents
}

// NewSimpleFinalityGadget creates a new SimpleFinalityGadget.
func NewSimpleFinalityGadget(t *tangle.Tangle, opts ...Option) *SimpleFinalityGadget {
	sfg := &SimpleFinalityGadget{
		tangle:               t,
		opts:                 &Options{},
		lastConfirmedMarkers: make(map[markers.SequenceID]markers.Index),
		events:               tangle.NewConfirmationEvents(),
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
func (s *SimpleFinalityGadget) IsMarkerConfirmed(marker markers.Marker) (confirmed bool) {
	messageID := s.tangle.Booker.MarkersManager.MessageID(marker)
	if messageID == tangle.EmptyMessageID {
		return false
	}

	s.tangle.Storage.MessageMetadata(messageID).Consume(func(messageMetadata *tangle.MessageMetadata) {
		if messageMetadata.ConfirmationState() >= confirmation.Accepted {
			confirmed = true
		}
	})
	return
}

// IsMessageConfirmed returns whether the given message is confirmed.
func (s *SimpleFinalityGadget) IsMessageConfirmed(msgID tangle.MessageID) (confirmed bool) {
	s.tangle.Storage.MessageMetadata(msgID).Consume(func(messageMetadata *tangle.MessageMetadata) {
		if messageMetadata.ConfirmationState() >= confirmation.Accepted {
			confirmed = true
		}
	})
	return
}

// FirstUnconfirmedMarkerIndex returns the first Index in the given Sequence that was not confirmed, yet.
func (s *SimpleFinalityGadget) FirstUnconfirmedMarkerIndex(sequenceID markers.SequenceID) (index markers.Index) {
	s.lastConfirmedMarkersMutex.Lock()
	defer s.lastConfirmedMarkersMutex.Unlock()

	// TODO: MAP GROWS INDEFINITELY
	index, exists := s.lastConfirmedMarkers[sequenceID]
	if exists {
		return index + 1
	}

	s.tangle.Booker.MarkersManager.Manager.Sequence(sequenceID).Consume(func(sequence *markers.Sequence) {
		index = sequence.LowestIndex()
	})

	if !s.tangle.ConfirmationOracle.IsMarkerConfirmed(markers.NewMarker(sequenceID, index)) {
		return index
	}

	// do-while loop
	s.lastConfirmedMarkers[sequenceID] = index
	index++
	for s.tangle.ConfirmationOracle.IsMarkerConfirmed(markers.NewMarker(sequenceID, index)) {
		s.lastConfirmedMarkers[sequenceID] = index
		index++
	}

	return index
}

// IsBranchConfirmed returns whether the given branch is confirmed.
func (s *SimpleFinalityGadget) IsBranchConfirmed(branchID utxo.TransactionID) (confirmed bool) {
	return s.tangle.Ledger.ConflictDAG.ConfirmationState(utxo.NewTransactionIDs(branchID)) >= confirmation.Accepted
}

// IsTransactionConfirmed returns whether the given transaction is confirmed.
func (s *SimpleFinalityGadget) IsTransactionConfirmed(transactionID utxo.TransactionID) (confirmed bool) {
	s.tangle.Ledger.Storage.CachedTransactionMetadata(transactionID).Consume(func(transactionMetadata *ledger.TransactionMetadata) {
		if transactionMetadata.ConfirmationState() >= confirmation.Accepted {
			confirmed = true
		}
	})
	return
}

// IsOutputConfirmed returns whether the given output is confirmed.
func (s *SimpleFinalityGadget) IsOutputConfirmed(outputID utxo.OutputID) (confirmed bool) {
	s.tangle.Ledger.Storage.CachedOutputMetadata(outputID).Consume(func(outputMetadata *ledger.OutputMetadata) {
		if outputMetadata.ConfirmationState() >= confirmation.Accepted {
			confirmed = true
		}
	})
	return
}

// HandleMarker receives a marker and its current approval weight. It propagates the ConfirmationState according to AW to its past cone.
func (s *SimpleFinalityGadget) HandleMarker(marker markers.Marker, aw float64) (err error) {
	confirmationState := s.opts.MessageTransFunc(aw)
	if confirmationState == confirmation.Pending {
		return nil
	}

	// get message ID of marker
	messageID := s.tangle.Booker.MarkersManager.MessageID(marker)
	s.tangle.Storage.MessageMetadata(messageID).Consume(func(messageMetadata *tangle.MessageMetadata) {
		if confirmationState <= messageMetadata.ConfirmationState() {
			return
		}

		if confirmationState >= confirmation.Accepted {
			s.setMarkerConfirmed(marker)
		}

		s.propagateGoFToMessagePastCone(messageID, confirmationState)
	})

	return err
}

// setMarkerConfirmed marks the current Marker as confirmed.
func (s *SimpleFinalityGadget) setMarkerConfirmed(marker markers.Marker) (updated bool) {
	s.lastConfirmedMarkersMutex.Lock()
	defer s.lastConfirmedMarkersMutex.Unlock()

	if s.lastConfirmedMarkers[marker.SequenceID()] > marker.Index() {
		return false
	}

	s.lastConfirmedMarkers[marker.SequenceID()] = marker.Index()

	return true
}

// propagateGoFToMessagePastCone propagates the given ConfirmationState to the past cone of the Message.
func (s *SimpleFinalityGadget) propagateGoFToMessagePastCone(messageID tangle.MessageID, confirmationState confirmation.State) {
	strongParentWalker := walker.New[tangle.MessageID](false).Push(messageID)
	weakParentsSet := set.New[tangle.MessageID]()

	for strongParentWalker.HasNext() {
		strongParentMessageID := strongParentWalker.Next()
		if strongParentMessageID == tangle.EmptyMessageID {
			continue
		}

		s.tangle.Storage.MessageMetadata(strongParentMessageID).Consume(func(messageMetadata *tangle.MessageMetadata) {
			if messageMetadata.ConfirmationState() >= confirmationState {
				return
			}

			s.tangle.Storage.Message(strongParentMessageID).Consume(func(message *tangle.Message) {
				if !s.setMessageGoF(message, messageMetadata, confirmationState) {
					return
				}

				message.ForEachParent(func(parent tangle.Parent) {
					if parent.Type == tangle.StrongParentType {
						strongParentWalker.Push(parent.ID)
						return
					}
					weakParentsSet.Add(parent.ID)
				})
			})
		})
	}

	weakParentsSet.ForEach(func(weakParent tangle.MessageID) {
		if strongParentWalker.Pushed(weakParent) {
			return
		}
		s.tangle.Storage.MessageMetadata(weakParent).Consume(func(messageMetadata *tangle.MessageMetadata) {
			if messageMetadata.ConfirmationState() >= confirmationState {
				return
			}

			s.tangle.Storage.Message(weakParent).Consume(func(message *tangle.Message) {
				s.setMessageGoF(message, messageMetadata, confirmationState)
			})
		})
	})
}

// HandleBranch receives a branchID and its approval weight. It propagates the ConfirmationState according to AW to transactions
// in the branch (UTXO future cone) and their outputs.
func (s *SimpleFinalityGadget) HandleBranch(branchID utxo.TransactionID, aw float64) (err error) {
	fmt.Println("HandleBranch", branchID, aw)
	if s.opts.BranchTransFunc(branchID, aw) >= confirmation.Accepted {
		fmt.Println("HandleBranch", branchID, aw, "confirmed", confirmation.Accepted)
		s.tangle.Ledger.ConflictDAG.SetBranchAccepted(branchID)
	}

	return nil
}

func (s *SimpleFinalityGadget) setMessageGoF(message *tangle.Message, messageMetadata *tangle.MessageMetadata, confirmationState confirmation.State) (modified bool) {
	// abort if message has ConfirmationState already set
	if modified = messageMetadata.SetConfirmationState(confirmationState); !modified {
		return
	}

	if confirmationState >= confirmation.Accepted {
		s.Events().MessageAccepted.Trigger(&tangle.MessageAcceptedEvent{
			Message: message,
		})

		// set ConfirmationState of payload (applicable only to transactions)
		if tx, ok := message.Payload().(*devnetvm.Transaction); ok {
			s.tangle.Ledger.SetTransactionInclusionTime(tx.ID(), message.IssuingTime())
		}
	}

	return modified
}

func (s *SimpleFinalityGadget) getTransactionBranchesGoF(transactionMetadata *ledger.TransactionMetadata) (lowestBranchConfirmationState confirmation.State) {
	lowestBranchConfirmationState = confirmation.Pending
	for it := transactionMetadata.BranchIDs().Iterator(); it.HasNext(); {
		branchConfirmationState, err := s.tangle.Ledger.Utils.BranchConfirmationState(it.Next())
		if err != nil {
			// TODO: properly handle error
			panic(err)
		}
		if branchConfirmationState < lowestBranchConfirmationState {
			lowestBranchConfirmationState = branchConfirmationState
		}
	}
	return
}
