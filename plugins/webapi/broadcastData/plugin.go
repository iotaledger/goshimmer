package broadcastData

import (
	"net/http"

	"github.com/iotaledger/goshimmer/packages/model/value_transaction"
	"github.com/iotaledger/goshimmer/plugins/webapi"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
	"github.com/iotaledger/hive.go/typeutils"
	"github.com/labstack/echo"
)

var PLUGIN = node.NewPlugin("WebAPI broadcastData Endpoint", node.Enabled, configure)
var log *logger.Logger

func configure(plugin *node.Plugin) {
	log = logger.NewLogger("API-broadcastData")
	webapi.Server.POST("broadcastData", broadcastData)
}

// broadcastData creates a data (0-value) transaction given an input of bytes and
// broadcasts it to the node's neighbors. It returns the transaction hash if successful.
func broadcastData(c echo.Context) error {
	var request Request
	if err := c.Bind(&request); err != nil {
		log.Info(err.Error())
		return c.JSON(http.StatusBadRequest, Response{Error: err.Error()})
	}

	log.Debug("Received - address:", request.Address, " data:", request.Data)

	tx := value_transaction.New()
	tx.SetHead(true)
	tx.SetTail(true)

	buffer := make([]byte, 2187)
	if len(request.Data) > 2187 {
		log.Warnf("data exceeds 2187 byte limit - (payload data size: %d)", len(request.Data))
		return c.JSON(http.StatusBadRequest, Response{Error: "data exceeds 2187 byte limit"})
	}

	copy(buffer, typeutils.StringToBytes(request.Data))

	// TODO: FIX FOR NEW TX LAYOUT

	/*
		trytes, err := trinary.BytesToTrytes(buffer)
		if err != nil {
			log.Warnf("trytes conversion failed: %s", err.Error())
			return c.JSON(http.StatusBadRequest, Response{Error: err.Error()})
		}

		err = address.ValidAddress(request.Address)
		if err != nil {
			log.Warnf("invalid Address: %s", request.Address)
			return c.JSON(http.StatusBadRequest, Response{Error: err.Error()})
		}

		tx.SetAddress(request.Address)
		tx.SetSignatureMessageFragment(trytes)
		tx.SetValue(0)
		tx.SetBranchTransactionHash(tipselectionn.GetRandomTip())
		tx.SetTrunkTransactionHash(tipselectionn.GetRandomTip(tx.GetBranchTransactionHash()))
		tx.SetTimestamp(uint(time.Now().Unix()))
		if err := tx.DoProofOfWork(meta_transaction.MIN_WEIGHT_MAGNITUDE); err != nil {
			log.Warnf("PoW failed: %s", err)
			return c.JSON(http.StatusInternalServerError, Response{Error: err.Error()})
		}

		gossip.Events.TransactionReceived.Trigger(&gossip.TransactionReceivedEvent{Data: tx.GetBytes(), Peer: &local.GetInstance().Peer})
	*/
	return c.JSON(http.StatusOK, Response{Hash: tx.GetHash()})
}

type Response struct {
	Hash  string `json:"hash,omitempty"`
	Error string `json:"error,omitempty"`
}

type Request struct {
	Address string `json:"address"`
	Data    string `json:"data"`
}
