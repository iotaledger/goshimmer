package drop

import "github.com/pkg/errors"

var (
	ErrInvalidSignature     = errors.New("invalid signature in drop message")
	ErrMalformedDropMessage = errors.New("malformed drop message")
)
