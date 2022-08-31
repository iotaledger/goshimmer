package manainitializer

import "github.com/iotaledger/goshimmer/plugins/config"

// ParametersDefinition contains the definition of the parameters used by the manainitializer plugin.
type ParametersDefinition struct {
	// FaucetAPI defines API address of the faucet node.
	FaucetAPI string `default:"http://faucet-01.devnet.shimmer.iota.cafe:8080" usage:"API address of the faucet node"`
	// Address defines address to request the funds for. By default, the address of the node is used.
	Address string `usage:"address to request the funds for. By default, the address of the node is used"`
}

// Parameters contains the configuration used by the manainitializer plugin.
var Parameters = &ParametersDefinition{}

func init() {
	config.BindParameters(Parameters, "manaInitializer")
}
