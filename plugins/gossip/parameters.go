package gossip

import (
	"time"

	"github.com/iotaledger/hive.go/configuration"
)

// ParametersDefinition contains the definition of configuration parameters used by the gossip plugin.
type ParametersDefinition struct {
	// BindAddress defines on which address the gossip service should listen.
	BindAddress string `default:"0.0.0.0:14666" usage:"the bind address for the gossip"`

	// MissingMessageRequestRelayProbability defines the probability of missing message requests being relayed to other neighbors.
	MissingMessageRequestRelayProbability float64 `default:"0.01" usage:"the probability of missing message requests being relayed to other neighbors"`

	MessagesRateLimit messagesLimitParameters
}

type messagesLimitParameters struct {
	Interval time.Duration `default:"1s" usage:"the time interval for which we count the messages rate"`
	Limit    int           `default:"200" usage:"the default limit of messages per interval"`
}

// Parameters contains the configuration parameters of the gossip plugin.
var Parameters = &ParametersDefinition{}

func init() {
	configuration.BindParameters(Parameters, "gossip")
}
