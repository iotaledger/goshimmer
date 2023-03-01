package warpsync

import (
	"time"

	"github.com/iotaledger/goshimmer/plugins/config"
)

// ParametersDefinition contains the definition of configuration parameters used by the gossip plugin.
type ParametersDefinition struct {
	// Concurrency defines the amount of slots to attempt to sync at the same time.
	Concurrency int `default:"10" usage:"the amount of slots to attempt to sync at the same time"`
	// BlockBatchSize defines the amount of blocks to send in a single slot blocks response.
	BlockBatchSize int `default:"100" usage:"the amount of blocks to send in a single slot blocks response"`
	// SyncRangeTimeOut defines the time after which a sync range is considered as failed.
	SyncRangeTimeOut time.Duration `default:"5m" usage:"the time after which a sync range is considered as failed"`
}

// Parameters contains the configuration parameters of the gossip plugin.
var Parameters = &ParametersDefinition{}

func init() {
	config.BindParameters(Parameters, "warpsync")
}
