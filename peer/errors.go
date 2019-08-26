package peer

import "errors"

var (
	ErrNilInput  = errors.New("nil input")
	ErrMarshal   = errors.New("Marshal failed")
	ErrUnmarshal = errors.New("Unmarshal failed")
)