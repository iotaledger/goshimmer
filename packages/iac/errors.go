package iac

import "github.com/iotaledger/goshimmer/packages/errors"

var (
	ErrConversionFailed = errors.New("conversion between IAC and internal OLC format failed")
	ErrDecodeFailed     = errors.Wrap(errors.New("decoding error"), "failed to decode the IAC")
)
