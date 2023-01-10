package activity

import (
	"time"

	"github.com/iotaledger/goshimmer/plugins/config"
)

// ParametersDefinition contains the definition of configuration parameters used by the activity plugin.
type ParametersDefinition struct {
	// BroadcastInterval is the interval at which the node broadcasts its activity block.
	BroadcastInterval time.Duration `default:"2s" usage:"the interval at which the node will broadcast its activity block"`
	// ParentsCount is the number of parents that node will choose for its activity blocks.
	ParentsCount int `default:"8" usage:"the number of parents that node will choose for its activity blocks"`
	// DelayOffset is the maximum for random initial time delay before sending the activity block.
	DelayOffset time.Duration `default:"1ms" usage:"the maximum for random initial time delay before sending the activity block"`
}

// Parameters contains the configuration parameters of the activity plugin.
var Parameters = &ParametersDefinition{}

func init() {
	config.BindParameters(Parameters, "activity")
}
