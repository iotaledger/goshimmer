package faucet

import (
	"github.com/iotaledger/hive.go/configuration"
)

// ParametersDefinition contains the definition of configuration parameters used by the faucet plugin.
type ParametersDefinition struct {
	// Seed defines the base58 encoded seed the faucet uses.
	Seed string `usage:"the base58 encoded seed of the faucet, must be defined if this faucet is enabled"`

	// TokensPerRequest defines the amount of tokens the faucet should send for each request.
	TokensPerRequest int `default:"1000000" usage:"the amount of tokens the faucet should send for each request"`

	// MaxTransactionBookedAwaitTimeSeconds defines the time to await for the transaction fulfilling a funding request
	// to become booked in the value layer.
	MaxTransactionBookedAwaitTimeSeconds int `default:"5" usage:"the max amount of time for a funding transaction to become booked in the value layer"`

	// PowDifficulty defines the PoW difficulty for faucet payloads.
	PowDifficulty int `default:"22" usage:"defines the PoW difficulty for faucet payloads"`

	// BlacklistCapacity holds the maximum amount the address blacklist holds.
	// An address for which a funding was done in the past is added to the blacklist and eventually is removed from it.
	BlacklistCapacity int `default:"10000" usage:"holds the maximum amount the address blacklist holds"`

	// PreparedOutputsCount is the number of outputs the faucet prepares for requests.
	PreparedOutputsCount int `default:"126" usage:"number of outputs the faucet prepares"`

	// StartIndex defines from which address index the faucet should start gathering outputs.
	StartIndex int `default:"0" usage:"address index to start faucet with"`
}

// Parameters contains the configuration parameters of the faucet plugin.
var Parameters = &ParametersDefinition{}

func init() {
	configuration.BindParameters(Parameters, "faucet")
}
