package attachments

import (
	"net/http"

	"github.com/iotaledger/goshimmer/dapps/valuetransfers"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/address"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/balance"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/transaction"
	"github.com/labstack/echo"
	"github.com/labstack/gommon/log"
)

// Handler gets the value attachments.
func Handler(c echo.Context) error {
	txnID, err := transaction.IDFromBase58(c.QueryParam("txnID"))
	if err != nil {
		log.Info(err)
		return c.JSON(http.StatusBadRequest, Response{Error: err.Error()})
	}

	var valueObjs []ValueObject

	// get txn by txn id
	txnObj := valuetransfers.Tangle.Transaction(txnID)
	if !txnObj.Exists() {
		return c.JSON(http.StatusNotFound, Response{Error: "Transaction not found"})
	}
	txn := parseTransaction(txnObj.Unwrap())

	// get attachements by txn id
	for _, attachmentObj := range valuetransfers.Tangle.Attachments(txnID) {
		if !attachmentObj.Exists() {
			continue
		}
		attachment := attachmentObj.Unwrap()

		// get payload by payload id
		payloadObj := valuetransfers.Tangle.Payload(attachment.PayloadID())
		if !payloadObj.Exists() {
			continue
		}
		payload := payloadObj.Unwrap()

		// append value object
		valueObjs = append(valueObjs, ValueObject{
			ID:          payload.ID().String(),
			ParentID0:   payload.TrunkID().String(),
			ParentID1:   payload.BranchID().String(),
			Transaction: txn,
		})
	}

	return c.JSON(http.StatusOK, Response{Attachments: valueObjs})
}

func parseTransaction(t *transaction.Transaction) (txn Transaction) {
	var inputs []string
	var outputs []Output
	// process inputs
	t.Inputs().ForEachAddress(func(currentAddress address.Address) bool {
		inputs = append(inputs, currentAddress.String())
		return true
	})

	// process outputs: address + balance
	t.Outputs().ForEach(func(address address.Address, balances []*balance.Balance) bool {
		var b []Balance
		for _, balance := range balances {
			b = append(b, Balance{
				Value: balance.Value(),
				Color: balance.Color().String(),
			})
		}
		t := Output{
			Address:  address.String(),
			Balances: b,
		}
		outputs = append(outputs, t)

		return true
	})

	return Transaction{
		Inputs:      inputs,
		Outputs:     outputs,
		Signature:   t.SignatureBytes(),
		DataPayload: t.GetDataPayload(),
	}
}

// Response is the HTTP response from retreiving value objects.
type Response struct {
	Attachments []ValueObject `json:"attachments,omitempty"`
	Error       string        `json:"error,omitempty"`
}

// ValueObject holds the information of a value object.
type ValueObject struct {
	ID          string `json:"id"`
	ParentID0   string `json:"parent0_id"`
	ParentID1   string `json:"parent1_id"`
	Transaction `json:"transaction"`
}

// Transaction holds the information of a transaction.
type Transaction struct {
	Inputs      []string `json:"inputs"`
	Outputs     []Output `json:"outputs"`
	Signature   []byte   `json:"signature"`
	DataPayload []byte   `json:"data_payload"`
}

// Output consists an address and balances
type Output struct {
	Address  string    `json:"address"`
	Balances []Balance `json:"balances"`
}

// Balance holds the value and the color of token
type Balance struct {
	Value int64  `json:"value"`
	Color string `json:"color"`
}
