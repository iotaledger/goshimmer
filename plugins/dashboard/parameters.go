package dashboard

import (
	"github.com/iotaledger/hive.go/configuration"
)

// ParametersDefinition contains the definition of configuration parameters used by the dashboard plugin.
type ParametersDefinition struct {
	// BindAddress defines the config flag of the dashboard binding address.
	BindAddress string `default:"127.0.0.1:8081" usage:"the bind address of the dashboard"`

	// Dev defines the config flag of the  dashboard dev mode.
	Dev bool `default:"false" usage:"whether the dashboard runs in dev mode"`

	// DevDashboardAddress defines the address of dashboard running in development mode.
	DevDashboardAddress string `default:"127.0.0.1:9090" usage:"address of the dashboard when running in dev mode, e.g. with yarn start"`

	BasicAuth struct {
		// Enabled defines the config flag of the dashboard basic auth enabler.
		Enabled bool `default:"false" usage:"whether to enable HTTP basic auth"`

		// Username defines the config flag of the dashboard basic auth username.
		Username string `default:"goshimmer" usage:"HTTP basic auth username"`

		// Password defines the config flag of the dashboard basic auth password.
		Password string `default:"goshimmer" usage:"HTTP basic auth password"`
	}

	// Conflicts defines the config flag for the configs tab of the dashboard.
	Conflicts struct {
		// MaxCount defines the max number of conflicts stored on the dashboard.
		MaxCount int `default:"100" usage:"max number of conflicts stored on the dashboard"`
	}
}

// Parameters contains the configuration parameters of the dashboard plugin.
var Parameters = &ParametersDefinition{}

func init() {
	configuration.BindParameters(Parameters, "dashboard")
}
