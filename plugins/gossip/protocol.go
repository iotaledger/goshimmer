package gossip

import (
    "github.com/iotaledger/goshimmer/packages/errors"
    "strconv"
)

//region interfaces ////////////////////////////////////////////////////////////////////////////////////////////////////

type protocolState interface {
    Consume(protocol *protocol, data []byte, offset int, length int) (int, errors.IdentifiableError)
}

//endregion ////////////////////////////////////////////////////////////////////////////////////////////////////////////

// region protocol /////////////////////////////////////////////////////////////////////////////////////////////////////

type protocol struct {
    neighbor     *Neighbor
    currentState protocolState
}

func newProtocol(neighbor *Neighbor) *protocol {
    protocol := &protocol{
        neighbor:     neighbor,
        currentState: &versionState{},
    }

    return protocol
}

func (protocol *protocol) parseData(data []byte) {
    offset := 0
    length := len(data)
    for offset < length && protocol.currentState != nil {
        if readBytes, err := protocol.currentState.Consume(protocol, data, offset, length); err != nil {
            Events.Error.Trigger(err)

            protocol.neighbor.Conn.Close()

            return
        } else {
            offset += readBytes
        }
    }
}

// endregion ////////////////////////////////////////////////////////////////////////////////////////////////////////////

// region versionState //////////////////////////////////////////////////////////////////////////////////////////////////

type versionState struct{}

func (state *versionState) Consume(protocol *protocol, data []byte, offset int, length int) (int, errors.IdentifiableError) {
    switch data[offset] {
    case 1:
        Events.ReceiveVersion.Trigger(1)

        protocol.currentState = newIndentificationStateV1()

        return 1, nil

    default:
        return 1, ErrInvalidStateTransition.Derive("invalid version state transition (" + strconv.Itoa(int(data[offset])) + ")")
    }
}

// endregion ////////////////////////////////////////////////////////////////////////////////////////////////////////////
