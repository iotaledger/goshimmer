package tangle

import (
	"sync"

	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/types"
)

type Opinioner interface {
	OnMessageBooked(messageID MessageID)
	SetupEvents(*events.Event, *events.Event)
}

// Events defines all the events related to the opinion manager.
type OpinionFormerEvents struct {
	// Fired when an opinion of a payload is formed.
	PayloadOpinionFormed *events.Event

	// Fired when an opinion of a timestamp is formed.
	TimestampOpinionFormed *events.Event

	// Fired when an opinion of a message is formed.
	MessageOpinionFormed *events.Event
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

type OpinionFormer struct {
	Events OpinionFormerEvents

	tangle  *Tangle
	waiting *opinionWait

	opinioner Opinioner
}

func NewOpinionFormer(tangle *Tangle, opinioner Opinioner) (opinionFormer *OpinionFormer) {
	opinionFormer = &OpinionFormer{
		tangle:    tangle,
		waiting:   &opinionWait{waitMap: make(map[MessageID]types.Empty)},
		opinioner: opinioner,
		Events: OpinionFormerEvents{
			PayloadOpinionFormed:   events.NewEvent(payloadOpinionCaller),
			TimestampOpinionFormed: events.NewEvent(messageIDEventHandler),
			MessageOpinionFormed:   events.NewEvent(messageIDEventHandler),
		},
	}

	return
}

func (o *OpinionFormer) Setup() {
	o.opinioner.SetupEvents(o.Events.PayloadOpinionFormed, o.Events.TimestampOpinionFormed)
	o.tangle.Booker.Events.MessageBooked.Attach(events.NewClosure(o.opinioner.OnMessageBooked))
	o.Events.PayloadOpinionFormed.Attach(events.NewClosure(o.onPayloadOpinionFormed))
	o.Events.TimestampOpinionFormed.Attach(events.NewClosure(o.onTimestampOpinionFormed))
}

func (o *OpinionFormer) onPayloadOpinionFormed(ev *OpinionFormedEvent) {
	if o.waiting.done(ev.MessageID) {
		o.Events.MessageOpinionFormed.Trigger(ev.MessageID)
	}
}

func (o *OpinionFormer) onTimestampOpinionFormed(ev *OpinionFormedEvent) {
	if o.waiting.done(ev.MessageID) {
		o.Events.MessageOpinionFormed.Trigger(ev.MessageID)
	}
}

type opinionWait struct {
	waitMap map[MessageID]types.Empty
	sync.Mutex
}

func (o *opinionWait) done(messageID MessageID) (done bool) {
	o.Lock()
	defer o.Unlock()
	if _, exist := o.waitMap[messageID]; !exist {
		o.waitMap[messageID] = types.Void
		return
	}
	delete(o.waitMap, messageID)
	done = true
	return
}
