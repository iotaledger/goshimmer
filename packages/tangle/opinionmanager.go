package tangle

import (
	"github.com/iotaledger/hive.go/events"
)

// region OpinionManager ////////////////////////////////////////////////////////////////////////////////////////////////

// OpinionManager is the component in charge of forming opinions about timestamps and payloads.
type OpinionManager struct {
	Events OpinionFormerEvents

	tangle    *Tangle
	consensus ConsensusProvider
}

// NewOpinionManager returns a new OpinionManager.
func NewOpinionManager(tangle *Tangle, consensus ConsensusProvider) (opinionFormer *OpinionManager) {
	opinionFormer = &OpinionManager{
		Events: OpinionFormerEvents{
			PayloadOpinionFormed:   events.NewEvent(payloadOpinionCaller),
			TimestampOpinionFormed: events.NewEvent(messageIDEventHandler),
			MessageOpinionFormed:   events.NewEvent(messageIDEventHandler),
			TransactionConfirmed:   events.NewEvent(messageIDEventHandler),
		},

		tangle:    tangle,
		consensus: consensus,
	}

	return
}

// Setup sets up the behavior of the component by making it attach to the relevant events of the other components.
func (o *OpinionManager) Setup() {
	if o == nil {
		return
	}

	o.consensus.Setup(o.tangle)
}

// Shutdown shuts down the component and persists its state.
func (o *OpinionManager) Shutdown() {
	if o == nil {
		return
	}

	o.consensus.Shutdown()
}

// PayloadLiked returns the opinion of the given MessageID.
func (o *OpinionManager) PayloadLiked(messageID MessageID) (liked bool) {
	if o == nil {
		return
	}

	return o.consensus.PayloadLiked(messageID)
}

// MessageEligible returns whether the given messageID is marked as eligible.
func (o *OpinionManager) MessageEligible(messageID MessageID) (eligible bool) {
	if o == nil {
		return
	}

	// return true if the message is the Genesis
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
	// Fired when an opinion of a payload is formed.
	PayloadOpinionFormed *events.Event

	// Fired when an opinion of a timestamp is formed.
	TimestampOpinionFormed *events.Event

	// Fired when an opinion of a message is formed.
	MessageOpinionFormed *events.Event

	// Fired when a transaction gets confirmed.
	TransactionConfirmed *events.Event
}

// OpinionFormedEvent holds data about a Payload/MessageOpinionFormed event.
type OpinionFormedEvent struct {
	// The messageID of the message containing the payload.
	MessageID MessageID
	// The opinion of the payload.
	Opinion bool
}

func payloadOpinionCaller(handler interface{}, params ...interface{}) {
	handler.(func(*OpinionFormedEvent))(params[0].(*OpinionFormedEvent))
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region ConsensusProvider ////////////////////////////////////////////////////////////////////////////////////////////

type ConsensusProvider interface {
	Setup(tangle *Tangle)
	EvaluateMessage(messageID MessageID)
	PayloadLiked(messageID MessageID) (liked bool)
	Shutdown()
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
