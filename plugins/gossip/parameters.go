package gossip

import (
	"time"

	"github.com/iotaledger/goshimmer/plugins/config"
)

// ParametersDefinition contains the definition of configuration parameters used by the gossip plugin.
type ParametersDefinition struct {
	// MissingBlockRequestRelayProbability defines the probability of missing block requests being relayed to other neighbors.
	MissingBlockRequestRelayProbability float64 `default:"0.01" usage:"the probability of missing block requests being relayed to other neighbors"`

	BlocksRateLimit        blocksLimitParameters
	BlockRequestsRateLimit blockRequestsLimitParameters
}

type blocksLimitParameters struct {
	Interval time.Duration `default:"10s" usage:"the time interval for which we count the blocks rate"`
	Limit    int           `default:"50000" usage:"the base limit of blocks per interval"`
}

type blockRequestsLimitParameters struct {
	Interval time.Duration `default:"10s" usage:"the time interval for which we count the block requests rate"`
	Limit    int           `default:"50000" usage:"the base limit of block requests per interval"`
}

// Parameters contains the configuration parameters of the gossip plugin.
var Parameters = &ParametersDefinition{}

func init() {
	config.BindParameters(Parameters, "gossip")
}
