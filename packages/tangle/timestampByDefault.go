package tangle

import (
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

	tangle *Tangle
}

func NewTimestampByDefault(tangle *Tangle) (timestampByDefault *TimestampByDefault) {
	timestampByDefault = &TimestampByDefault{
		tangle: tangle,
		Events: &TimestampByDefaultEvents{},
	}

	return
}

func (t *TimestampByDefault) Setup(timestampEvent *events.Event) {
	t.Events.TimestampOpinionFormed = timestampEvent
}

func (t *TimestampByDefault) Opinion(messageID MessageID) (opinion bool) {
	return true
}

func (t *TimestampByDefault) Evaluate(messageID MessageID) {
	t.tangle.Storage.Message(messageID).Consume(func(message *Message) {
		t.tangle.Storage.MessageMetadata(messageID).Consume(func(messageMetadata *MessageMetadata) {
			messageMetadata.SetTimestampOpinion(TimestampOpinion{
				Value: voter.Like,
				LoK:   Two,
			})
			t.Events.TimestampOpinionFormed.Trigger(messageID)
		})
	})
}
