package tangle

import (
	"github.com/iotaledger/hive.go/events"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
)

// region ConsensusManager /////////////////////////////////////////////////////////////////////////////////////////////

// ConsensusManager is the component in charge of forming opinions about timestamps and payloads.
type ConsensusManager struct {
	Events *ConsensusManagerEvents

	tangle *Tangle
}

// NewConsensusManager returns a new ConsensusManager.
func NewConsensusManager(tangle *Tangle) (opinionFormer *ConsensusManager) {
	opinionFormer = &ConsensusManager{
		Events: &ConsensusManagerEvents{
			MessageOpinionFormed: events.NewEvent(MessageIDCaller),
			TransactionConfirmed: events.NewEvent(MessageIDCaller),
		},

		tangle: tangle,
	}

	return
}

// Setup sets up the behavior of the component by making it attach to the relevant events of the other components.
func (o *ConsensusManager) Setup() {
	if o.tangle.Options.ConsensusMechanism == nil {
		o.tangle.Booker.Events.MessageBooked.Attach(events.NewClosure(func(messageID MessageID) {
			o.Events.MessageOpinionFormed.Trigger(messageID)
		}))
		return
	}

	o.tangle.Options.ConsensusMechanism.Setup()
}

// Shutdown shuts down the component and persists its state.
func (o *ConsensusManager) Shutdown() {
	if o.tangle.Options.ConsensusMechanism == nil {
		return
	}

	o.tangle.Options.ConsensusMechanism.Shutdown()
}

// PayloadLiked returns the opinion of the given MessageID.
func (o *ConsensusManager) PayloadLiked(messageID MessageID) (liked bool) {
	o.tangle.Storage.Message(messageID).Consume(func(message *Message) {
		if message.Payload().Type() != ledgerstate.TransactionType {
			liked = true
			return
		}

		if o.tangle.Options.ConsensusMechanism == nil {
			return
		}

		liked = o.tangle.Options.ConsensusMechanism.TransactionLiked(message.Payload().(*ledgerstate.Transaction).ID())
	})

	return
}

// MessageEligible returns whether the given messageID is marked as eligible.
func (o *ConsensusManager) MessageEligible(messageID MessageID) (eligible bool) {
	if messageID == EmptyMessageID {
		return true
	}

	o.tangle.Storage.MessageMetadata(messageID).Consume(func(messageMetadata *MessageMetadata) {
		eligible = messageMetadata.IsEligible()
	})

	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region ConsensusManagerEvents ///////////////////////////////////////////////////////////////////////////////////////

// ConsensusManagerEvents defines all the events related to the opinion manager.
type ConsensusManagerEvents struct {
	// Fired when an opinion of a message is formed.
	MessageOpinionFormed *events.Event

	// Fired when a transaction gets confirmed.
	TransactionConfirmed *events.Event
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region ConsensusMechanism ///////////////////////////////////////////////////////////////////////////////////////////

// ConsensusMechanism is a generic interface allowing the Tangle to use different methods to reach consensus.
type ConsensusMechanism interface {
	// Init initializes the ConsensusMechanism by making the Tangle object available that is using it.
	Init(tangle *Tangle)

	// Setup sets up the behavior of the ConsensusMechanism by making it attach to the relevant events in the Tangle.
	Setup()

	// TransactionLiked returns a boolean value indicating whether the given Transaction is liked.
	TransactionLiked(transactionID ledgerstate.TransactionID) (liked bool)

	// Shutdown shuts down the ConsensusMechanism and persists its state.
	Shutdown()
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
