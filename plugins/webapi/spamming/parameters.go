package spamming

import "github.com/iotaledger/goshimmer/plugins/config"

// ParametersDefinition contains the definition of the parameters used by the profiling plugin.
type ParametersDefinition struct {
	// Endpoints defines the endpoints for spamming.
	Endpoints []string `default:"http://127.0.0.1:8080" usage:"endpoints for spamming"`
}

// Parameters contains the configuration used by the profiling plugin.
var Parameters = &ParametersDefinition{}

func init() {
	config.BindParameters(Parameters, "webapispamming")
}
