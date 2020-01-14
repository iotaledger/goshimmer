package broadcastData

import (
	"net/http"
	"time"

	"github.com/iotaledger/goshimmer/packages/gossip"
	"github.com/iotaledger/goshimmer/packages/model/meta_transaction"
	"github.com/iotaledger/goshimmer/packages/model/value_transaction"
	"github.com/iotaledger/goshimmer/plugins/autopeering/local"
	"github.com/iotaledger/goshimmer/plugins/tipselection"
	"github.com/iotaledger/goshimmer/plugins/webapi"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
	"github.com/iotaledger/hive.go/typeutils"
	"github.com/iotaledger/iota.go/address"
	"github.com/iotaledger/iota.go/trinary"
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
		return requestFailed(c, err.Error())
	}

	log.Debug("Received - address:", request.Address, " data:", request.Data)

	tx := value_transaction.New()
	tx.SetHead(true)
	tx.SetTail(true)

	buffer := make([]byte, 2187)
	if len(request.Data) > 2187 {
		log.Warn("Data exceeding 2187 byte limit -", len(request.Data))
		return requestFailed(c, "Data exceeding 2187 byte limit")
	}

	copy(buffer, typeutils.StringToBytes(request.Data))

	trytes, err := trinary.BytesToTrytes(buffer)
	if err != nil {
		log.Warn("Trytes conversion failed", err.Error())
		return requestFailed(c, err.Error())
	}

	err = address.ValidAddress(request.Address)
	if err != nil {
		log.Warn("Invalid Address:", request.Address)
		return requestFailed(c, err.Error())
	}

	tx.SetAddress(request.Address)
	tx.SetSignatureMessageFragment(trytes)
	tx.SetValue(0)
	tx.SetBranchTransactionHash(tipselection.GetRandomTip())
	tx.SetTrunkTransactionHash(tipselection.GetRandomTip())
	tx.SetTimestamp(uint(time.Now().Unix()))
	if err := tx.DoProofOfWork(meta_transaction.MIN_WEIGHT_MAGNITUDE); err != nil {
		log.Warn("PoW failed", err)
		return requestFailed(c, err.Error())
	}

	gossip.Events.TransactionReceived.Trigger(&gossip.TransactionReceivedEvent{Data: tx.GetBytes(), Peer: &local.GetInstance().Peer})
	return requestSuccessful(c, tx.GetHash())
}

func requestSuccessful(c echo.Context, txHash string) error {
	return c.JSON(http.StatusCreated, Response{
		Hash: txHash,
	})
}

func requestFailed(c echo.Context, message string) error {
	return c.JSON(http.StatusBadRequest, Response{
		Error: message,
	})
}

type Response struct {
	Hash  string `json:"hash,omitempty"`
	Error string `json:"error,omitempty"`
}

type Request struct {
	Address string `json:"address"`
	Data    string `json:"data"`
}
