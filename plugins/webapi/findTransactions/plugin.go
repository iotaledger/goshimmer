package findTransactions

import (
	"net/http"

	"github.com/iotaledger/goshimmer/plugins/tangle"
	"github.com/iotaledger/goshimmer/plugins/webapi"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
	"github.com/iotaledger/iota.go/trinary"
	"github.com/labstack/echo"
)

var PLUGIN = node.NewPlugin("WebAPI findTransactions Endpoint", node.Enabled, configure)
var log *logger.Logger

func configure(plugin *node.Plugin) {
	log = logger.NewLogger("API-findTransactions")
	webapi.Server.POST("findTransactions", findTransactions)
}

// findTransactions returns the array of transaction hashes for the
// given addresses (in the same order as the parameters).
// If a node doesn't have any transaction hash for a given address in its ledger,
// the value at the index of that address is empty.
func findTransactions(c echo.Context) error {
	var request Request

	if err := c.Bind(&request); err != nil {
		log.Info(err.Error())
		return requestFailed(c, err.Error())
	}
	log.Debug("Received:", request.Addresses)
	result := make([][]trinary.Trytes, len(request.Addresses))

	for i, address := range request.Addresses {
		txs, err := tangle.ReadTransactionHashesForAddressFromDatabase(address)
		if err != nil {
			return requestFailed(c, err.Error())
		}
		result[i] = append(result[i], txs...)
	}

	return requestSuccessful(c, result)
}

func requestSuccessful(c echo.Context, txHashes [][]trinary.Trytes) error {
	return c.JSON(http.StatusOK, Response{
		Transactions: txHashes,
	})
}

func requestFailed(c echo.Context, message string) error {
	return c.JSON(http.StatusNotFound, Response{
		Error: message,
	})
}

type Response struct {
	Transactions [][]trinary.Trytes `json:"transactions,omitempty"` //string
	Error        string             `json:"error,omitempty"`
}

type Request struct {
	Addresses []string `json:"addresses"`
}
