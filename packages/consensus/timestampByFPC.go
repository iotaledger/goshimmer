package consensus

import (
	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/hive.go/events"
)

// TimestampByFPCEvents defines all the events related to FCoB.
type TimestampByFPCEvents struct {
	// Fired when an opinion of a timestamp is formed.
	TimestampOpinionFormed *events.Event

	// Error gets called when FPC faces an error.
	Error *events.Event

	// Vote gets called when FPCTimestampEvents needs to vote.
	Vote *events.Event
}

type TimestampByFPC struct {
	Events *TimestampByFPCEvents

	tangle *tangle.Tangle
}

func NewTimestampByFPC(tangle *tangle.Tangle) (timestampByFPC *TimestampByFPC) {
	timestampByFPC = &TimestampByFPC{
		tangle: tangle,
		Events: &TimestampByFPCEvents{
			Error: events.NewEvent(events.ErrorCaller),
			Vote:  events.NewEvent(voteEvent),
		},
	}

	return
}

func (t *TimestampByFPC) Setup(timestampEvent *events.Event) {
	t.Events.TimestampOpinionFormed = timestampEvent
}

func (t *TimestampByFPC) Evaluate(messageID tangle.MessageID) {
	panic("Implement me")
}

func (t *TimestampByFPC) Opinion(messageID tangle.MessageID) (opinion bool) {
	panic("Implement me")
}
