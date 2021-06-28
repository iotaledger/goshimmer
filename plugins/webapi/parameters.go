package webapi

import (
	"github.com/iotaledger/hive.go/configuration"
)

// ParametersDefinition contains the definition of the parameters used by the webapi plugin.
type ParametersDefinition struct {
	// BindAddress defines the bind address for the web API.
	BindAddress string `default:"127.0.0.1:8080" usage:"the bind address for the web API"`
	// BasicAuthEnabled defines whether basic HTTP authentication is required to access the API.
	BasicAuthEnabled bool `default:"false" usage:"whether to enable HTTP basic auth"`
	// BasicAuthUsername defines the user used by the basic HTTP authentication.
	BasicAuthUsername string `default:"goshimmer" usage:"HTTP basic auth username"`
	// BasicAuthPassword defines the password used by the basic HTTP authentication.
	BasicAuthPassword string `default:"goshimmer" usage:"HTTP basic auth password"`
}

// Parameters contains the configuration used by the webapi plugin
var Parameters = ParametersDefinition{}

func init() {
	configuration.BindParameters(&Parameters, "webapi")
}
