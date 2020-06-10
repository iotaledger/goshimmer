package healthz

import (
	"net/http"

	"github.com/iotaledger/goshimmer/plugins/gossip"
	"github.com/iotaledger/goshimmer/plugins/sync"
	"github.com/iotaledger/goshimmer/plugins/webapi"
	"github.com/iotaledger/hive.go/node"
	"github.com/labstack/echo"
)

// PluginName is the name of the web API healthz endpoint plugin.
const PluginName = "WebAPI healthz Endpoint"

// Plugin is the plugin instance of the web API info endpoint plugin.
var Plugin = node.NewPlugin(PluginName, node.Enabled, configure)

func configure(_ *node.Plugin) {
	webapi.Server.GET("healthz", getHealthz)
}

func getHealthz(c echo.Context) error {
	if !IsNodeHealthy() {
		return c.NoContent(http.StatusServiceUnavailable)
	}

	return c.NoContent(http.StatusOK)
}

// IsNodeHealthy returns whether the node is synced, has active neighbors.
func IsNodeHealthy() bool {
	// Synced
	if !sync.Synced() {
		return false
	}

	// Has connected neighbors
	if len(gossip.Manager().AllNeighbors()) == 0 {
		return false
	}

	return true
}
