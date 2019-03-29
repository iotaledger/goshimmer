package protocol

import "github.com/pkg/errors"

var (
    ErrPublicSaltExpired         = errors.New("expired public salt in peering request")
    ErrPublicSaltInvalidLifetime = errors.New("invalid public salt lifetime")
    ErrInvalidSignature          = errors.New("invalid signature in peering request")
    ErrMalformedPeeringRequest   = errors.New("malformed peering request")
)
