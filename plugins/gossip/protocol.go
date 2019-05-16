package gossip

import (
    "github.com/iotaledger/goshimmer/packages/accountability"
    "github.com/iotaledger/goshimmer/packages/errors"
    "github.com/iotaledger/goshimmer/packages/events"
    "github.com/iotaledger/goshimmer/packages/identity"
    "github.com/iotaledger/goshimmer/packages/network"
    "strconv"
)

// region interfaces ///////////////////////////////////////////////////////////////////////////////////////////////////

type protocolState interface {
    Consume(protocol *protocol, data []byte, offset int, length int) (int, errors.IdentifiableError)
}

// endregion ////////////////////////////////////////////////////////////////////////////////////////////////////////////

//region protocol //////////////////////////////////////////////////////////////////////////////////////////////////////

type protocol struct {
    Conn         *network.ManagedConnection
    Neighbor     *Peer
    Version      int
    CurrentState protocolState
    Events       protocolEvents
}

func newProtocol(conn *network.ManagedConnection) *protocol {
    protocol := &protocol{
        Conn:         conn,
        CurrentState: &versionState{},
        Events: protocolEvents{
            ReceiveVersion:            events.NewEvent(intCaller),
            ReceiveIdentification:     events.NewEvent(identityCaller),
            ReceiveConnectionAccepted: events.NewEvent(events.CallbackCaller),
            ReceiveConnectionRejected: events.NewEvent(events.CallbackCaller),
        },
    }

    return protocol
}

func (protocol *protocol) sendVersion() {
    protocol.Conn.Write([]byte{1})
}

func (protocol *protocol) sendIdentification() {
    if signature, err := accountability.OWN_ID.Sign(accountability.OWN_ID.Identifier); err == nil {
        protocol.Conn.Write(accountability.OWN_ID.Identifier)
        protocol.Conn.Write(signature)
    }
}

func (protocol *protocol) rejectConnection() {
    protocol.Conn.Write([]byte{0})

    protocol.Conn.Close()
}

func (protocol *protocol) acceptConnection() {
    protocol.Conn.Write([]byte{1})
}

func (protocol *protocol) init() {
    //region setup event handlers
    onReceiveIdentification := events.NewClosure(func(identity *identity.Identity) {
        if protocol.Neighbor == nil {
            protocol.rejectConnection()
        } else {
            protocol.acceptConnection()
        }
    })

    onReceiveData := events.NewClosure(protocol.parseData)

    var onClose *events.Closure // define var first so we can use it in the closure
    onClose = events.NewClosure(func() {
        protocol.Conn.Events.ReceiveData.Detach(onReceiveData)
        protocol.Conn.Events.Close.Detach(onClose)
    })
    //endregion

    //region register event handlers
    protocol.Events.ReceiveIdentification.Attach(onReceiveIdentification)
    protocol.Conn.Events.ReceiveData.Attach(onReceiveData)
    protocol.Conn.Events.Close.Attach(onClose)
    //endregion

    //region send initial handshake
    protocol.sendVersion()
    protocol.sendIdentification()
    //endregion

    // start reading from the connection
    protocol.Conn.Read(make([]byte, 1000))
}

func (protocol *protocol) parseData(data []byte) {
    offset := 0
    length := len(data)
    for offset < length && protocol.CurrentState != nil {
        if readBytes, err := protocol.CurrentState.Consume(protocol, data, offset, length); err != nil {
            Events.Error.Trigger(err)

            protocol.Neighbor.InitiatedConn.Close()

            return
        } else {
            offset += readBytes
        }
    }
}

// endregion ////////////////////////////////////////////////////////////////////////////////////////////////////////////

// region versionState /////////////////////////////////////////////////////////////////////////////////////////////////

type versionState struct{}

func (state *versionState) Consume(protocol *protocol, data []byte, offset int, length int) (int, errors.IdentifiableError) {
    switch data[offset] {
    case 1:
        protocol.Version = 1
        protocol.Events.ReceiveVersion.Trigger(1)

        protocol.CurrentState = newIndentificationStateV1()

        return 1, nil

    default:
        return 1, ErrInvalidStateTransition.Derive("invalid version state transition (" + strconv.Itoa(int(data[offset])) + ")")
    }
}

// endregion ////////////////////////////////////////////////////////////////////////////////////////////////////////////
