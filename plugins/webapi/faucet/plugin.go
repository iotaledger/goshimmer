package faucet

import (
	"net/http"

	faucetpayload "github.com/iotaledger/goshimmer/dapps/faucet/packages/payload"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/address"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
	"github.com/iotaledger/goshimmer/plugins/webapi"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
	"github.com/labstack/echo"
)

// Plugin is the plugin instance of the web API faucet request endpoint plugin.
var Plugin = node.NewPlugin("WebAPI faucet Endpoint", node.Enabled, configure)
var log *logger.Logger

func configure(plugin *node.Plugin) {
	log = logger.NewLogger("API-faucet")
	webapi.Server.POST("faucet", requestFunds)
}

// requestFunds creates a faucet request (0-value) message given an input of address and
// broadcasts it to the node's neighbors. It returns the message ID if successful.
func requestFunds(c echo.Context) error {
	var request Request
	var addr address.Address
	if err := c.Bind(&request); err != nil {
		log.Info(err.Error())
		return c.JSON(http.StatusBadRequest, Response{Error: err.Error()})
	}

	log.Debug("Received - address:", request.Address)

	addr, err := address.FromBase58(request.Address)
	if err != nil {
		log.Warnf("Invalid address")
		return c.JSON(http.StatusBadRequest, Response{Error: "Invalid address"})
	}

	// Build faucet message with transaction factory
	msg := messagelayer.MessageFactory.IssuePayload(faucetpayload.New(addr))
	if msg == nil {
		return c.JSON(http.StatusInternalServerError, Response{Error: "Fail to send faucetrequest"})
	}

	return c.JSON(http.StatusOK, Response{ID: msg.Id().String()})
}

// Response contains the ID of the message sent.
type Response struct {
	ID    string `json:"id,omitempty"`
	Error string `json:"error,omitempty"`
}

// Request contains the address to request funds from faucet.
type Request struct {
	Address string `json:"address"`
}
