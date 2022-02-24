package evilspammer

import "github.com/cockroachdb/errors"

var (
	ErrFailPostTransaction      = errors.New("failed to post transaction")
	ErrFailSendDataMessage      = errors.New("failed to send a data message")
	ErrTransactionIsNil         = errors.New("provided transaction is nil")
	ErrFailToPrepareTransaction = errors.New("failed to get next spam transaction")
)
