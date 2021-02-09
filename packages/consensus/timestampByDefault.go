package consensus

import (
	"github.com/iotaledger/goshimmer/packages/tangle"
	voter "github.com/iotaledger/goshimmer/packages/vote/opinion"
	"github.com/iotaledger/hive.go/events"
)

// TimestampByFPCEvents defines all the events related to FCoB.
type TimestampByDefaultEvents struct {
	// Fired when an opinion of a timestamp is formed.
	TimestampOpinionFormed *events.Event
}

type TimestampByDefault struct {
	Events *TimestampByDefaultEvents

	tangle *tangle.Tangle
}

func NewTimestampByDefault(tangle *tangle.Tangle) (timestampByDefault *TimestampByDefault) {
	timestampByDefault = &TimestampByDefault{
		tangle: tangle,
		Events: &TimestampByDefaultEvents{},
	}

	return
}

func (t *TimestampByDefault) Setup(timestampEvent *events.Event) {
	t.Events.TimestampOpinionFormed = timestampEvent
}

func (t *TimestampByDefault) Evaluate(messageID tangle.MessageID) {
	t.tangle.Storage.Message(messageID).Consume(func(message *tangle.Message) {
		t.tangle.Storage.MessageMetadata(messageID).Consume(func(messageMetadata *tangle.MessageMetadata) {
			messageMetadata.SetTimestampOpinion(tangle.TimestampOpinion{
				Value: voter.Like,
				LoK:   Two,
			})
			t.Events.TimestampOpinionFormed.Trigger(messageID)
		})
	})
}
