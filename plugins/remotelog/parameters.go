package remotelog

import "github.com/iotaledger/goshimmer/plugins/config"

// ParametersDefinition contains the definition of the parameters used by the remotelog plugin.
type ParametersDefinition struct {
	// RemoteLog defines the parameters to reach the remote logging server.
	RemoteLog struct {
		// ServerAddress defines the server address that will receive the logs.
		ServerAddress string `default:"metrics-01.devnet.shimmer.iota.cafe:5213" usage:"RemoteLog server address"`
	} `name:"remotelog"`
}

// Parameters contains the configuration used by the remotelog plugin.
var Parameters = &ParametersDefinition{}

func init() {
	config.BindParameters(Parameters, "logger")
}
