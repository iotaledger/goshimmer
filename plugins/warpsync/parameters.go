package warpsync

import (
	"github.com/iotaledger/hive.go/configuration"
)

// ParametersDefinition contains the definition of configuration parameters used by the gossip plugin.
type ParametersDefinition struct {
	// Concurrency defines the amount of epochs to attempt to sync at the same time.
	Concurrency int `default:"10" usage:"the amount of epochs to attempt to sync at the same time"`
	// BlockBatchSize defines the amount of blocks to send in a single epoch blocks response"`
	BlockBatchSize int `default:"100" usage:"the amount of blocks to send in a single epoch blocks response"`
}

// Parameters contains the configuration parameters of the gossip plugin.
var Parameters = &ParametersDefinition{}

func init() {
	configuration.BindParameters(Parameters, "warpsync")
}
