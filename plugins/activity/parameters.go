package activity

import (
	"time"

	"github.com/iotaledger/hive.go/configuration"
)

// ParametersDefinition contains the definition of configuration parameters used by the activity plugin.
type ParametersDefinition struct {
	// BroadcastInterval is the interval at which the node broadcasts its activity message.
	BroadcastInterval time.Duration `default:"2s" usage:"the interval at which the node will broadcast its activity message"`
	// ParentsCount is the number of parents that node will choose for its activity messages.
	ParentsCount int `default:"8" usage:"the number of parents that node will choose for its activity messages"`
	// DelayOffset is the maximum for random initial time delay before sending the activity message.
	DelayOffset time.Duration `default:"10s" usage:"the maximum for random initial time delay before sending the activity message"`
}

// Parameters contains the configuration parameters of the activity plugin.
var Parameters = &ParametersDefinition{}

func init() {
	configuration.BindParameters(Parameters, "activity")
}
