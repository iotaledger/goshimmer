package txvm

import (
	"github.com/cockroachdb/errors"
)

var (
	// ErrTransactionInvalid is returned if a Transaction or any of its building blocks is considered to be invalid.
	ErrTransactionInvalid = errors.New("transaction invalid")
)
