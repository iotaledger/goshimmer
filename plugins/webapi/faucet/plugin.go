package faucet

import (
	"fmt"
	"net/http"
	goSync "sync"

	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/address"
	"github.com/iotaledger/goshimmer/plugins/config"
	"github.com/iotaledger/goshimmer/plugins/faucet"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
	"github.com/iotaledger/goshimmer/plugins/webapi"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
	"github.com/labstack/echo"
)

const (
	// PluginName is the name of the web API faucet endpoint plugin.
	PluginName = "WebAPI faucet Endpoint"
)

var (
	// plugin is the plugin instance of the web API info endpoint plugin.
	plugin    *node.Plugin
	once      goSync.Once
	log       *logger.Logger
	fundingMu goSync.Mutex
)

// Plugin gets the plugin instance.
func Plugin() *node.Plugin {
	once.Do(func() {
		plugin = node.NewPlugin(PluginName, node.Enabled, configure)
	})
	return plugin
}

func configure(plugin *node.Plugin) {
	log = logger.NewLogger("API-faucet")
	webapi.Server().POST("faucet", requestFunds)
}

// requestFunds creates a faucet request (0-value) message with the given destination address and
// broadcasts it to the node's neighbors. It returns the message ID if successful.
func requestFunds(c echo.Context) error {
	fundingMu.Lock()
	defer fundingMu.Unlock()
	var request Request
	var addr address.Address
	if err := c.Bind(&request); err != nil {
		log.Info(err.Error())
		return c.JSON(http.StatusBadRequest, Response{Error: err.Error()})
	}

	log.Debug("Received - address:", request.Address)

	addr, err := address.FromBase58(request.Address)
	if err != nil {
		return c.JSON(http.StatusBadRequest, Response{Error: "Invalid address"})
	}

	faucetPayload, err := faucet.NewRequest(addr, config.Node().Int(faucet.CfgFaucetPoWDifficulty))
	if err != nil {
		return c.JSON(http.StatusBadRequest, Response{Error: err.Error()})
	}
	msg, err := messagelayer.Tangle().MessageFactory.IssuePayload(faucetPayload, messagelayer.Tangle())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, Response{Error: fmt.Sprintf("Failed to send faucetrequest: %s", err.Error())})
	}

	return c.JSON(http.StatusOK, Response{ID: msg.ID().String()})
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
