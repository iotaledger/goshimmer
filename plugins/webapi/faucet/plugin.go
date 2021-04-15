package faucet

import (
	"fmt"
	"net/http"
	goSync "sync"

	"github.com/iotaledger/hive.go/identity"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
	"github.com/labstack/echo"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/mana"
	"github.com/iotaledger/goshimmer/plugins/config"
	"github.com/iotaledger/goshimmer/plugins/faucet"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
	"github.com/iotaledger/goshimmer/plugins/webapi"
	"github.com/iotaledger/goshimmer/plugins/webapi/jsonmodels"
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
	var request jsonmodels.FaucetRequest
	var addr ledgerstate.Address
	if err := c.Bind(&request); err != nil {
		log.Info(err.Error())
		return c.JSON(http.StatusBadRequest, jsonmodels.FaucetResponse{Error: err.Error()})
	}

	log.Debug("Received - address:", request.Address)

	addr, err := ledgerstate.AddressFromBase58EncodedString(request.Address)
	if err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels.FaucetResponse{Error: "Invalid address"})
	}

	emptyID := identity.ID{}
	accessManaPledgeID := emptyID
	consensusManaPledgeID := emptyID

	if request.AccessManaPledgeID != "" {
		accessManaPledgeID, err = mana.IDFromStr(request.AccessManaPledgeID)
		if err != nil {
			return c.JSON(http.StatusBadRequest, jsonmodels.FaucetResponse{Error: "Invalid access mana node ID"})
		}
	}

	if request.ConsensusManaPledgeID != "" {
		consensusManaPledgeID, err = mana.IDFromStr(request.ConsensusManaPledgeID)
		if err != nil {
			return c.JSON(http.StatusBadRequest, jsonmodels.FaucetResponse{Error: "Invalid consensus mana node ID"})
		}
	}

	faucetPayload, err := faucet.NewRequest(addr, config.Node().Int(faucet.CfgFaucetPoWDifficulty), accessManaPledgeID, consensusManaPledgeID)
	if err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels.FaucetResponse{Error: err.Error()})
	}
	msg, err := messagelayer.Tangle().MessageFactory.IssuePayload(faucetPayload, messagelayer.Tangle())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, jsonmodels.FaucetResponse{Error: fmt.Sprintf("Failed to send faucetrequest: %s", err.Error())})
	}

	return c.JSON(http.StatusOK, jsonmodels.FaucetResponse{ID: msg.ID().Base58()})
}
