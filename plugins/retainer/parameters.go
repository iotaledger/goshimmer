package retainer

import "github.com/iotaledger/goshimmer/plugins/config"

// ParametersDefinition contains the definition of the parameters used by the remotelog plugin.
type ParametersDefinition struct {
	DBPath         string `default:"retainer" usage:"path where retainer database is stored"`
	DBPruningDelay uint32 `default:"8640" usage:"how many confirmed epochs should be retained"`
}

// Parameters contains the configuration used by the remotelog plugin.
var Parameters = &ParametersDefinition{}

func init() {
	config.BindParameters(Parameters, "retainer")
}
