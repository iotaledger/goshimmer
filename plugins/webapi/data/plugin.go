package data

import (
	"net/http"
	"sync"

	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
	"github.com/labstack/echo"

	"github.com/iotaledger/goshimmer/packages/tangle/payload"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
	"github.com/iotaledger/goshimmer/plugins/webapi"
	"github.com/iotaledger/goshimmer/plugins/webapi/jsonmodels"
)

// PluginName is the name of the web API data endpoint plugin.
const PluginName = "WebAPI data Endpoint"

var (
	// plugin is the plugin instance of the web API data endpoint plugin.
	plugin *node.Plugin
	once   sync.Once
	log    *logger.Logger
)

// Plugin gets the plugin instance.
func Plugin() *node.Plugin {
	once.Do(func() {
		plugin = node.NewPlugin(PluginName, node.Enabled, configure)
	})
	return plugin
}

func configure(plugin *node.Plugin) {
	log = logger.NewLogger(PluginName)
	webapi.Server().POST("data", broadcastData)
}

// broadcastData creates a message of the given payload and
// broadcasts it to the node's neighbors. It returns the message ID if successful.
func broadcastData(c echo.Context) error {
	var request jsonmodels.DataRequest
	if err := c.Bind(&request); err != nil {
		log.Info(err.Error())
		return c.JSON(http.StatusBadRequest, jsonmodels.DataResponse{Error: err.Error()})
	}

	msg, err := messagelayer.Tangle().IssuePayload(payload.NewGenericDataPayload(request.Data))
	if err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels.DataResponse{Error: err.Error()})
	}
	return c.JSON(http.StatusOK, jsonmodels.DataResponse{ID: msg.ID().Base58()})
}
