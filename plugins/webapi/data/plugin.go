package data

import (
	"fmt"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"go.uber.org/dig"

	"github.com/iotaledger/goshimmer/packages/app/blockissuer"
	"github.com/iotaledger/goshimmer/packages/app/jsonmodels"
	"github.com/iotaledger/goshimmer/packages/node"
	"github.com/iotaledger/goshimmer/packages/protocol/models/payload"
	"github.com/iotaledger/hive.go/logger"
)

const maxIssuedAwaitTime = 5 * time.Second

// PluginName is the name of the web API data endpoint plugin.
const PluginName = "WebAPIDataEndpoint"

type dependencies struct {
	dig.In

	Server      *echo.Echo
	BlockIssuer *blockissuer.BlockIssuer
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

// broadcastData creates a block of the given payload and
// broadcasts it to the node's neighbors. It returns the block ID if successful.
func broadcastData(c echo.Context) error {
	var request jsonmodels.DataRequest
	if err := c.Bind(&request); err != nil {
		log.Info(err.Error())
		return c.JSON(http.StatusBadRequest, jsonmodels.DataResponse{Error: err.Error()})
	}

	if len(request.Data) == 0 {
		return c.JSON(http.StatusBadRequest, jsonmodels.DataResponse{Error: "no data provided"})
	}

	maxAwaitTime := maxIssuedAwaitTime
	if request.MaxEstimate > 0 {
		maxAwaitTime = time.Duration(request.MaxEstimate) * time.Millisecond
		if deps.BlockIssuer.Estimate().Milliseconds() > request.MaxEstimate {
			return c.JSON(http.StatusBadRequest, jsonmodels.DataResponse{
				Error: fmt.Sprintf("issuance estimate greater than %d ms", request.MaxEstimate),
			})
		}
	}

	constructedBlock, err := deps.BlockIssuer.CreateBlock(payload.NewGenericDataPayload(request.Data))
	if err != nil {
		return c.JSON(http.StatusInternalServerError, jsonmodels.DataResponse{Error: err.Error()})
	}

	// await BlockScheduled event to be triggered.
	err = deps.BlockIssuer.IssueBlockAndAwaitBlockToBeScheduled(constructedBlock, maxAwaitTime)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, jsonmodels.DataResponse{Error: err.Error()})
	}

	return c.JSON(http.StatusOK, jsonmodels.DataResponse{ID: constructedBlock.ID().Base58()})
}
