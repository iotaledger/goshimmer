package mempool

import (
	"github.com/pkg/errors"
)

var (
	// ErrTransactionInvalid is returned if a Transaction is found to be invalid.
	ErrTransactionInvalid = errors.New("transaction invalid")

	// ErrTransactionUnsolid is returned if a Transaction consumes unsolid Outputs..
	ErrTransactionUnsolid = errors.New("transaction unsolid")
)
