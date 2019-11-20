package gossip

import (
	"strconv"
	"sync"

	"github.com/iotaledger/goshimmer/packages/errors"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/goshimmer/packages/network"
)

// region constants and variables //////////////////////////////////////////////////////////////////////////////////////

var DEFAULT_PROTOCOL = protocolDefinition{
	version:     VERSION_1,
	initializer: protocolV1,
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region protocol /////////////////////////////////////////////////////////////////////////////////////////////////////

type protocol struct {
	Conn                      *network.ManagedConnection
	Neighbor                  *Neighbor
	Version                   byte
	sendHandshakeCompleted    bool
	receiveHandshakeCompleted bool
	SendState                 protocolState
	ReceivingState            protocolState
	Events                    protocolEvents
	sendMutex                 sync.Mutex
	handshakeMutex            sync.Mutex
}

func newProtocol(conn *network.ManagedConnection) *protocol {
	protocol := &protocol{
		Conn: conn,
		Events: protocolEvents{
			ReceiveVersion:            events.NewEvent(intCaller),
			ReceiveIdentification:     events.NewEvent(identityCaller),
			ReceiveConnectionAccepted: events.NewEvent(events.CallbackCaller),
			ReceiveConnectionRejected: events.NewEvent(events.CallbackCaller),
			ReceiveTransactionData:    events.NewEvent(dataCaller),
			HandshakeCompleted:        events.NewEvent(events.CallbackCaller),
			Error:                     events.NewEvent(errorCaller),
		},
		sendHandshakeCompleted:    false,
		receiveHandshakeCompleted: false,
	}

	protocol.SendState = &versionState{protocol: protocol}
	protocol.ReceivingState = &versionState{protocol: protocol}

	return protocol
}

func (protocol *protocol) Init() {
	// setup event handlers
	onReceiveData := events.NewClosure(protocol.Receive)
	onConnectionAccepted := events.NewClosure(func() {
		protocol.handshakeMutex.Lock()
		defer protocol.handshakeMutex.Unlock()

		protocol.receiveHandshakeCompleted = true
		if protocol.sendHandshakeCompleted {
			protocol.Events.HandshakeCompleted.Trigger()
		}
	})
	var onClose *events.Closure
	onClose = events.NewClosure(func() {
		protocol.Conn.Events.ReceiveData.Detach(onReceiveData)
		protocol.Conn.Events.Close.Detach(onClose)
		protocol.Events.ReceiveConnectionAccepted.Detach(onConnectionAccepted)
	})

	// region register event handlers
	protocol.Conn.Events.ReceiveData.Attach(onReceiveData)
	protocol.Conn.Events.Close.Attach(onClose)
	protocol.Events.ReceiveConnectionAccepted.Attach(onConnectionAccepted)

	// send protocol version
	if err := protocol.Send(DEFAULT_PROTOCOL.version); err != nil {
		return
	}

	// initialize default protocol
	if err := DEFAULT_PROTOCOL.initializer(protocol); err != nil {
		protocol.SendState = nil

		_ = protocol.Conn.Close()

		protocol.Events.Error.Trigger(err)

		return
	}

	// start reading from the connection
	_, _ = protocol.Conn.Read(make([]byte, 1000))
}

func (protocol *protocol) Receive(data []byte) {
	offset := 0
	length := len(data)
	for offset < length && protocol.ReceivingState != nil {
		if readBytes, err := protocol.ReceivingState.Receive(data, offset, length); err != nil {
			Events.Error.Trigger(err)

			_ = protocol.Conn.Close()

			return
		} else {
			offset += readBytes
		}
	}
}

func (protocol *protocol) Send(data interface{}) errors.IdentifiableError {
	protocol.sendMutex.Lock()
	defer protocol.sendMutex.Unlock()

	return protocol.send(data)
}

func (protocol *protocol) send(data interface{}) errors.IdentifiableError {
	if protocol.SendState != nil {
		if err := protocol.SendState.Send(data); err != nil {
			protocol.SendState = nil

			_ = protocol.Conn.Close()

			protocol.Events.Error.Trigger(err)

			return err
		}
	}

	return nil
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region versionState /////////////////////////////////////////////////////////////////////////////////////////////////

type versionState struct {
	protocol *protocol
}

func (state *versionState) Receive(data []byte, offset int, length int) (int, errors.IdentifiableError) {
	switch data[offset] {
	case 1:
		protocol := state.protocol

		protocol.Version = 1
		protocol.Events.ReceiveVersion.Trigger(1)

		protocol.ReceivingState = newIndentificationStateV1(protocol)

		return 1, nil

	default:
		return 1, ErrInvalidStateTransition.Derive("invalid version state transition (" + strconv.Itoa(int(data[offset])) + ")")
	}
}

func (state *versionState) Send(param interface{}) errors.IdentifiableError {
	if version, ok := param.(byte); ok {
		switch version {
		case VERSION_1:
			protocol := state.protocol

			if _, err := protocol.Conn.Write([]byte{version}); err != nil {
				return ErrSendFailed.Derive(err, "failed to send version byte")
			}

			protocol.SendState = newIndentificationStateV1(protocol)

			return nil
		}
	}

	return ErrInvalidSendParam.Derive("passed in parameter is not a valid version byte")
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region types and interfaces /////////////////////////////////////////////////////////////////////////////////////////

type protocolState interface {
	Send(param interface{}) errors.IdentifiableError
	Receive(data []byte, offset int, length int) (int, errors.IdentifiableError)
}

type protocolDefinition struct {
	version     byte
	initializer func(*protocol) errors.IdentifiableError
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
