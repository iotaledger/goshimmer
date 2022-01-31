package dashboard

import "github.com/iotaledger/hive.go/configuration"

// ParametersDefinition contains the definition of the parameters used by the dasbhoard plugin.
type ParametersDefinition struct {
	// BindAddress defines the analysis dashboard binding address.
	BindAddress string `default:"0.0.0.0:8000" usage:"the bind address of the analysis dashboard"`
	// Dev defines the analysis dashboard dev mode.
	Dev bool `default:"false" usage:"whether the analysis dashboard runs in dev mode"`
	// BasicAuthEnabled defines  the analysis dashboard basic auth enabler.
	BasicAuthEnabled bool `default:"false" usage:"whether to enable HTTP basic auth"`
	// BasicAuthUsername defines the analysis dashboard basic auth username.
	BasicAuthUsername string `default:"goshimmer" usage:"HTTP basic auth username"`
	// BasicAuthPassword defines the analysis dashboard basic auth password.
	BasicAuthPassword string `default:"goshimmer" usage:"HTTP basic auth password"`
	// ManaDashboardAddress defines the mana dashboard address to stream mana info from.
	ManaDashboardAddress string `default:"http://127.0.0.1:8081" usage:"dashboard host address"`
}

// Parameters contains the configuration parameters of the dashboard plugin.
var Parameters = &ParametersDefinition{}

func init() {
	configuration.BindParameters(Parameters, "analysis.dashboard")
}
