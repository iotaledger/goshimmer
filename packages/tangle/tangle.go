package tangle

import (
	"github.com/iotaledger/hive.go/kvstore"
)

// Tangle represents the base layer of messages.
type Tangle struct {
	*MessageStore

	Events *Events
}

// New creates a new Tangle.
func New(store kvstore.KVStore) (result *Tangle) {
	result = &Tangle{
		MessageStore: NewMessageStore(store),
		Events:       newEvents(),
	}

	return
}

// Shutdown marks the tangle as stopped, so it will not accept any new messages (waits for all backgroundTasks to finish).
func (t *Tangle) Shutdown() *Tangle {
	t.MessageStore.Shutdown()

	return t
}
