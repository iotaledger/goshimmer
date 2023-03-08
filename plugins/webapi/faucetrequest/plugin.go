package faucetrequest

import (
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"
	"go.uber.org/dig"

	"github.com/iotaledger/goshimmer/packages/app/blockissuer"
	faucetpkg "github.com/iotaledger/goshimmer/packages/app/faucet"
	"github.com/iotaledger/goshimmer/packages/app/jsonmodels"
	"github.com/iotaledger/goshimmer/packages/node"
	"github.com/iotaledger/goshimmer/packages/protocol"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/vm/devnetvm"
	"github.com/iotaledger/hive.go/crypto/identity"
)

var (
	// Plugin is the plugin instance of the web API info endpoint plugin.
	Plugin *node.Plugin
	deps   = new(dependencies)
)

type dependencies struct {
	dig.In

	Server      *echo.Echo
	Protocol    *protocol.Protocol
	BlockIssuer *blockissuer.BlockIssuer
}

// Plugin gets the plugin instance.
func init() {
	Plugin = node.NewPlugin("WebAPIFaucetRequestEndpoint", deps, node.Enabled, configure)
}

func configure(_ *node.Plugin) {
	deps.Server.POST("faucetrequest", requestFunds)
}

// requestFunds creates a faucet request (0-value) block with the given destination address and
// broadcasts it to the node's neighbors. It returns the block ID if successful.
func requestFunds(c echo.Context) error {
	var request jsonmodels.FaucetRequest
	if err := c.Bind(&request); err != nil {
		Plugin.LogInfo(err.Error())
		return c.JSON(http.StatusBadRequest, jsonmodels.FaucetRequestResponse{Error: err.Error()})
	}

	Plugin.LogInfo("Received - address:", request.Address)
	Plugin.LogDebug(request)

	addr, err := devnetvm.AddressFromBase58EncodedString(request.Address)
	if err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels.FaucetRequestResponse{Error: "Invalid address"})
	}

	var accessManaPledgeID identity.ID
	var consensusManaPledgeID identity.ID
	if request.AccessManaPledgeID != "" {
		accessManaPledgeID, err = identity.DecodeIDBase58(request.AccessManaPledgeID)
		if err != nil {
			return c.JSON(http.StatusBadRequest, jsonmodels.FaucetRequestResponse{Error: "Invalid access mana node ID"})
		}
	}

	if request.ConsensusManaPledgeID != "" {
		consensusManaPledgeID, err = identity.DecodeIDBase58(request.ConsensusManaPledgeID)
		if err != nil {
			return c.JSON(http.StatusBadRequest, jsonmodels.FaucetRequestResponse{Error: "Invalid consensus mana node ID"})
		}
	}

	faucetPayload := faucetpkg.NewRequest(addr, accessManaPledgeID, consensusManaPledgeID, request.Nonce)
	// TODO: finish when issuing blocks if figured out
	blk, err := deps.BlockIssuer.IssuePayload(faucetPayload)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, jsonmodels.FaucetRequestResponse{Error: fmt.Sprintf("Failed to send faucetrequest: %s", err.Error())})
	}

	return c.JSON(http.StatusOK, jsonmodels.FaucetRequestResponse{ID: blk.ID().Base58()})
}
