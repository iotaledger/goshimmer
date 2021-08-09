package faucet

import (
	"github.com/cockroachdb/errors"
)

var (
	// ErrNotEnoughFundingOutputs if there are not enough funding outputs in the faucet.
	ErrNotEnoughFundingOutputs = errors.New("not enough funding outputs to complete the request")
	// ErrMissingRemainderOutput is returned if the remainder output can not be found.
	ErrMissingRemainderOutput = errors.New("can't find faucet remainder output")
	// ErrNotEnoughFunds is returned when not enough funds are left in the faucet.
	ErrNotEnoughFunds = errors.New("not enough funds in the faucet")
	// ErrConfirmationTimeoutExpired is returned when a faucet transaction was not confirmed in expected time.
	ErrConfirmationTimeoutExpired = errors.New("tx confirmation time expired")
)
