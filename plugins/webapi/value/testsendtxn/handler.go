package testsendtxn

import (
	"net/http"

	"github.com/iotaledger/goshimmer/dapps/valuetransfers"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/address"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/balance"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/transaction"
	"github.com/iotaledger/goshimmer/plugins/issuer"
	"github.com/iotaledger/goshimmer/plugins/webapi/value/utils"
	"github.com/labstack/echo"
	"github.com/labstack/gommon/log"
)

// Handler sends a transaction.
func Handler(c echo.Context) error {
	var request Request
	if err := c.Bind(&request); err != nil {
		log.Info(err.Error())
		return c.JSON(http.StatusBadRequest, Response{Error: err.Error()})
	}

	// prepare inputs
	outputids := []transaction.OutputID{}
	for _, in := range request.Inputs {
		id, err := transaction.OutputIDFromBase58(in)
		if err != nil {
			log.Info(err.Error())
			return c.JSON(http.StatusBadRequest, Response{Error: err.Error()})
		}
		outputids = append(outputids, id)
	}
	inputs := transaction.NewInputs(outputids...)

	// prepare outputs
	outmap := map[address.Address][]*balance.Balance{}
	for _, out := range request.Outputs {
		addr, err := address.FromBase58(out.Address)
		if err != nil {
			log.Info(err.Error())
			return c.JSON(http.StatusBadRequest, Response{Error: err.Error()})
		}

		// iterate balances
		balances := []*balance.Balance{}
		for _, b := range out.Balances {
			// get token color
			if b.Color == "IOTA" {
				balances = append(balances, balance.New(balance.ColorIOTA, b.Value))
			} else {
				color, _, err := balance.ColorFromBytes([]byte(b.Color))
				if err != nil {
					log.Info(err.Error())
					return c.JSON(http.StatusBadRequest, Response{Error: err.Error()})
				}
				balances = append(balances, balance.New(color, b.Value))
			}
		}
		outmap[addr] = balances
	}
	outputs := transaction.NewOutputs(outmap)

	// prepare transaction
	// Note: not signed
	tx := transaction.New(inputs, outputs)

	// Prepare value payload and send the message to tangle
	payload, err := valuetransfers.ValueObjectFactory().IssueTransaction(tx)
	if err != nil {
		return c.JSON(http.StatusBadRequest, Response{Error: err.Error()})
	}
	_, err = issuer.IssuePayload(payload)
	if err != nil {
		return c.JSON(http.StatusBadRequest, Response{Error: err.Error()})
	}

	return c.JSON(http.StatusOK, Response{TransactionID: tx.ID().String()})
}

// Request holds the inputs and outputs to send.
type Request struct {
	Inputs  []string       `json:"inputs"`
	Outputs []utils.Output `json:"outputs"`
}

// Response is the HTTP response from sending transaction.
type Response struct {
	TransactionID string `json:"transaction_id,omitempty"`
	Error         string `json:"error,omitempty"`
}
