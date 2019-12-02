package gossip

import "errors"

var (
	errClosed    = errors.New("socket closed")
	errSender    = errors.New("could not match sender")
	errRecipient = errors.New("could not match recipient")
	errSignature = errors.New("could not verify signature")
	errVersion   = errors.New("protocol version not supported")
)
