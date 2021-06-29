package remotelog

import (
	"github.com/iotaledger/hive.go/configuration"
)

// ParametersDefinition contains the definition of the parameters used by the remotelog plugin.
type ParametersDefinition struct {
	// ServerAddress defines the server address that will receive the logs.
	ServerAddress string `default:"ressims.iota.cafe:5213" usage:"RemoteLog server address"`
	// DisableEvents defines whether to disable logger events.
	DisableEvents bool `default:"false" usage:"disable events capturing"`
}

// Parameters contains the configuration used by the remotelog plugin.
var Parameters = ParametersDefinition{}

func init() {
	configuration.BindParameters(&Parameters, "remotelog")
}
