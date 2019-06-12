package response

import "github.com/pkg/errors"

var (
	ErrInvalidSignature = errors.New("invalid signature in peering request")
)
