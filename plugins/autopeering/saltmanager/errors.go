package saltmanager

import "github.com/pkg/errors"

var (
    ErrPublicSaltExpired         = errors.New("expired public salt in ping")
    ErrPublicSaltInvalidLifetime = errors.New("invalid public salt lifetime in ping")
)
