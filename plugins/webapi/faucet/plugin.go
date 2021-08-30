package faucet

import (
	"fmt"
	"net/http"
	"sync"

	"github.com/iotaledger/hive.go/identity"
	"github.com/iotaledger/hive.go/node"
	"github.com/labstack/echo"

	faucetpkg "github.com/iotaledger/goshimmer/packages/faucet"
	"github.com/iotaledger/goshimmer/packages/jsonmodels"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/mana"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
	"github.com/iotaledger/goshimmer/plugins/webapi"
)

var (
	// plugin is the plugin instance of the web API info endpoint plugin.
	plugin *node.Plugin
	once   sync.Once
)

// Plugin gets the plugin instance.
func Plugin() *node.Plugin {
	once.Do(func() {
		plugin = node.NewPlugin("WebAPIFaucetEndpoint", node.Enabled, configure)
	})
	return plugin
}

func configure(_ *node.Plugin) {
	webapi.Server().POST("faucet", requestFunds)
}

// requestFunds creates a faucet request (0-value) message with the given destination address and
// broadcasts it to the node's neighbors. It returns the message ID if successful.
func requestFunds(c echo.Context) error {
	var request jsonmodels.FaucetRequest
	if err := c.Bind(&request); err != nil {
		plugin.LogInfo(err.Error())
		return c.JSON(http.StatusBadRequest, jsonmodels.FaucetResponse{Error: err.Error()})
	}

	plugin.LogInfo("Received - address:", request.Address)
	plugin.LogDebug(request)

	addr, err := ledgerstate.AddressFromBase58EncodedString(request.Address)
	if err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels.FaucetResponse{Error: "Invalid address"})
	}

	var accessManaPledgeID identity.ID
	var consensusManaPledgeID identity.ID
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

	faucetPayload := faucetpkg.NewRequest(addr, accessManaPledgeID, consensusManaPledgeID, request.Nonce)

	msg, err := messagelayer.Tangle().MessageFactory.IssuePayload(faucetPayload)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, jsonmodels.FaucetResponse{Error: fmt.Sprintf("Failed to send faucetrequest: %s", err.Error())})
	}

	return c.JSON(http.StatusOK, jsonmodels.FaucetResponse{ID: msg.ID().Base58()})
}
