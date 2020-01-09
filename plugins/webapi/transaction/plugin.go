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
	webapi.AddEndpoint("getTrytes", getTrytesHandler)
}

func getTrytesHandler(c echo.Context) error {
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
			//return requestFailed(c, "Tx not found")
			result = append(result, tx.GetBytes())
		}
	}

	return requestSuccessful(c, result)
}

func requestSuccessful(c echo.Context, txs [][]byte) error {
	return c.JSON(http.StatusOK, webResponse{
		Duration:     time.Since(c.Get("requestStartTime").(time.Time)).Nanoseconds() / 1e6,
		Transactions: txs,
		Status:       "OK",
	})
}

func requestFailed(c echo.Context, message string) error {
	return c.JSON(http.StatusNotFound, webResponse{
		Duration: time.Since(c.Get("requestStartTime").(time.Time)).Nanoseconds() / 1e6,
		Status:   message,
	})
}

type webResponse struct {
	Duration     int64    `json:"duration"`
	Transactions [][]byte `json:"transaction"`
	Status       string   `json:"status"`
}

type webRequest struct {
	Hashes []string `json:"Hashes"`
}
