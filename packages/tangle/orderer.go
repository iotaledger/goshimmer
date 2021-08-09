package tangle

import (
	"sync"

	"github.com/iotaledger/hive.go/events"
)

const inboxSize = 1024

// Orderer is a Tangle component that makes sure that no messages are booked without their parents being booked first.
type Orderer struct {
	Events *OrdererEvents

	tangle         *Tangle
	shutdownSignal chan struct{}
	shutdownWG     sync.WaitGroup
	shutdownOnce   sync.Once

	bookedMessageChan chan MessageID
	inbox             chan MessageID
	parentsMap        map[MessageID][]MessageID
}

// NewOrderer is the constructor for Orderer.
func NewOrderer(tangle *Tangle) (orderer *Orderer) {
	orderer = &Orderer{
		Events: &OrdererEvents{
			MessageOrdered: events.NewEvent(MessageIDCaller),
		},
		tangle:            tangle,
		shutdownSignal:    make(chan struct{}),
		bookedMessageChan: make(chan MessageID, inboxSize),
		inbox:             make(chan MessageID, inboxSize),
		parentsMap:        make(map[MessageID][]MessageID),
	}

	orderer.run()

	return
}

// Setup sets up the behavior of the component by making it attach to the relevant events of other components.
func (o *Orderer) Setup() {
	o.tangle.Scheduler.Events.MessageScheduled.Attach(events.NewClosure(o.onMessageScheduled))
}

// Shutdown shuts down the Orderer and persists its state.
func (o *Orderer) Shutdown() {
	o.shutdownOnce.Do(func() {
		close(o.shutdownSignal)
	})

	o.shutdownWG.Wait()
}

// run runs the background thread that listens to TangleTime updates (through a channel) and then schedules messages
// as the TangleTime advances forward.
func (o *Orderer) run() {
	o.shutdownWG.Add(1)
	go func() {
		defer o.shutdownWG.Done()

		for {
			select {
			case messageID := <-o.inbox:
				parentsToGossip := o.tryToGossip(messageID)
				o.updateParentsMap(messageID, parentsToGossip)

			case <-o.shutdownSignal:
				// no need to wait for unordered messages
				return
			}
		}
	}()
}

func (o *Orderer) parentsToGossip(messageID MessageID) (parents MessageIDs) {
	o.tangle.Storage.Message(messageID).Consume(func(message *Message) {
		message.ForEachParent(func(parent Parent) {
			o.tangle.Storage.MessageMetadata(parent.ID).Consume(func(messageMetadata *MessageMetadata) {
				if !messageMetadata.Scheduled() && !messageMetadata.ScheduledBypass() {
					parents = append(parents, parent.ID)
				}
			})
		})
	})

	return parents
}

func (o *Orderer) tryToGossip(messageID MessageID) (parentsToGossip []MessageID) {
	parentsToGossip = o.parentsToGossip(messageID)
	if len(parentsToGossip) > 0 {
		return
	}

	// all parents are scheduled
	o.Events.MessageOrdered.Trigger(messageID)

	return
}

func (o *Orderer) updateParentsMap(messageID MessageID, parentsToGossip []MessageID) {
	// try to order children if it's ordered
	if len(parentsToGossip) == 0 {
		if _, exists := o.parentsMap[messageID]; !exists {
			return
		}

		for _, childID := range o.parentsMap[messageID] {
			o.tryToGossip(childID)
		}
		delete(o.parentsMap, messageID)
		return
	}

	// update parent list if its parent(s) are not yet scheduled
	for _, parent := range parentsToGossip {
		if _, exists := o.parentsMap[parent]; !exists {
			o.parentsMap[parent] = make([]MessageID, 0)
		}
		o.parentsMap[parent] = append(o.parentsMap[parent], messageID)
	}
}

func (o *Orderer) onMessageScheduled(messageID MessageID) {
	o.inbox <- messageID
}

// region OrdererEvents ////////////////////////////////////////////////////////////////////////////////////////////////

// OrdererEvents represents events happening in the Orderer.
type OrdererEvents struct {
	// MessageOrdered is triggered when a message is ordered and thus ready to be booked.
	MessageOrdered *events.Event
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
