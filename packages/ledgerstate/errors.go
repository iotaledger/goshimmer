package ledgerstate

import (
	"errors"
)

var (
	// ErrTransactionInvalid is returned if a Transaction or any of its building blocks is considered to be invalid.
	ErrTransactionInvalid = errors.New("transaction invalid")

	// ErrTransactionNotSolid is returned if a Transaction is processed whose Inputs are not known.
	ErrTransactionNotSolid = errors.New("transaction not solid")

	// ErrInvalidStateTransition is returned if there is an invalid state transition in the ledger state.
	ErrInvalidStateTransition = errors.New("invalid state transition")
)
