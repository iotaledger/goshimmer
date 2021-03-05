package faucet

import "errors"

var (
	// ErrAddressIsBlacklisted is returned if a funding can't be processed since the address is blacklisted.
	ErrAddressIsBlacklisted = errors.New("can't fund address as it is blacklisted")
	// ErrPrepareFaucet is returned if the faucet cannot prepare outputs.
	ErrPrepareFaucet = errors.New("can't prepare faucet outputs")
)
