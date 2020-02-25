package getTransactionTrytesByHash

import (
	"net/http"

	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
	"github.com/iotaledger/iota.go/trinary"
	"github.com/labstack/echo"

	"github.com/iotaledger/goshimmer/plugins/webapi"
)

var PLUGIN = node.NewPlugin("WebAPI getTransactionTrytesByHash Endpoint", node.Enabled, configure)
var log *logger.Logger

func configure(plugin *node.Plugin) {
	log = logger.NewLogger("API-getTransactionTrytesByHash")
	webapi.Server.POST("getTransactionTrytesByHash", getTransactionTrytesByHash)
}

// getTransactionTrytesByHash returns the array of transaction trytes for the
// given transaction hashes (in the same order as the parameters).
// If a node doesn't have the trytes for a given transaction hash in its ledger,
// the value at the index of that transaction hash is empty.
func getTransactionTrytesByHash(c echo.Context) error {

	var request Request
	result := []trinary.Trytes{}
	if err := c.Bind(&request); err != nil {
		log.Info(err.Error())
		return c.JSON(http.StatusBadRequest, Response{Error: err.Error()})
	}
	log.Debug("Received:", request.Hashes)

	/*
		for _, hash := range request.Hashes {
			tx, err := tangle_old.GetTransaction(hash)
			if err != nil {
				return c.JSON(http.StatusInternalServerError, Response{Error: err.Error()})
			}
			if tx == nil {
				return c.JSON(http.StatusNotFound, Response{Error: fmt.Sprintf("transaction not found: %s", hash)})
			}
			trytes := trinary.MustTritsToTrytes(tx.GetTrits())
			result = append(result, trytes)
		}
	*/

	return c.JSON(http.StatusOK, Response{Trytes: result})
}

type Response struct {
	Trytes []trinary.Trytes `json:"trytes,omitempty"` //string
	Error  string           `json:"error,omitempty"`
}

type Request struct {
	Hashes []string `json:"hashes"`
}
