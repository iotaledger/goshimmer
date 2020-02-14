package findTransactionHashes

import (
	"net/http"

	"github.com/iotaledger/goshimmer/plugins/webapi"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
	"github.com/iotaledger/iota.go/trinary"
	"github.com/labstack/echo"
)

var PLUGIN = node.NewPlugin("WebAPI findTransactionHashes Endpoint", node.Enabled, configure)
var log *logger.Logger

func configure(plugin *node.Plugin) {
	log = logger.NewLogger("API-findTransactionHashes")
	webapi.Server.POST("findTransactionHashes", findTransactionHashes)
}

// findTransactionHashes returns the array of transaction hashes for the
// given addresses (in the same order as the parameters).
// If a node doesn't have any transaction hash for a given address in its ledger,
// the value at the index of that address is empty.
func findTransactionHashes(c echo.Context) error {
	return c.JSON(http.StatusInternalServerError, Response{Error: "TODO: ADD LOGIC ACCORDING TO VALUE TANGLE"})

	// TODO: ADD LOGIC ACCORDING TO VALUE TANGLE
	/*
		var request Request

		if err := c.Bind(&request); err != nil {
			log.Info(err.Error())
			return c.JSON(http.StatusBadRequest, Response{Error: err.Error()})
		}
		log.Debug("Received:", request.Addresses)
		result := make([][]trinary.Trytes, len(request.Addresses))

		for i, address := range request.Addresses {
			txs, err := tangle_old.ReadTransactionHashesForAddressFromDatabase(address)
			if err != nil {
				return c.JSON(http.StatusInternalServerError, Response{Error: err.Error()})
			}
			result[i] = append(result[i], txs...)
		}

		return c.JSON(http.StatusOK, Response{Transactions: result})
	*/
}

type Response struct {
	Transactions [][]trinary.Trytes `json:"transactions,omitempty"` //string
	Error        string             `json:"error,omitempty"`
}

type Request struct {
	Addresses []string `json:"addresses"`
}
