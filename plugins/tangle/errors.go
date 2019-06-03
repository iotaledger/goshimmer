package tangle

import "github.com/iotaledger/goshimmer/packages/errors"

var (
    ErrDatabaseError = errors.Wrap(errors.New("database error"), "failed to access the database")
    ErrUnmarshalFailed = errors.Wrap(errors.New("unmarshall failed"), "input data is corrupted")
    ErrMarshallFailed = errors.Wrap(errors.New("marshal failed"), "the source object contains invalid values")
)
