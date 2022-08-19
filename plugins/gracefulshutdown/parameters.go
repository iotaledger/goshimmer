package gracefulshutdown

import (
	"time"

	"github.com/iotaledger/goshimmer/plugins/config"
)

// ParametersDefinition contains the definition of configuration parameters used by the graceful shutdown plugin.
type ParametersDefinition struct {
	// WaitToKillTime is the maximum amount of time to wait for background processes to terminate.
	WaitToKillTime time.Duration `default:"120s" usage:"the maximum amount of time to wait for background processes to terminate"`
}

// Parameters contains the configuration parameters of the graceful shutdown plugin.
var Parameters = &ParametersDefinition{}

func init() {
	config.BindParameters(Parameters, "gracefulShutdown")
}
