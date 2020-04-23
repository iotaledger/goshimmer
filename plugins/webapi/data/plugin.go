package data

import (
	"net/http"

	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/payload"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
	"github.com/iotaledger/goshimmer/plugins/webapi"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
	"github.com/labstack/echo"
)

var PLUGIN = node.NewPlugin("WebAPI data Endpoint", node.Enabled, configure)
var log *logger.Logger

func configure(plugin *node.Plugin) {
	log = logger.NewLogger("API-data")
	webapi.Server.POST("data", broadcastData)
}

// broadcastData creates a message of the given payload and
// broadcasts it to the node's neighbors. It returns the message ID if successful.
func broadcastData(c echo.Context) error {
	var request Request
	if err := c.Bind(&request); err != nil {
		log.Info(err.Error())
		return c.JSON(http.StatusBadRequest, Response{Error: err.Error()})
	}

	//TODO: to check max payload size allowed, if exceeding return an error
	msg := messagelayer.MessageFactory.IssuePayload(payload.NewData(request.Data))
	return c.JSON(http.StatusOK, Response{Id: msg.Id().String()})
}

type Response struct {
	Id    string `json:"id,omitempty"`
	Error string `json:"error,omitempty"`
}

type Request struct {
	Data []byte `json:"data"`
}
