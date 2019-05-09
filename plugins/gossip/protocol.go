package gossip

import (
    "github.com/iotaledger/goshimmer/packages/errors"
    "github.com/iotaledger/goshimmer/packages/events"
    "strconv"
)

// region interfaces ///////////////////////////////////////////////////////////////////////////////////////////////////

type protocolState interface {
    Consume(protocol *protocol, data []byte, offset int, length int) (int, errors.IdentifiableError)
}

// endregion ////////////////////////////////////////////////////////////////////////////////////////////////////////////

// region protocol /////////////////////////////////////////////////////////////////////////////////////////////////////

type protocol struct {
    Events       protocolEvents
    neighbor     *Peer
    currentState protocolState
}

func newProtocol(neighbor *Peer) *protocol {
    protocol := &protocol{
        Events: protocolEvents{
            ReceiveVersion: events.NewEvent(intCaller),
        },
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

// region versionState /////////////////////////////////////////////////////////////////////////////////////////////////

type versionState struct{}

func (state *versionState) Consume(protocol *protocol, data []byte, offset int, length int) (int, errors.IdentifiableError) {
    switch data[offset] {
    case 1:
        protocol.Events.ReceiveVersion.Trigger(1)

        protocol.currentState = newIndentificationStateV1()

        return 1, nil

    default:
        return 1, ErrInvalidStateTransition.Derive("invalid version state transition (" + strconv.Itoa(int(data[offset])) + ")")
    }
}

// endregion ////////////////////////////////////////////////////////////////////////////////////////////////////////////
