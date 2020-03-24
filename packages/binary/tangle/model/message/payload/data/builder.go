package data

import "github.com/iotaledger/goshimmer/packages/binary/tangle/model/message/payload"

// BuildPayload constructs a data payload wrapped in an payload.Payload interface and returns it.
// It triggers PayloadConstructed event once it's done.
func BuildPayload(raw []byte) payload.Payload {
	var dataPayload payload.Payload = New(raw)

	// TODO: this creates a circular dependency. do we really need that event?
	//messagefactory.Events.PayloadConstructed.Trigger(dataPayload)

	return dataPayload
}
