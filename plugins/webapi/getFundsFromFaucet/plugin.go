package getFundsFromFaucet

import (
	"net/http"

	faucetpayload "github.com/iotaledger/goshimmer/packages/binary/faucet/payload"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/address"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
	"github.com/iotaledger/goshimmer/plugins/webapi"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
	"github.com/labstack/echo"
)

var PLUGIN = node.NewPlugin("WebAPI faucet Endpoint", node.Enabled, configure)
var log *logger.Logger

func configure(plugin *node.Plugin) {
	log = logger.NewLogger("API-faucet")
	webapi.Server.POST("faucet", requestFunds)
}

// requestFunds creates a faucet request (0-value) message given an input of address and
// broadcasts it to the node's neighbors. It returns the message ID if successful.
func requestFunds(c echo.Context) error {
	var request Request
	if err := c.Bind(&request); err != nil {
		log.Info(err.Error())
		return c.JSON(http.StatusBadRequest, Response{Error: err.Error()})
	}

	log.Debug("Received - address:", request.Address)

	if len(request.Address) > address.Length {
		log.Warnf("Invalid address")
		return c.JSON(http.StatusBadRequest, Response{Error: "Invalid address"})
	}

	// Build faucet message with transaction factory
	msg := messagelayer.MessageFactory.IssuePayload(faucetpayload.New([]byte(request.Address)))
	if msg == nil {
		return c.JSON(http.StatusInternalServerError, Response{Error: "Fail to send faucetrequest"})
	}

	return c.JSON(http.StatusOK, Response{Id: msg.Id().String()})
}

type Response struct {
	Id    string `json:"id,omitempty"`
	Error string `json:"error,omitempty"`
}

type Request struct {
	Address string `json:"address"`
}
