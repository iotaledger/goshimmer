package getTransactionObjectsByHash

import (
	"net/http"

	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
	"github.com/iotaledger/iota.go/trinary"
	"github.com/labstack/echo"

	"github.com/iotaledger/goshimmer/plugins/webapi"
)

var PLUGIN = node.NewPlugin("WebAPI getTransactionObjectsByHash Endpoint", node.Enabled, configure)
var log *logger.Logger

func configure(plugin *node.Plugin) {
	log = logger.NewLogger("API-getTransactionObjectsByHash")
	webapi.Server.POST("getTransactionObjectsByHash", getTransactionObjectsByHash)
}

// getTransactionObjectsByHash returns the array of transactions for the
// given transaction hashes (in the same order as the parameters).
// If a node doesn't have the transaction for a given transaction hash in its ledger,
// the value at the index of that transaction hash is empty.
func getTransactionObjectsByHash(c echo.Context) error {

	var request Request
	result := []Transaction{}

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
			t := Transaction{
				Hash:                     tx.GetHash(),
				WeightMagnitude:          tx.GetWeightMagnitude(),
				TrunkTransactionHash:     tx.GetTrunkTransactionHash(),
				BranchTransactionHash:    tx.GetBranchTransactionHash(),
				Head:                     tx.IsHead(),
				Tail:                     tx.IsTail(),
				Nonce:                    tx.GetNonce(),
				Address:                  tx.GetAddress(),
				Value:                    tx.GetValue(),
				Timestamp:                tx.GetTimestamp(),
				SignatureMessageFragment: tx.GetSignatureMessageFragment(),
			}
			result = append(result, t)
		}
	*/

	return c.JSON(http.StatusOK, Response{Transactions: result})
}

type Response struct {
	Transactions []Transaction `json:"transaction,omitempty"`
	Error        string        `json:"error,omitempty"`
}

type Request struct {
	Hashes []string `json:"hashes"`
}

type Transaction struct {
	Hash                     trinary.Trytes `json:"hash,omitempty"`
	WeightMagnitude          int            `json:"weightMagnitude,omitempty"`
	TrunkTransactionHash     trinary.Trytes `json:"trunkTransactionHash,omitempty"`
	BranchTransactionHash    trinary.Trytes `json:"branchTransactionHash,omitempty"`
	Head                     bool           `json:"head,omitempty"`
	Tail                     bool           `json:"tail,omitempty"`
	Nonce                    trinary.Trytes `json:"nonce,omitempty"`
	Address                  trinary.Trytes `json:"address,omitempty"`
	Value                    int64          `json:"value,omitempty"`
	Timestamp                uint           `json:"timestamp,omitempty"`
	SignatureMessageFragment trinary.Trytes `json:"signatureMessageFragment,omitempty"`
}
