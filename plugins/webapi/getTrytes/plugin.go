package getTrytes

import (
	"net/http"

	"github.com/iotaledger/goshimmer/plugins/tangle"
	"github.com/iotaledger/goshimmer/plugins/webapi"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
	"github.com/iotaledger/iota.go/trinary"
	"github.com/labstack/echo"
)

var PLUGIN = node.NewPlugin("WebAPI getTrytes Endpoint", node.Enabled, configure)
var log *logger.Logger

func configure(plugin *node.Plugin) {
	log = logger.NewLogger("API-getTrytes")
	webapi.Server.GET("getTrytes", getTrytes)
}

// getTrytes returns the array of transaction trytes for the
// given transaction hashes (in the same order as the parameters).
// If a node doesn't have the trytes for a given transaction hash in its ledger,
// the value at the index of that transaction hash is empty.
func getTrytes(c echo.Context) error {

	var request Request
	result := []trinary.Trytes{}
	if err := c.Bind(&request); err != nil {
		log.Info(err.Error())
		return requestFailed(c, err.Error())
	}
	log.Debug("Received:", request.Hashes)

	for _, hash := range request.Hashes {
		tx, err := tangle.GetTransaction(hash)
		if err != nil {
			return requestFailed(c, err.Error())
		}
		if tx != nil {
			trytes := trinary.MustTritsToTrytes(tx.GetTrits())
			result = append(result, trytes)
		} else {
			//tx not found
			result = append(result, "")
		}

	}

	return requestSuccessful(c, result)
}

func requestSuccessful(c echo.Context, txTrytes []trinary.Trytes) error {
	return c.JSON(http.StatusOK, Response{
		Trytes: txTrytes,
	})
}

func requestFailed(c echo.Context, message string) error {
	return c.JSON(http.StatusNotFound, Response{
		Error: message,
	})
}

type Response struct {
	Trytes []trinary.Trytes `json:"trytes,omitempty"` //string
	Error  string           `json:"error,omitempty"`
}

type Request struct {
	Hashes []string `json:"hashes"`
}
