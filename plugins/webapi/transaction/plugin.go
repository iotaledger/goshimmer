package transaction

import (
	"net/http"
	"time"

	"github.com/iotaledger/goshimmer/plugins/tangle"
	"github.com/iotaledger/goshimmer/plugins/webapi"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
	"github.com/labstack/echo"
)

var PLUGIN = node.NewPlugin("WebAPI Transaction Request Endpoint", node.Enabled, configure)
var log *logger.Logger

func configure(plugin *node.Plugin) {
	log = logger.NewLogger("API-Transaction")
	webapi.AddEndpoint("getTrytes", getTrytes)
}

// getTrytes returns the array of transaction trytes for the
// given transaction hashes (in the same order as the parameters).
// If a node doesn't have the trytes for a given transaction hash in its ledger,
// the value at the index of that transaction hash is nil.
func getTrytes(c echo.Context) error {
	c.Set("requestStartTime", time.Now())

	var request webRequest
	result := [][]byte{}
	if err := c.Bind(&request); err != nil {
		log.Info(err.Error())
		return requestFailed(c, err.Error())
	}
	log.Info("Received:", request.Hashes)

	for _, hash := range request.Hashes {
		tx, err := tangle.GetTransaction(hash)
		if err != nil {
			return requestFailed(c, err.Error())
		}
		if tx != nil {
			result = append(result, tx.GetBytes())
		} else {
			//tx not found
			result = append(result, nil)
		}

	}

	return requestSuccessful(c, result)
}

func requestSuccessful(c echo.Context, txs [][]byte) error {
	return c.JSON(http.StatusOK, webResponse{
		Duration: time.Since(c.Get("requestStartTime").(time.Time)).Nanoseconds() / 1e6,
		Trytes:   txs,
	})
}

func requestFailed(c echo.Context, message string) error {
	return c.JSON(http.StatusNotFound, webResponse{
		Duration: time.Since(c.Get("requestStartTime").(time.Time)).Nanoseconds() / 1e6,
		Error:    message,
	})
}

type webResponse struct {
	Duration int64    `json:"duration"`
	Trytes   [][]byte `json:"trytes"`
	Error    string   `json:"error"`
}

type webRequest struct {
	Hashes []string `json:"hashes"`
}
