package activity

import "github.com/iotaledger/hive.go/configuration"

// Parameters contains the configuration parameters used by the message layer.
var Parameters = struct {
	// BroadcastIntervalSec is the interval in seconds at which the node broadcasts its activity message.
	BroadcastIntervalSec int `default:"3" usage:"the interval at which the node will broadcast its activity message"`
	// ParentsCount is the number of parents that node will choose for its activity messages.
	ParentsCount int `default:"8" usage:"the number of parents that node will choose for its activity messages"`
	// DelayOffset is the maximum for random initial time delay before sending the activity message.
	DelayOffset int `default:"10" usage:"the maximum for random initial time delay before sending the activity message"`
}{}

func init() {
	configuration.BindParameters(&Parameters, "activity")
}
