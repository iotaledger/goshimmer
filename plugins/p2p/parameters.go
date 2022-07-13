package p2p

import (
	"github.com/iotaledger/hive.go/configuration"
)

// ParametersDefinition contains the definition of configuration parameters used by the gossip plugin.
type ParametersDefinition struct {
	// BindAddress defines on which address the gossip service should listen.
	BindAddress string `default:"0.0.0.0:14666" usage:"the bind address for p2p connections"`
}

// Parameters contains the configuration parameters of the gossip plugin.
var Parameters = &ParametersDefinition{}

func init() {
	configuration.BindParameters(Parameters, "p2p")
}
