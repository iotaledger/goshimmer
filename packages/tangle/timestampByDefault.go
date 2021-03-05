package tangle

import (
	voter "github.com/iotaledger/goshimmer/packages/vote/opinion"
	"github.com/iotaledger/hive.go/events"
)

// region TimestampLikedByDefault //////////////////////////////////////////////////////////////////////////////////////

// TimestampLikedByDefault is the opinion provider that likes all the timestamps.
type TimestampLikedByDefault struct {
	Events *TimestampLikedByDefaultEvents

	tangle *Tangle
}

// NewTimestampLikedByDefault returns a new instance of the TimestampLikedByDefault.
func NewTimestampLikedByDefault(tangle *Tangle) (timestampByDefault *TimestampLikedByDefault) {
	timestampByDefault = &TimestampLikedByDefault{
		tangle: tangle,
		Events: &TimestampLikedByDefaultEvents{},
	}

	return
}

// Setup sets up the behavior of the component by making it attach to the relevant events of the other components.
// It is required to satisfy the OpinionProvider interface.
func (t *TimestampLikedByDefault) Setup(timestampEvent *events.Event) {
	t.Events.TimestampOpinionFormed = timestampEvent
}

// Shutdown shuts down component and persists its state. It is required to satisfy the OpinionProvider interface.
func (t *TimestampLikedByDefault) Shutdown() {}

// Opinion returns the liked status of a given messageID.
func (t *TimestampLikedByDefault) Opinion(messageID MessageID) (opinion bool) {
	return true
}

// Evaluate evaluates the opinion of the given messageID.
func (t *TimestampLikedByDefault) Evaluate(messageID MessageID) {
	t.tangle.Storage.MessageMetadata(messageID).Consume(func(messageMetadata *MessageMetadata) {
		messageMetadata.SetTimestampOpinion(TimestampOpinion{
			Value: voter.Like,
			LoK:   Two,
		})
		t.Events.TimestampOpinionFormed.Trigger(messageID)
	})
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region TimestampLikedByDefaultEvents ////////////////////////////////////////////////////////////////////////////////

// TimestampLikedByDefaultEvents defines all the events related to the TimestampLikedByDefault opinion provider.
type TimestampLikedByDefaultEvents struct {
	// Fired when an opinion of a timestamp is formed.
	TimestampOpinionFormed *events.Event
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
