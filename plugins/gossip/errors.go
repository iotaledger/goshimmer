package gossip

import "github.com/iotaledger/goshimmer/packages/errors"

var (
    ErrConnectionFailed = errors.Wrap(errors.New("connection error"), "could not connect to neighbor")
    ErrInvalidAuthenticationMessage = errors.Wrap(errors.New("protocol error"), "invalid authentication message")
    ErrInvalidIdentity = errors.Wrap(errors.New("protocol error"), "invalid identity message")
    ErrInvalidStateTransition = errors.New("protocol error: invalid state transition message")
    ErrSendFailed = errors.Wrap(errors.New("protocol error"), "failed to send message")
    ErrInvalidSendParam = errors.New("invalid parameter passed to send")
)
