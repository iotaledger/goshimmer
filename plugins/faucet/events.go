package faucet

import (
	"github.com/iotaledger/hive.go/generics/event"

	"github.com/iotaledger/goshimmer/packages/faucet"
)

// Events defines the events of the faucet.
type Events struct {
	// Fired when the messages per second metric is updated.
	WebAPIFaucetRequest *event.Event[*FaucetRequestEvent]
}

func newEvents() (new *Events) {
	return &Events{
		WebAPIFaucetRequest: event.New[*FaucetRequestEvent](),
	}
}

// FaucetRequestEvent represent a faucet request performed through web API.
type FaucetRequestEvent struct {
	Request *faucet.Payload
}
