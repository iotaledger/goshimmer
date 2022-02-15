package evilspammer

import "github.com/cockroachdb/errors"

var (
	ErrFailPostTransaction = errors.New("failed to post transaction")
	ErrTransactionIsNil    = errors.New("provided transaction is nil")
)
