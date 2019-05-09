package gossip

import (
    "github.com/iotaledger/goshimmer/packages/byteutils"
    "github.com/iotaledger/goshimmer/packages/errors"
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
        buffer: make([]byte, MARSHALLED_NEIGHBOR_TOTAL_SIZE),
        offset: 0,
    }
}

func (state *indentificationStateV1) Consume(protocol *protocol, data []byte, offset int, length int) (int, errors.IdentifiableError) {
    bytesRead := byteutils.ReadAvailableBytesToBuffer(state.buffer, state.offset, data, offset, length)

    state.offset += bytesRead
    if state.offset == MARSHALLED_NEIGHBOR_TOTAL_SIZE {
        if unmarshalledNeighbor, err := UnmarshalNeighbor(state.buffer); err != nil {
            return bytesRead, ErrInvalidAuthenticationMessage.Derive(err, "invalid authentication message")
        } else {
            protocol.neighbor.Identity = unmarshalledNeighbor.Identity
            protocol.neighbor.Port = unmarshalledNeighbor.Port

            Events.ReceiveIdentification.Trigger(protocol.neighbor)

            protocol.currentState = newacceptanceStateV1()
            state.offset = 0
        }
    }

    return bytesRead, nil
}

//endregion ////////////////////////////////////////////////////////////////////////////////////////////////////////////

//region acceptanceStateV1 /////////////////////////////////////////////////////////////////////////////////////////////

type acceptanceStateV1 struct {}

func newacceptanceStateV1() *acceptanceStateV1 {
    return &acceptanceStateV1{}
}

func (state *acceptanceStateV1) Consume(protocol *protocol, data []byte, offset int, length int) (int, errors.IdentifiableError) {
    switch data[offset] {
        case 1:
            Events.AcceptConnection.Trigger()

            protocol.currentState = newDispatchStateV1()
        break

        case 2:
            Events.RejectConnection.Trigger()

            protocol.neighbor.Conn.Close()
            protocol.currentState = nil
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
            Events.RejectConnection.Trigger()

            protocol.neighbor.Conn.Close()
            protocol.currentState = nil

        case 1:
            protocol.currentState = newTransactionStateV1()
        break

        case 2:
            protocol.currentState = newRequestStateV1()
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

        Events.ReceiveTransactionData.Trigger(transactionData)

        go func() {
            Events.ReceiveTransaction.Trigger(transaction.FromBytes(transactionData))
        }()

        protocol.currentState = newDispatchStateV1()
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
