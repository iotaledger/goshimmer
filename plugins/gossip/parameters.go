package gossip

import (
	"github.com/iotaledger/hive.go/configuration"
)

// Parameters contains the configuration parameters used by the gossip plugin.
var Parameters = struct {
	// NetworkVersion defines the config flag of the network version.
	Port int `default:"14666" usage:"tcp port for gossip connection"`
}{}

func init() {
	configuration.BindParameters(&Parameters, "gossip")
}
