package ping

import "github.com/pkg/errors"

var (
    ErrInvalidSignature          = errors.New("invalid signature in ping")
    ErrMalformedPing             = errors.New("malformed ping")
)
