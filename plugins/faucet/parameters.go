package faucet

import (
	"time"

	"github.com/iotaledger/goshimmer/plugins/config"
)

// ParametersDefinition contains the definition of configuration parameters used by the faucet plugin.
type ParametersDefinition struct {
	// Seed defines the base58 encoded seed the faucet uses.
	Seed string `usage:"the base58 encoded seed of the faucet, must be defined if this faucet is enabled"`

	// FaucetWalletFile defines the file name of faucet wallet states.
	FaucetWalletFile string `default:"faucet.wallet" usage:"the states that is preserved for faucet wallet"`

	// TokensPerRequest defines the amount of tokens the faucet should send for each request.
	TokensPerRequest int `default:"1000000" usage:"the amount of tokens the faucet should send for each request"`

	// MaxTransactionBookedAwaitTime defines the time to await for the transaction fulfilling a funding request
	// to become booked in the value layer.
	MaxTransactionBookedAwaitTime time.Duration `default:"60s" usage:"the max amount of time for a funding transaction to become booked in the value layer"`

	// PowDifficulty defines the PoW difficulty for faucet payloads.
	PowDifficulty int `default:"22" usage:"defines the PoW difficulty for faucet payloads"`

	// BlacklistCapacity holds the maximum amount the address blacklist holds.
	// An address for which a funding was done in the past is added to the blacklist and eventually is removed from it.
	BlacklistCapacity int `default:"10000" usage:"holds the maximum amount the address blacklist holds"`

	// SupplyOutputsCount is the number of supply outputs, and splitting transactions accordingly, the faucet prepares.
	SupplyOutputsCount int `default:"100" usage:"the number of supply outputs, and splitting transactions accordingly, the faucet prepares."`

	// GenesisTokenAmount is the total supply.
	GenesisTokenAmount uint64 `default:"1000000000000000" usage:"GenesisTokenAmount is the total supply."`
}

// Parameters contains the configuration parameters of the faucet plugin.
var Parameters = &ParametersDefinition{}

func init() {
	config.BindParameters(Parameters, "faucet")
}
