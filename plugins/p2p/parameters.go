package p2p

import "github.com/iotaledger/goshimmer/plugins/config"

// ParametersDefinition contains the definition of configuration parameters used by the p2p plugin.
type ParametersDefinition struct {
	// BindAddress defines on which address the p2p service should listen.
	BindAddress string `default:"0.0.0.0:14666" usage:"the bind address for p2p connections"`
}

// Parameters contains the configuration parameters of the p2p plugin.
var Parameters = &ParametersDefinition{}

func init() {
	config.BindParameters(Parameters, "p2p")
}
