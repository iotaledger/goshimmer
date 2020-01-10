package broadcastData

import (
	"net/http"
	"time"

	"github.com/iotaledger/goshimmer/plugins/autopeering/local"

	"github.com/iotaledger/goshimmer/packages/gossip"
	"github.com/iotaledger/goshimmer/packages/model/meta_transaction"
	"github.com/iotaledger/goshimmer/packages/model/value_transaction"
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
	webapi.AddEndpoint("broadcastData", broadcastData)
}

// broadcastData creates a data (0-value) transaction given an input of bytes and
// broadcasts it to the node's neighbors. It returns the transaction hash if successful.
func broadcastData(c echo.Context) error {
	c.Set("requestStartTime", time.Now())

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
		return requestFailed(c, "data exceding 6561 byte limit")
	}

	copy(buffer, []byte(request.Data))

	trytes, err := trinary.BytesToTrytes(buffer)
	if err != nil {
		log.Info("Trytes conversion", err.Error())
		return requestFailed(c, err.Error())
	}

	tx.SetSignatureMessageFragment(trytes)
	tx.SetBranchTransactionHash(tipselection.GetRandomTip())
	tx.SetTrunkTransactionHash(tipselection.GetRandomTip())
	tx.SetTimestamp(uint(time.Now().Unix()))
	if err := tx.DoProofOfWork(meta_transaction.MIN_WEIGHT_MAGNITUDE); err != nil {
		log.Warn("PoW failed", err)
	}

	gossip.Events.TransactionReceived.Trigger(&gossip.TransactionReceivedEvent{Data: tx.GetBytes(), Peer: &local.GetInstance().Peer})
	return requestSuccessful(c, tx.GetHash())
}

func requestSuccessful(c echo.Context, txHash string) error {
	return c.JSON(http.StatusCreated, webResponse{
		Duration: time.Since(c.Get("requestStartTime").(time.Time)).Nanoseconds() / 1e6,
		Hash:     txHash,
	})
}

func requestFailed(c echo.Context, message string) error {
	return c.JSON(http.StatusNotModified, webResponse{
		Duration: time.Since(c.Get("requestStartTime").(time.Time)).Nanoseconds() / 1e6,
		Error:    message,
	})
}

type webResponse struct {
	Duration int64  `json:"duration"`
	Hash     string `json:"hash"`
	Error    string `json:"error"`
}

type webRequest struct {
	Data string `json:"data"`
}
