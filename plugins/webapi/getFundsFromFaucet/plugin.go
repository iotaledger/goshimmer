package getFundsFromFaucet

import (
	"net/http"
	"time"

	faucetpayload "github.com/iotaledger/goshimmer/packages/binary/faucet/payload"
	"github.com/iotaledger/goshimmer/packages/binary/signature/ed25119"
	"github.com/iotaledger/goshimmer/packages/binary/tangle/model/message"
	"github.com/iotaledger/goshimmer/packages/binary/valuetransfer/address"
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

// requestFunds creates a faucet request (0-value) transaction given an input of address and
// broadcasts it to the node's neighbors. It returns the transaction ID if successful.
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

	// TODO: build faucet transaction with transaction factory
	tx := message.New(
		message.EmptyId,
		message.EmptyId,
		ed25119.GenerateKeyPair(),
		time.Now(),
		0,
		faucetpayload.New([]byte(request.Address)),
	)

	return c.JSON(http.StatusOK, Response{Id: tx.GetId().String()})
}

type Response struct {
	Id    string `json:"id,omitempty"`
	Error string `json:"error,omitempty"`
}

type Request struct {
	Address string `json:"address"`
}
