package scheduler

import (
	"net/http"

	"github.com/iotaledger/hive.go/core/autopeering/peer"
	"github.com/iotaledger/hive.go/core/node"
	"github.com/labstack/echo"
	"go.uber.org/dig"

	"github.com/iotaledger/goshimmer/packages/app/jsonmodels"
	"github.com/iotaledger/goshimmer/packages/core/tangleold"
)

// PluginName is the name of the web API info endpoint plugin.
const PluginName = "WebAPISchedulerEndpoint"

type dependencies struct {
	dig.In

	Server *echo.Echo
	Local  *peer.Local
	Tangle *tangleold.Tangle
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
	nodeQueueSizes := make(map[string]int)
	for nodeID, size := range deps.Tangle.Scheduler.NodeQueueSizes() {
		nodeQueueSizes[nodeID.String()] = size
	}

	deficit, _ := deps.Tangle.Scheduler.GetDeficit(deps.Local.ID()).Float64()
	return c.JSON(http.StatusOK, jsonmodels.Scheduler{
		Running:           deps.Tangle.Scheduler.Running(),
		Rate:              deps.Tangle.Scheduler.Rate().String(),
		MaxBufferSize:     deps.Tangle.Scheduler.MaxBufferSize(),
		CurrentBufferSize: deps.Tangle.Scheduler.BufferSize(),
		NodeQueueSizes:    nodeQueueSizes,
		Deficit:           deficit,
	})
}
