package transactionfactory

import (
	"github.com/iotaledger/goshimmer/packages/binary/tangle/model/message"
	"github.com/iotaledger/goshimmer/packages/binary/tangle/model/message/payload"
	"github.com/iotaledger/hive.go/events"
)

var Events = struct {
	// A PayloadConstructed event is triggered when a transaction's payload is built.
	// Each ontology should implement a PayloadBuilder which triggers PayloadConstructed.
	PayloadConstructed *events.Event
	// A TransactionConstructed event is triggered when a transaction is built including tips, sequence number and other metadata.
	TransactionConstructed *events.Event
}{
	PayloadConstructed:     events.NewEvent(payloadConstructedEvent),
	TransactionConstructed: events.NewEvent(transactionConstructedEvent),
}

func payloadConstructedEvent(handler interface{}, params ...interface{}) {
	handler.(func(*payload.Payload))(params[0].(*payload.Payload))
}

func transactionConstructedEvent(handler interface{}, params ...interface{}) {
	handler.(func(*message.Transaction))(params[0].(*message.Transaction))
}
