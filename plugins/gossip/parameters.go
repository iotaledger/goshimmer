package gossip

import (
	"github.com/iotaledger/hive.go/configuration"
)

// ParametersDefinition contains the definition of configuration parameters used by the gossip plugin.
type ParametersDefinition struct {
	// BindAddress defines on which address the gossip service should listen.
	BindAddress string `default:"0.0.0.0:14666" usage:"the bind address for the gossip"`
}

// Parameters contains the configuration parameters of the gossip plugin.
var Parameters = &ParametersDefinition{}

func init() {
	configuration.BindParameters(Parameters, "gossip")
}
