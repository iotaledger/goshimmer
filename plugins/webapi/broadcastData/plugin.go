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

	var request webRequest
	if err := c.Bind(&request); err != nil {
		log.Info(err.Error())
		return requestFailed(c, err.Error())
	}
	log.Info("Received:", request.Data)
	tx := value_transaction.New()
	tx.SetHead(true)
	tx.SetTail(true)

	buffer := make([]byte, 6561)
	if len(request.Data) > 6561 {
		log.Warn("data exceding 6561 byte limit -", len(request.Data))
		return requestFailed(c, "data exceding 6561 byte limit")
	}

	copy(buffer, []byte(request.Data))

	trytes, err := trinary.BytesToTrytes(buffer)
	if err != nil {
		log.Warn("Trytes conversion failed", err.Error())
		return requestFailed(c, err.Error())
	}

	tx.SetSignatureMessageFragment(trytes)
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
	return c.JSON(http.StatusCreated, webResponse{
		Hash: txHash,
	})
}

func requestFailed(c echo.Context, message string) error {
	return c.JSON(http.StatusNotModified, webResponse{
		Error: message,
	})
}

type webResponse struct {
	Hash  string `json:"hash,omitempty"`
	Error string `json:"error,omitempty"`
}

type webRequest struct {
	Data string `json:"data"`
}
