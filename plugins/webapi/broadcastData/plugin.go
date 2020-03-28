package broadcastData

import (
	"net/http"

	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/payload"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
	"github.com/iotaledger/goshimmer/plugins/webapi"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
	"github.com/labstack/echo"
)

var PLUGIN = node.NewPlugin("WebAPI broadcastData Endpoint", node.Enabled, configure)
var log *logger.Logger

func configure(plugin *node.Plugin) {
	log = logger.NewLogger("API-broadcastData")
	webapi.Server.POST("broadcastData", broadcastData)
}

// broadcastData creates a data (0-value) transaction given an input of bytes and
// broadcasts it to the node's neighbors. It returns the transaction hash if successful.
func broadcastData(c echo.Context) error {
	var request Request
	if err := c.Bind(&request); err != nil {
		log.Info(err.Error())
		return c.JSON(http.StatusBadRequest, Response{Error: err.Error()})
	}

	tx := messagelayer.MessageFactory.BuildMessage(payload.NewData([]byte(request.Data)))

	return c.JSON(http.StatusOK, Response{Hash: tx.GetId().String()})
}

type Response struct {
	Hash  string `json:"hash,omitempty"`
	Error string `json:"error,omitempty"`
}

type Request struct {
	Address string `json:"address"`
	Data    string `json:"data"`
}
