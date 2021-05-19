package faucet

import (
	"crypto"
	"fmt"
	"net/http"
	"sync"

	"github.com/iotaledger/hive.go/identity"
	"github.com/iotaledger/hive.go/node"
	"github.com/labstack/echo"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/mana"
	"github.com/iotaledger/goshimmer/packages/pow"
	"github.com/iotaledger/goshimmer/plugins/config"
	"github.com/iotaledger/goshimmer/plugins/faucet"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
	"github.com/iotaledger/goshimmer/plugins/webapi"
	"github.com/iotaledger/goshimmer/plugins/webapi/jsonmodels"
)

var (
	// plugin is the plugin instance of the web API info endpoint plugin.
	plugin              *node.Plugin
	once                sync.Once
	powVerifier         = pow.New(crypto.BLAKE2b_512)
	targetPoWDifficulty int
)

// Plugin gets the plugin instance.
func Plugin() *node.Plugin {
	once.Do(func() {
		plugin = node.NewPlugin("WebAPI faucet Endpoint", node.Enabled, configure)
	})
	return plugin
}

func configure(plugin *node.Plugin) {
	webapi.Server().POST("faucet", requestFunds)
	targetPoWDifficulty = config.Node().Int(faucet.CfgFaucetPoWDifficulty)
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

	faucetPayload := faucet.NewRequest(addr, accessManaPledgeID, consensusManaPledgeID, request.Nonce)

	// verify PoW
	leadingZeroes, err := powVerifier.LeadingZeros(faucetPayload.Bytes())
	if err != nil {
		plugin.LogInfof("couldn't verify PoW of funding request for address %s", addr.Base58())
		return c.JSON(http.StatusBadRequest, jsonmodels.FaucetResponse{Error: "Could not verify PoW"})
	}

	if leadingZeroes < targetPoWDifficulty {
		plugin.LogInfof("funding request for address %s doesn't fulfill PoW requirement %d vs. %d", addr.Base58(), targetPoWDifficulty, leadingZeroes)
		return c.JSON(http.StatusBadRequest, jsonmodels.FaucetResponse{Error: "Funding request doesn't fulfill PoW requirement"})
	}

	msg, err := messagelayer.Tangle().MessageFactory.IssuePayload(faucetPayload)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, jsonmodels.FaucetResponse{Error: fmt.Sprintf("Failed to send faucetrequest: %s", err.Error())})
	}

	return c.JSON(http.StatusOK, jsonmodels.FaucetResponse{ID: msg.ID().Base58()})
}
