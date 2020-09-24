package webapi

import (
	"fmt"
	"net/http"
	goSync "sync"

	"github.com/iotaledger/goshimmer/dapps/faucet"
	faucetpayload "github.com/iotaledger/goshimmer/dapps/faucet/packages/payload"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/address"
	"github.com/iotaledger/goshimmer/plugins/config"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
	"github.com/labstack/echo"
)

const FaucetEndpoint = "faucet"

var fundingMu goSync.Mutex

func init() {
	Server().POST("faucet", requestFunds)
}

// requestFunds creates a faucet request (0-value) message with the given destination address and
// broadcasts it to the node's neighbors. It returns the message ID if successful.
func requestFunds(c echo.Context) error {
	if _, exists := DisabledAPIs[FaucetEndpoint]; exists {
		return c.JSON(http.StatusForbidden, FaucetResponse{Error: "Forbidden endpoint"})
	}

	fundingMu.Lock()
	defer fundingMu.Unlock()

	var request FaucetRequest
	var addr address.Address
	if err := c.Bind(&request); err != nil {
		log.Info(err.Error())
		return c.JSON(http.StatusBadRequest, FaucetResponse{Error: err.Error()})
	}

	log.Debug("Received - address:", request.Address)

	addr, err := address.FromBase58(request.Address)
	if err != nil {
		return c.JSON(http.StatusBadRequest, FaucetResponse{Error: "Invalid address"})
	}

	faucetPayload, err := faucetpayload.New(addr, config.Node().GetInt(faucet.CfgFaucetPoWDifficulty))
	if err != nil {
		return c.JSON(http.StatusBadRequest, FaucetResponse{Error: err.Error()})
	}
	msg, err := messagelayer.MessageFactory().IssuePayload(faucetPayload)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, FaucetResponse{Error: fmt.Sprintf("Failed to send faucetrequest: %s", err.Error())})
	}

	return c.JSON(http.StatusOK, FaucetResponse{ID: msg.ID().String()})
}

// FaucetResponse contains the ID of the message sent.
type FaucetResponse struct {
	ID    string `json:"id,omitempty"`
	Error string `json:"error,omitempty"`
}

// FaucetRequest contains the address to request funds from faucet.
type FaucetRequest struct {
	Address string `json:"address"`
}
