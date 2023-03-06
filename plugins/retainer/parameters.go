package retainer

import "github.com/iotaledger/goshimmer/plugins/config"

// ParametersDefinition contains the definition of the parameters used by the remotelog plugin.
type ParametersDefinition struct {
	Directory        string `default:"retainer" usage:"path to the database directory"`
	PruningThreshold uint32 `default:"8640" usage:"how many confirmed slots should be retained"`
	MaxOpenDBs       int    `default:"10" usage:"maximum number of open database instances"`
	DBGranularity    int64  `default:"10" usage:"how many slots should be contained in a single DB instance"`
}

// Parameters contains the configuration used by the remotelog plugin.
var Parameters = &ParametersDefinition{}

func init() {
	config.BindParameters(Parameters, "retainer")
}
