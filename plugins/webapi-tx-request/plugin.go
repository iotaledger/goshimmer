package webapi_tx_request

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
var log = logger.NewLogger("API-TxRequest")

func configure(plugin *node.Plugin) {
	webapi.AddEndpoint("txRequest", Handler)
}

func Handler(c echo.Context) error {
	c.Set("requestStartTime", time.Now())

	var request webRequest
	if err := c.Bind(&request); err != nil {
		log.Info(err.Error())
		return requestFailed(c, err.Error())
	}
	log.Info("Received:", request.TransactionHash)

	tx, err := tangle.GetTransaction(request.TransactionHash)
	if err != nil {
		return requestFailed(c, err.Error())
	}
	if tx == nil {
		return requestFailed(c, "Tx not found")
	}

	return requestSuccessful(c, tx.GetBytes())
}

func requestSuccessful(c echo.Context, tx []byte) error {
	return c.JSON(http.StatusOK, webResponse{
		Duration:    time.Since(c.Get("requestStartTime").(time.Time)).Nanoseconds() / 1e6,
		Transaction: tx,
		Status:      "OK",
	})
}

func requestFailed(c echo.Context, message string) error {
	return c.JSON(http.StatusNotFound, webResponse{
		Duration: time.Since(c.Get("requestStartTime").(time.Time)).Nanoseconds() / 1e6,
		Status:   message,
	})
}

type webResponse struct {
	Duration    int64  `json:"duration"`
	Transaction []byte `json:"transaction"`
	Status      string `json:"status"`
}

type webRequest struct {
	TransactionHash string `json:"transactionHash"`
}
