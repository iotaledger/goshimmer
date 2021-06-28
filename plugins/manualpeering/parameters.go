package manualpeering

import "github.com/iotaledger/hive.go/configuration"

// ParametersDefinition contains the definition of the parameters used by the manualpeering plugin.
type ParametersDefinition struct {
	// KnownPeers defines the list of peers to be used as known peers
	KnownPeers string `default:"" usage:"list of peers that will be used as known peers"`
}

// Parameters contains the configuration used by the manualpeering plugin
var Parameters = ParametersDefinition{}

func init() {
	configuration.BindParameters(&Parameters, "manaeventlogger")
}
