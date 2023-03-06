package scheduler

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"go.uber.org/dig"

	"github.com/iotaledger/goshimmer/packages/app/jsonmodels"
	"github.com/iotaledger/goshimmer/packages/node"
	"github.com/iotaledger/goshimmer/packages/protocol"
	"github.com/iotaledger/hive.go/autopeering/peer"
)

// PluginName is the name of the web API info endpoint plugin.
const PluginName = "WebAPISchedulerEndpoint"

type dependencies struct {
	dig.In

	Server   *echo.Echo
	Local    *peer.Local
	Protocol *protocol.Protocol
}

var (
	// Plugin is the plugin instance of the web API info endpoint plugin.
	Plugin *node.Plugin
	deps   = new(dependencies)
)

func init() {
	Plugin = node.NewPlugin(PluginName, deps, node.Enabled, configure)
}

func configure(_ *node.Plugin) {
	deps.Server.GET("scheduler", getSchedulerInfo)
}

func getSchedulerInfo(c echo.Context) error {
	scheduler := deps.Protocol.CongestionControl.Scheduler()
	nodeQueueSizes := make(map[string]int)
	for nodeID, size := range scheduler.IssuerQueueSizes() {
		nodeQueueSizes[nodeID.String()] = size
	}

	deficit, _ := scheduler.Deficit(deps.Local.ID()).Float64()
	return c.JSON(http.StatusOK, jsonmodels.Scheduler{
		Running:           scheduler.IsRunning(),
		Rate:              scheduler.Rate().String(),
		MaxBufferSize:     scheduler.MaxBufferSize(),
		CurrentBufferSize: scheduler.BufferSize(),
		NodeQueueSizes:    nodeQueueSizes,
		Deficit:           deficit,
	})
}
