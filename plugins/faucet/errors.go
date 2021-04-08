package faucet

import (
	"golang.org/x/xerrors"
)

var (
	// ErrNotEnoughFundingOutputs if there are not enough funding outputs in the faucet.
	ErrNotEnoughFundingOutputs = xerrors.New("not enough funding outputs to complete the request")
	// ErrMissingRemainderOutput is returned if the remainder output can not be found.
	ErrMissingRemainderOutput = xerrors.New("can't find faucet remainder output")
	// ErrNotEnoughFunds is returned when not enough funds are left in the faucet.
	ErrNotEnoughFunds = xerrors.New("not enough funds in the faucet")
	// ErrConfirmationTimeoutExpired is returned when a faucet transaction was not confirmed in expected time.
	ErrConfirmationTimeoutExpired = xerrors.New("tx confirmation time expired")
)
