package data

import (
	"net/http"

	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/payload"
	"github.com/iotaledger/goshimmer/plugins/issuer"
	"github.com/iotaledger/goshimmer/plugins/webapi"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
	"github.com/labstack/echo"
)

// PluginName is the name of the web API data endpoint plugin.
const PluginName = "WebAPI data Endpoint"

var (
	// Plugin is the plugin instance of the web API data endpoint plugin.
	Plugin = node.NewPlugin(PluginName, node.Enabled, configure)
	log    *logger.Logger
)

func configure(plugin *node.Plugin) {
	log = logger.NewLogger(PluginName)
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
	msg, err := issuer.IssuePayload(payload.NewData(request.Data))
	if err != nil {
		return c.JSON(http.StatusBadRequest, Response{Error: err.Error()})
	}
	return c.JSON(http.StatusOK, Response{ID: msg.Id().String()})
}

// Response contains the ID of the message sent.
type Response struct {
	ID    string `json:"id,omitempty"`
	Error string `json:"error,omitempty"`
}

// Request contains the data of the message to send.
type Request struct {
	Data []byte `json:"data"`
}
