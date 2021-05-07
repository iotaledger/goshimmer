package activity

import "github.com/iotaledger/hive.go/configuration"

// Parameters contains the configuration parameters used by the message layer.
var Parameters = struct {
	// BroadcastIntervalSec is the interval in seconds at which the node broadcasts its activity message.
	BroadcastIntervalSec int `default:"3" usage:"the interval at which the node will broadcast its activity message"`
}{}

func init() {
	configuration.BindParameters(&Parameters, "activity")
}
