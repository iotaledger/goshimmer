package devnetvm

import (
	"github.com/pkg/errors"
)

// ErrTransactionInvalid is returned if a Transaction or any of its building blocks is considered to be invalid.
var ErrTransactionInvalid = errors.New("transaction invalid")
