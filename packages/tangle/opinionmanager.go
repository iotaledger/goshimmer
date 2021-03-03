package tangle

import (
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/hive.go/events"
)

// region OpinionManager ////////////////////////////////////////////////////////////////////////////////////////////////

// OpinionManager is the component in charge of forming opinions about timestamps and payloads.
type OpinionManager struct {
	Events OpinionFormerEvents

	tangle *Tangle
}

// NewOpinionManager returns a new OpinionManager.
func NewOpinionManager(tangle *Tangle) (opinionFormer *OpinionManager) {
	opinionFormer = &OpinionManager{
		Events: OpinionFormerEvents{
			MessageOpinionFormed: events.NewEvent(MessageIDEventHandler),
			TransactionConfirmed: events.NewEvent(MessageIDEventHandler),
		},

		tangle: tangle,
	}

	return
}

// Setup sets up the behavior of the component by making it attach to the relevant events of the other components.
func (o *OpinionManager) Setup() {
	if o.tangle.Options.ConsensusProvider == nil {
		o.tangle.Booker.Events.MessageBooked.Attach(events.NewClosure(func(messageID MessageID) {
			o.Events.MessageOpinionFormed.Trigger(messageID)
		}))
		return
	}

	o.tangle.Options.ConsensusProvider.Setup()
}

// Shutdown shuts down the component and persists its state.
func (o *OpinionManager) Shutdown() {
	if o.tangle.Options.ConsensusProvider == nil {
		return
	}

	o.tangle.Options.ConsensusProvider.Shutdown()
}

// PayloadLiked returns the opinion of the given MessageID.
func (o *OpinionManager) PayloadLiked(messageID MessageID) (liked bool) {
	o.tangle.Storage.Message(messageID).Consume(func(message *Message) {
		if message.Payload().Type() != ledgerstate.TransactionType {
			liked = true
			return
		}

		if o.tangle.Options.ConsensusProvider == nil {
			return
		}

		liked = o.tangle.Options.ConsensusProvider.PayloadLiked(messageID)
	})

	return
}

// MessageEligible returns whether the given messageID is marked as eligible.
func (o *OpinionManager) MessageEligible(messageID MessageID) (eligible bool) {
	if messageID == EmptyMessageID {
		return true
	}

	o.tangle.Storage.MessageMetadata(messageID).Consume(func(messageMetadata *MessageMetadata) {
		eligible = messageMetadata.IsEligible()
	})

	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region OpinionFormerEvents //////////////////////////////////////////////////////////////////////////////////////////

// OpinionFormerEvents defines all the events related to the opinion manager.
type OpinionFormerEvents struct {
	// Fired when an opinion of a message is formed.
	MessageOpinionFormed *events.Event

	// Fired when a transaction gets confirmed.
	TransactionConfirmed *events.Event
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region ConsensusProvider ////////////////////////////////////////////////////////////////////////////////////////////

type ConsensusProvider interface {
	Init(tangle *Tangle)
	Setup()
	PayloadLiked(messageID MessageID) (liked bool)
	Shutdown()
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
