package gossip

import (
	"bytes"
	"strconv"

	"github.com/iotaledger/goshimmer/packages/accountability"
	"github.com/iotaledger/goshimmer/packages/byteutils"
	"github.com/iotaledger/goshimmer/packages/errors"
	"github.com/iotaledger/goshimmer/packages/events"
	"github.com/iotaledger/goshimmer/packages/identity"
	"github.com/iotaledger/goshimmer/packages/model/meta_transaction"
	"github.com/iotaledger/iota.go/consts"
)

// region protocolV1 ///////////////////////////////////////////////////////////////////////////////////////////////////

func protocolV1(protocol *protocol) errors.IdentifiableError {
	if err := protocol.Send(accountability.OwnId()); err != nil {
		return err
	}

	onReceiveIdentification := events.NewClosure(func(identity *identity.Identity) {
		if protocol.Neighbor == nil {
			if err := protocol.Send(CONNECTION_REJECT); err != nil {
				return
			}
		} else {
			if err := protocol.Send(CONNECTION_ACCEPT); err != nil {
				return
			}

			protocol.handshakeMutex.Lock()
			defer protocol.handshakeMutex.Unlock()

			protocol.sendHandshakeCompleted = true
			if protocol.receiveHandshakeCompleted {
				protocol.Events.HandshakeCompleted.Trigger()
			}
		}
	})

	protocol.Events.ReceiveIdentification.Attach(onReceiveIdentification)

	return nil
}

func sendTransactionV1(protocol *protocol, tx *meta_transaction.MetaTransaction) {
	if _, ok := protocol.SendState.(*dispatchStateV1); ok {
		protocol.sendMutex.Lock()
		defer protocol.sendMutex.Unlock()

		if err := protocol.send(DISPATCH_TRANSACTION); err != nil {
			return
		}
		if err := protocol.send(tx); err != nil {
			return
		}
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region indentificationStateV1 ///////////////////////////////////////////////////////////////////////////////////////

type indentificationStateV1 struct {
	protocol *protocol
	buffer   []byte
	offset   int
}

func newIndentificationStateV1(protocol *protocol) *indentificationStateV1 {
	return &indentificationStateV1{
		protocol: protocol,
		buffer:   make([]byte, MARSHALED_IDENTITY_TOTAL_SIZE),
		offset:   0,
	}
}

func (state *indentificationStateV1) Receive(data []byte, offset int, length int) (int, errors.IdentifiableError) {
	bytesRead := byteutils.ReadAvailableBytesToBuffer(state.buffer, state.offset, data, offset, length)

	state.offset += bytesRead
	if state.offset == MARSHALED_IDENTITY_TOTAL_SIZE {
		if receivedIdentity, err := unmarshalIdentity(state.buffer); err != nil {
			return bytesRead, ErrInvalidAuthenticationMessage.Derive(err, "invalid authentication message")
		} else {
			protocol := state.protocol

			if neighbor, exists := GetNeighbor(receivedIdentity.StringIdentifier); exists {
				protocol.Neighbor = neighbor
			} else {
				protocol.Neighbor = nil
			}

			protocol.Events.ReceiveIdentification.Trigger(receivedIdentity)

			protocol.ReceivingState = newacceptanceStateV1(protocol)
			state.offset = 0
		}
	}

	return bytesRead, nil
}

func (state *indentificationStateV1) Send(param interface{}) errors.IdentifiableError {
	if id, ok := param.(*identity.Identity); ok {
		if signature, err := id.Sign(id.Identifier); err == nil {
			protocol := state.protocol

			if _, err := protocol.Conn.Write(id.Identifier); err != nil {
				return ErrSendFailed.Derive(err, "failed to send identifier")
			}
			if _, err := protocol.Conn.Write(signature); err != nil {
				return ErrSendFailed.Derive(err, "failed to send signature")
			}

			protocol.SendState = newacceptanceStateV1(protocol)

			return nil
		}
	}

	return ErrInvalidSendParam.Derive("passed in parameter is not a valid identity")
}

func unmarshalIdentity(data []byte) (*identity.Identity, error) {
	identifier := data[MARSHALED_IDENTITY_START:MARSHALED_IDENTITY_END]

	if restoredIdentity, err := identity.FromSignedData(identifier, data[MARSHALED_IDENTITY_SIGNATURE_START:MARSHALED_IDENTITY_SIGNATURE_END]); err != nil {
		return nil, err
	} else {
		if bytes.Equal(identifier, restoredIdentity.Identifier) {
			return restoredIdentity, nil
		} else {
			return nil, errors.New("signature does not match claimed identity")
		}
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region acceptanceStateV1 ////////////////////////////////////////////////////////////////////////////////////////////

type acceptanceStateV1 struct {
	protocol *protocol
}

func newacceptanceStateV1(protocol *protocol) *acceptanceStateV1 {
	return &acceptanceStateV1{protocol: protocol}
}

func (state *acceptanceStateV1) Receive(data []byte, offset int, length int) (int, errors.IdentifiableError) {
	protocol := state.protocol

	switch data[offset] {
	case 0:
		protocol.Events.ReceiveConnectionRejected.Trigger()

		_ = protocol.Conn.Close()

		protocol.ReceivingState = nil

	case 1:
		protocol.Events.ReceiveConnectionAccepted.Trigger()

		protocol.ReceivingState = newDispatchStateV1(protocol)

	default:
		return 1, ErrInvalidStateTransition.Derive("invalid acceptance state transition (" + strconv.Itoa(int(data[offset])) + ")")
	}

	return 1, nil
}

func (state *acceptanceStateV1) Send(param interface{}) errors.IdentifiableError {
	if responseType, ok := param.(byte); ok {
		switch responseType {
		case CONNECTION_REJECT:
			protocol := state.protocol

			if _, err := protocol.Conn.Write([]byte{CONNECTION_REJECT}); err != nil {
				return ErrSendFailed.Derive(err, "failed to send reject message")
			}

			_ = protocol.Conn.Close()

			protocol.SendState = nil

			return nil

		case CONNECTION_ACCEPT:
			protocol := state.protocol

			if _, err := protocol.Conn.Write([]byte{CONNECTION_ACCEPT}); err != nil {
				return ErrSendFailed.Derive(err, "failed to send accept message")
			}

			protocol.SendState = newDispatchStateV1(protocol)

			return nil
		}
	}

	return ErrInvalidSendParam.Derive("passed in parameter is not a valid acceptance byte")
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region dispatchStateV1 //////////////////////////////////////////////////////////////////////////////////////////////

type dispatchStateV1 struct {
	protocol *protocol
}

func newDispatchStateV1(protocol *protocol) *dispatchStateV1 {
	return &dispatchStateV1{
		protocol: protocol,
	}
}

func (state *dispatchStateV1) Receive(data []byte, offset int, length int) (int, errors.IdentifiableError) {
	switch data[0] {
	case DISPATCH_DROP:
		protocol := state.protocol

		protocol.Events.ReceiveConnectionRejected.Trigger()

		_ = protocol.Conn.Close()

		protocol.ReceivingState = nil

	case DISPATCH_TRANSACTION:
		protocol := state.protocol

		protocol.ReceivingState = newTransactionStateV1(protocol)

	case DISPATCH_REQUEST:
		protocol := state.protocol

		protocol.ReceivingState = newRequestStateV1(protocol)

	default:
		return 1, ErrInvalidStateTransition.Derive("invalid dispatch state transition (" + strconv.Itoa(int(data[offset])) + ")")
	}

	return 1, nil
}

func (state *dispatchStateV1) Send(param interface{}) errors.IdentifiableError {
	if dispatchByte, ok := param.(byte); ok {
		switch dispatchByte {
		case DISPATCH_DROP:
			protocol := state.protocol

			if _, err := protocol.Conn.Write([]byte{DISPATCH_DROP}); err != nil {
				return ErrSendFailed.Derive(err, "failed to send drop message")
			}

			_ = protocol.Conn.Close()

			protocol.SendState = nil

			return nil

		case DISPATCH_TRANSACTION:
			protocol := state.protocol

			if _, err := protocol.Conn.Write([]byte{DISPATCH_TRANSACTION}); err != nil {
				return ErrSendFailed.Derive(err, "failed to send transaction dispatch byte")
			}

			protocol.SendState = newTransactionStateV1(protocol)

			return nil

		case DISPATCH_REQUEST:
			protocol := state.protocol

			if _, err := protocol.Conn.Write([]byte{DISPATCH_REQUEST}); err != nil {
				return ErrSendFailed.Derive(err, "failed to send request dispatch byte")
			}

			protocol.SendState = newTransactionStateV1(protocol)

			return nil
		}
	}

	return ErrInvalidSendParam.Derive("passed in parameter is not a valid dispatch byte")
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region transactionStateV1 ///////////////////////////////////////////////////////////////////////////////////////////

type transactionStateV1 struct {
	protocol *protocol
	buffer   []byte
	offset   int
}

func newTransactionStateV1(protocol *protocol) *transactionStateV1 {
	return &transactionStateV1{
		protocol: protocol,
		buffer:   make([]byte, meta_transaction.MARSHALED_TOTAL_SIZE/consts.NumberOfTritsInAByte),
		offset:   0,
	}
}

func (state *transactionStateV1) Receive(data []byte, offset int, length int) (int, errors.IdentifiableError) {
	bytesRead := byteutils.ReadAvailableBytesToBuffer(state.buffer, state.offset, data, offset, length)

	state.offset += bytesRead
	if state.offset == meta_transaction.MARSHALED_TOTAL_SIZE/consts.NumberOfTritsInAByte {
		protocol := state.protocol

		transactionData := make([]byte, meta_transaction.MARSHALED_TOTAL_SIZE/consts.NumberOfTritsInAByte)
		copy(transactionData, state.buffer)

		protocol.Events.ReceiveTransactionData.Trigger(transactionData)

		go ProcessReceivedTransactionData(transactionData)

		protocol.ReceivingState = newDispatchStateV1(protocol)
		state.offset = 0
	}

	return bytesRead, nil
}

func (state *transactionStateV1) Send(param interface{}) errors.IdentifiableError {
	if tx, ok := param.(*meta_transaction.MetaTransaction); ok {
		protocol := state.protocol

		if _, err := protocol.Conn.Write(tx.GetBytes()); err != nil {
			return ErrSendFailed.Derive(err, "failed to send transaction")
		}

		protocol.SendState = newDispatchStateV1(protocol)

		return nil
	}

	return ErrInvalidSendParam.Derive("passed in parameter is not a valid transaction")
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region requestStateV1 ///////////////////////////////////////////////////////////////////////////////////////////////

type requestStateV1 struct {
	buffer []byte
	offset int
}

func newRequestStateV1(protocol *protocol) *requestStateV1 {
	return &requestStateV1{
		buffer: make([]byte, 1),
		offset: 0,
	}
}

func (state *requestStateV1) Receive(data []byte, offset int, length int) (int, errors.IdentifiableError) {
	return 0, nil
}

func (state *requestStateV1) Send(param interface{}) errors.IdentifiableError {
	return nil
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region constants and variables //////////////////////////////////////////////////////////////////////////////////////

const (
	VERSION_1 = byte(1)

	CONNECTION_REJECT = byte(0)
	CONNECTION_ACCEPT = byte(1)

	DISPATCH_DROP        = byte(0)
	DISPATCH_TRANSACTION = byte(1)
	DISPATCH_REQUEST     = byte(2)

	MARSHALED_IDENTITY_START           = 0
	MARSHALED_IDENTITY_SIGNATURE_START = MARSHALED_IDENTITY_END

	MARSHALED_IDENTITY_SIZE           = 20
	MARSHALED_IDENTITY_SIGNATURE_SIZE = 65

	MARSHALED_IDENTITY_END           = MARSHALED_IDENTITY_START + MARSHALED_IDENTITY_SIZE
	MARSHALED_IDENTITY_SIGNATURE_END = MARSHALED_IDENTITY_SIGNATURE_START + MARSHALED_IDENTITY_SIGNATURE_SIZE

	MARSHALED_IDENTITY_TOTAL_SIZE = MARSHALED_IDENTITY_SIGNATURE_END
)

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
