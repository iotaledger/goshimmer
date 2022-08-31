package faucet

import (
	"time"

	"github.com/iotaledger/goshimmer/plugins/config"
)

// ParametersDefinition contains the definition of configuration parameters used by the faucet plugin.
type ParametersDefinition struct {
	// Seed defines the base58 encoded seed the faucet uses.
	Seed string `usage:"the base58 encoded seed of the faucet, must be defined if this faucet is enabled"`

	// TokensPerRequest defines the amount of tokens the faucet should send for each request.
	TokensPerRequest int `default:"1000000" usage:"the amount of tokens the faucet should send for each request"`

	// MaxTransactionBookedAwaitTime defines the time to await for the transaction fulfilling a funding request
	// to become booked in the value layer.
	MaxTransactionBookedAwaitTime time.Duration `default:"60s" usage:"the max amount of time for a funding transaction to become booked in the value layer"`

	// PowDifficulty defines the PoW difficulty for faucet payloads.
	PowDifficulty int `default:"22" usage:"defines the PoW difficulty for faucet payloads"`

	// MaxWaitAttempts defines the maximum time to wait for a transaction to be accepted.
	MaxAwait time.Duration `default:"60s" usage:"the maximum time to wait for a transaction to be accepted"`
}

// Parameters contains the configuration parameters of the faucet plugin.
var Parameters = &ParametersDefinition{}

func init() {
	config.BindParameters(Parameters, "faucet")
}
