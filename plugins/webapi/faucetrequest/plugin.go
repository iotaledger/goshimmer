package faucetrequest

import (
	"fmt"
	"net/http"

	"github.com/iotaledger/hive.go/core/identity"
	"github.com/iotaledger/hive.go/core/node"
	"github.com/labstack/echo"
	"go.uber.org/dig"

	faucetpkg "github.com/iotaledger/goshimmer/packages/app/faucet"
	"github.com/iotaledger/goshimmer/packages/app/jsonmodels"
	"github.com/iotaledger/goshimmer/packages/core/ledger/vm/devnetvm"
	"github.com/iotaledger/goshimmer/packages/core/mana"
	"github.com/iotaledger/goshimmer/packages/core/tangleold"
)

var (
	// Plugin is the plugin instance of the web API info endpoint plugin.
	Plugin *node.Plugin
	deps   = new(dependencies)
)

type dependencies struct {
	dig.In

	Server *echo.Echo
	Tangle *tangleold.Tangle
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
		accessManaPledgeID, err = mana.IDFromStr(request.AccessManaPledgeID)
		if err != nil {
			return c.JSON(http.StatusBadRequest, jsonmodels.FaucetRequestResponse{Error: "Invalid access mana node ID"})
		}
	}

	if request.ConsensusManaPledgeID != "" {
		consensusManaPledgeID, err = mana.IDFromStr(request.ConsensusManaPledgeID)
		if err != nil {
			return c.JSON(http.StatusBadRequest, jsonmodels.FaucetRequestResponse{Error: "Invalid consensus mana node ID"})
		}
	}

	faucetPayload := faucetpkg.NewRequest(addr, accessManaPledgeID, consensusManaPledgeID, request.Nonce)

	blk, err := deps.Tangle.BlockFactory.IssuePayload(faucetPayload)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, jsonmodels.FaucetRequestResponse{Error: fmt.Sprintf("Failed to send faucetrequest: %s", err.Error())})
	}

	return c.JSON(http.StatusOK, jsonmodels.FaucetRequestResponse{ID: blk.ID().Base58()})
}
