package data

import (
	"net/http"
	"time"

	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
	"github.com/labstack/echo"
	"go.uber.org/dig"

	"github.com/iotaledger/goshimmer/packages/jsonmodels"
	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/goshimmer/packages/tangle/payload"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
)

const maxIssuedAwaitTime = 5 * time.Second

// PluginName is the name of the web API data endpoint plugin.
const PluginName = "WebAPIDataEndpoint"

type dependencies struct {
	dig.In

	Server *echo.Echo
	Tangle *tangle.Tangle
}

var (
	// Plugin is the plugin instance of the web API data endpoint plugin.
	Plugin *node.Plugin
	deps   = new(dependencies)
	log    *logger.Logger
)

func init() {
	Plugin = node.NewPlugin(PluginName, deps, node.Enabled, configure)
}

func configure(_ *node.Plugin) {
	log = logger.NewLogger(PluginName)
	deps.Server.POST("data", broadcastData)
}

// broadcastData creates a message of the given payload and
// broadcasts it to the node's neighbors. It returns the message ID if successful.
func broadcastData(c echo.Context) error {
	var request jsonmodels.DataRequest
	if err := c.Bind(&request); err != nil {
		log.Info(err.Error())
		return c.JSON(http.StatusBadRequest, jsonmodels.DataResponse{Error: err.Error()})
	}

	if len(request.Data) == 0 {
		return c.JSON(http.StatusBadRequest, jsonmodels.DataResponse{Error: "no data provided"})
	}

	issueData := func() (*tangle.Message, error) {
		return deps.Tangle.IssuePayload(payload.NewGenericDataPayload(request.Data))
	}

	// await MessageScheduled event to be triggered.
	msg, err := messagelayer.AwaitMessageToBeIssued(issueData, deps.Tangle.Options.Identity.PublicKey(), maxIssuedAwaitTime)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, jsonmodels.DataResponse{Error: err.Error()})
	}

	return c.JSON(http.StatusOK, jsonmodels.DataResponse{ID: msg.ID().Base58()})
}
