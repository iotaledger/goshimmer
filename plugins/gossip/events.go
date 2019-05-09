package gossip

import (
    "github.com/iotaledger/goshimmer/packages/events"
    "github.com/iotaledger/goshimmer/packages/transaction"
)

var Events = pluginEvents{
    AddNeighbor:        events.NewEvent(neighborCaller),
    UpdateNeighbor:     events.NewEvent(neighborCaller),
    RemoveNeighbor:     events.NewEvent(neighborCaller),
    DropNeighbor:       events.NewEvent(neighborCaller),
    IncomingConnection: events.NewEvent(errorCaller),
    ReceiveTransaction: events.NewEvent(transactionCaller),
    Error:              events.NewEvent(errorCaller),
}

type pluginEvents struct {
    // neighbor events
    AddNeighbor    *events.Event
    UpdateNeighbor *events.Event
    RemoveNeighbor *events.Event

    // low level network events
    IncomingConnection *events.Event

    // high level protocol events
    DropNeighbor              *events.Event
    SendTransaction           *events.Event
    SendTransactionRequest    *events.Event
    ReceiveTransaction        *events.Event
    ReceiveTransactionRequest *events.Event
    ProtocolError             *events.Event

    // generic events
    Error *events.Event
}

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

func intCaller(handler interface{}, params ...interface{}) { handler.(func(int))(params[0].(int)) }

func neighborCaller(handler interface{}, params ...interface{}) { handler.(func(*Peer))(params[0].(*Peer)) }

func errorCaller(handler interface{}, params ...interface{}) { handler.(func(error))(params[0].(error)) }

func transactionCaller(handler interface{}, params ...interface{}) { handler.(func(*transaction.Transaction))(params[0].(*transaction.Transaction)) }
