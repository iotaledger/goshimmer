package gossip

import (
    "github.com/iotaledger/goshimmer/packages/events"
    "github.com/iotaledger/goshimmer/packages/transaction"
)

var Events = pluginEvents{
    AddNeighbor:        events.NewEvent(errorCaller),
    RemoveNeighbor:     events.NewEvent(errorCaller),
    DropNeighbor:       events.NewEvent(errorCaller),
    IncomingConnection: events.NewEvent(errorCaller),
    ReceiveTransaction: events.NewEvent(transactionCaller),
    Error:              events.NewEvent(errorCaller),
}

type pluginEvents struct {
    // neighbor events
    AddNeighbor    *events.Event
    RemoveNeighbor *events.Event
    DropNeighbor   *events.Event

    // low level network events
    IncomingConnection *events.Event

    // high level protocol events
    SendTransaction           *events.Event
    SendTransactionRequest    *events.Event
    ReceiveTransaction        *events.Event
    ReceiveTransactionRequest *events.Event
    ProtocolError             *events.Event

    // generic events
    Error *events.Event
}

func intCaller(handler interface{}, params ...interface{}) { handler.(func(int))(params[0].(int)) }

func errorCaller(handler interface{}, params ...interface{}) { handler.(func(error))(params[0].(error)) }

func transactionCaller(handler interface{}, params ...interface{}) { handler.(func(*transaction.Transaction))(params[0].(*transaction.Transaction)) }

type protocolEvents struct {
    ReceiveVersion         *events.Event
    ReceiveIdentification  *events.Event
    AcceptConnection       *events.Event
    RejectConnection       *events.Event
    DropConnection         *events.Event
    ReceiveTransactionData *events.Event
    ReceiveRequestData     *events.Event
    Error                  *events.Event
}
