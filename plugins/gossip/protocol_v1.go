package gossip

import (
    "bytes"
    "github.com/iotaledger/goshimmer/packages/byteutils"
    "github.com/iotaledger/goshimmer/packages/errors"
    "github.com/iotaledger/goshimmer/packages/identity"
    "github.com/iotaledger/goshimmer/packages/transaction"
    "strconv"
)

//region indentificationStateV1 ////////////////////////////////////////////////////////////////////////////////////////

type indentificationStateV1 struct {
    buffer []byte
    offset int
}

func newIndentificationStateV1() *indentificationStateV1 {
    return &indentificationStateV1{
        buffer: make([]byte, MARSHALLED_IDENTITY_TOTAL_SIZE),
        offset: 0,
    }
}

func (state *indentificationStateV1) Consume(protocol *protocol, data []byte, offset int, length int) (int, errors.IdentifiableError) {
    bytesRead := byteutils.ReadAvailableBytesToBuffer(state.buffer, state.offset, data, offset, length)

    state.offset += bytesRead
    if state.offset == MARSHALLED_IDENTITY_TOTAL_SIZE {
        if receivedIdentity, err := unmarshalIdentity(state.buffer); err != nil {
            return bytesRead, ErrInvalidAuthenticationMessage.Derive(err, "invalid authentication message")
        } else {
            if neighbor, exists := GetNeighbor(receivedIdentity.StringIdentifier); exists {
                protocol.Neighbor = neighbor
            } else {
                protocol.Neighbor = nil
            }

            protocol.Events.ReceiveIdentification.Trigger(receivedIdentity)

            protocol.CurrentState = newacceptanceStateV1()
            state.offset = 0
        }
    }

    return bytesRead, nil
}

func unmarshalIdentity(data []byte) (*identity.Identity, error) {
    identifier := data[MARSHALLED_IDENTITY_START:MARSHALLED_IDENTITY_END]

    if restoredIdentity, err := identity.FromSignedData(identifier, data[MARSHALLED_IDENTITY_SIGNATURE_START:MARSHALLED_IDENTITY_SIGNATURE_END]); err != nil {
        return nil, err
    } else {
        if bytes.Equal(identifier, restoredIdentity.Identifier) {
            return restoredIdentity, nil
        } else {
            return nil, errors.New("signature does not match claimed identity")
        }
    }
}

//endregion ////////////////////////////////////////////////////////////////////////////////////////////////////////////

//region acceptanceStateV1 /////////////////////////////////////////////////////////////////////////////////////////////

type acceptanceStateV1 struct {}

func newacceptanceStateV1() *acceptanceStateV1 {
    return &acceptanceStateV1{}
}

func (state *acceptanceStateV1) Consume(protocol *protocol, data []byte, offset int, length int) (int, errors.IdentifiableError) {
    switch data[offset] {
        case 0:
            protocol.Events.ReceiveConnectionRejected.Trigger()

            protocol.Conn.Close()

            protocol.CurrentState = nil
        break

        case 1:
            protocol.Events.ReceiveConnectionAccepted.Trigger()

            protocol.CurrentState = newDispatchStateV1()
        break

        default:
            return 1, ErrInvalidStateTransition.Derive("invalid acceptance state transition (" + strconv.Itoa(int(data[offset])) + ")")
    }

    return 1, nil
}

//endregion ////////////////////////////////////////////////////////////////////////////////////////////////////////////

//region dispatchStateV1 ///////////////////////////////////////////////////////////////////////////////////////////////

type dispatchStateV1 struct {}

func newDispatchStateV1() *dispatchStateV1 {
    return &dispatchStateV1{}
}

func (state *dispatchStateV1) Consume(protocol *protocol, data []byte, offset int, length int) (int, errors.IdentifiableError) {
    switch data[0] {
        case 0:
            protocol.Events.ReceiveConnectionRejected.Trigger()

            protocol.Neighbor.InitiatedConn.Close()
            protocol.CurrentState = nil

        case 1:
            protocol.CurrentState = newTransactionStateV1()
        break

        case 2:
            protocol.CurrentState = newRequestStateV1()
        break

        default:
            return 1, ErrInvalidStateTransition.Derive("invalid dispatch state transition (" + strconv.Itoa(int(data[offset])) + ")")
    }
    return 1, nil
}

//endregion ////////////////////////////////////////////////////////////////////////////////////////////////////////////

//region transactionStateV1 ////////////////////////////////////////////////////////////////////////////////////////////

type transactionStateV1 struct {
    buffer []byte
    offset int
}

func newTransactionStateV1() *transactionStateV1 {
    return &transactionStateV1{
        buffer: make([]byte, transaction.MARSHALLED_TOTAL_SIZE),
        offset: 0,
    }
}

func (state *transactionStateV1) Consume(protocol *protocol, data []byte, offset int, length int) (int, errors.IdentifiableError) {
    bytesRead := byteutils.ReadAvailableBytesToBuffer(state.buffer, state.offset, data, offset, length)

    state.offset += bytesRead
    if state.offset == transaction.MARSHALLED_TOTAL_SIZE {
        transactionData := make([]byte, transaction.MARSHALLED_TOTAL_SIZE)
        copy(transactionData, state.buffer)

        protocol.Events.ReceiveTransactionData.Trigger(transactionData)

        go processTransactionData(transactionData)

        protocol.CurrentState = newDispatchStateV1()
        state.offset = 0
    }

    return bytesRead, nil
}

//endregion ////////////////////////////////////////////////////////////////////////////////////////////////////////////

//region requestStateV1 ////////////////////////////////////////////////////////////////////////////////////////////////

type requestStateV1 struct {
    buffer []byte
    offset int
}

func newRequestStateV1() *requestStateV1 {
    return &requestStateV1{
        buffer: make([]byte, 1),
        offset: 0,
    }
}

func (state *requestStateV1) Consume(protocol *protocol, data []byte, offset int, length int) (int, errors.IdentifiableError) {
    return 0, nil
}

//endregion ////////////////////////////////////////////////////////////////////////////////////////////////////////////

const (
    MARSHALLED_IDENTITY_START = 0
    MARSHALLED_IDENTITY_SIGNATURE_START = MARSHALLED_IDENTITY_END

    MARSHALLED_IDENTITY_SIZE = 20
    MARSHALLED_IDENTITY_SIGNATURE_SIZE = 65

    MARSHALLED_IDENTITY_END = MARSHALLED_IDENTITY_START + MARSHALLED_IDENTITY_SIZE
    MARSHALLED_IDENTITY_SIGNATURE_END = MARSHALLED_IDENTITY_SIGNATURE_START + MARSHALLED_IDENTITY_SIGNATURE_SIZE

    MARSHALLED_IDENTITY_TOTAL_SIZE = MARSHALLED_IDENTITY_SIGNATURE_END
)