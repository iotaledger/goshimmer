package gossip

import "github.com/iotaledger/goshimmer/packages/errors"

var (
    ErrInvalidAuthenticationMessage = errors.Wrap(errors.New("protocol error"), "invalid authentication message")
    ErrInvalidStateTransition = errors.New("protocol error: invalid state transition message")
)
