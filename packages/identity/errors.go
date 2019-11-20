package identity

import "errors"

var (
	ErrInvalidDataLen   = errors.New("identity: invalid input data length")
	ErrInvalidSignature = errors.New("identity: invalid signature")
)
