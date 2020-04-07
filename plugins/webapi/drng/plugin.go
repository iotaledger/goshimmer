package drng

import (
	"github.com/iotaledger/goshimmer/plugins/webapi"
	"github.com/iotaledger/goshimmer/plugins/webapi/drng/collectiveBeacon"
	"github.com/iotaledger/goshimmer/plugins/webapi/drng/info/committee"
	"github.com/iotaledger/goshimmer/plugins/webapi/drng/info/randomness"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
)

var PLUGIN = node.NewPlugin("WebAPI dRNG Endpoint", node.Enabled, configure)
var log *logger.Logger

func configure(plugin *node.Plugin) {
	log = logger.NewLogger("API-dRNG")
	webapi.Server.POST("drng/collectiveBeacon", collectiveBeacon.Handler)

	webapi.Server.GET("drng/info/committee", committee.Handler)
	webapi.Server.GET("drng/info/randomness", randomness.Handler)
}
