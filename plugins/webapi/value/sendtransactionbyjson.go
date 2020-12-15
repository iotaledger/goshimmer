package value

import (
	"fmt"
	"net/http"
	"sync"

	"github.com/iotaledger/goshimmer/dapps/valuetransfers"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/address"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/address/signaturescheme"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/balance"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/transaction"
	"github.com/iotaledger/goshimmer/plugins/issuer"
	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/labstack/echo"
	"github.com/mr-tron/base58/base58"
)

var (
	sendTxByJSONMu sync.Mutex

	// ErrMalformedInputs defines a malformed inputs error.
	ErrMalformedInputs = fmt.Errorf("malformed inputs")
	// ErrMalformedOutputs defines a malformed outputs error.
	ErrMalformedOutputs = fmt.Errorf("malformed outputs")
	// ErrMalformedData defines a malformed data error.
	ErrMalformedData = fmt.Errorf("malformed data")
	// ErrMalformedColor defines a malformed color error.
	ErrMalformedColor = fmt.Errorf("malformed color")
	// ErrMalformedPublicKey defines a malformed publicKey error.
	ErrMalformedPublicKey = fmt.Errorf("malformed publicKey")
	// ErrMalformedSignature defines a malformed signature error.
	ErrMalformedSignature = fmt.Errorf("malformed signature")
	// ErrWrongSignature defines a wrong signature error.
	ErrWrongSignature = fmt.Errorf("wrong signature")
	// ErrSignatureVersion defines a unsupported signature version error.
	ErrSignatureVersion = fmt.Errorf("unsupported signature version")
)

// sendTransactionByJSONHandler sends a transaction.
func sendTransactionByJSONHandler(c echo.Context) error {
	sendTxByJSONMu.Lock()
	defer sendTxByJSONMu.Unlock()

	var request SendTransactionByJSONRequest
	if err := c.Bind(&request); err != nil {
		return c.JSON(http.StatusBadRequest, SendTransactionByJSONResponse{Error: err.Error()})
	}

	tx, err := NewTransactionFromJSON(request)
	if err != nil {
		return c.JSON(http.StatusBadRequest, SendTransactionByJSONResponse{Error: err.Error()})
	}

	// validate transaction
	err = valuetransfers.Tangle().ValidateTransactionToAttach(tx)
	if err != nil {
		return c.JSON(http.StatusBadRequest, SendTransactionByJSONResponse{Error: err.Error()})
	}

	// Prepare value payload and send the message to tangle
	payload, err := valuetransfers.ValueObjectFactory().IssueTransaction(tx)
	if err != nil {
		return c.JSON(http.StatusBadRequest, SendTransactionByJSONResponse{Error: err.Error()})
	}
	_, err = issuer.IssuePayload(payload)
	if err != nil {
		return c.JSON(http.StatusBadRequest, SendTransactionByJSONResponse{Error: err.Error()})
	}

	if err := valuetransfers.AwaitTransactionToBeBooked(tx.ID(), maxBookedAwaitTime); err != nil {
		return c.JSON(http.StatusBadRequest, SendTransactionByJSONResponse{Error: err.Error()})
	}
	return c.JSON(http.StatusOK, SendTransactionByJSONResponse{TransactionID: tx.ID().String()})
}

// NewTransactionFromJSON returns a new transaction from a given JSON request or an error.
func NewTransactionFromJSON(request SendTransactionByJSONRequest) (*transaction.Transaction, error) {
	// prepare inputs
	inputs := make([]transaction.OutputID, len(request.Inputs))
	for i, input := range request.Inputs {
		b, err := base58.Decode(input)
		if err != nil || len(b) != transaction.OutputIDLength {
			return nil, ErrMalformedInputs
		}
		copy(inputs[i][:], b)
	}

	// prepare outputs
	outputs := make(map[address.Address][]*balance.Balance)
	for _, output := range request.Outputs {
		address, err := address.FromBase58(output.Address)
		if err != nil {
			return nil, ErrMalformedOutputs
		}

		balances := []*balance.Balance{}
		for _, b := range output.Balances {
			var color balance.Color
			if b.Color == "IOTA" {
				color = balance.ColorIOTA
			} else {
				colorBytes, err := base58.Decode(b.Color)
				if err != nil || len(colorBytes) != balance.ColorLength {
					return nil, ErrMalformedColor
				}
				copy(color[:], colorBytes)
			}
			balances = append(balances, &balance.Balance{
				Value: b.Value,
				Color: color,
			})
		}

		outputs[address] = balances
	}

	// prepare transaction
	tx := transaction.New(transaction.NewInputs(inputs...), transaction.NewOutputs(outputs))

	// add data payload
	if request.Data != nil {
		tx.SetDataPayload(request.Data)
	}

	// add signatures
	for _, signature := range request.Signatures {
		switch signature.Version {

		case address.VersionED25519:
			pubKeyBytes, err := base58.Decode(signature.PublicKey)
			if err != nil || len(pubKeyBytes) != ed25519.PublicKeySize {
				return nil, ErrMalformedPublicKey
			}

			signatureBytes, err := base58.Decode(signature.Signature)
			if err != nil || len(signatureBytes) != ed25519.SignatureSize {
				return nil, ErrMalformedSignature
			}

			marshalUtil := marshalutil.New(1 + ed25519.PublicKeySize + ed25519.SignatureSize)
			marshalUtil.WriteByte(address.VersionED25519)
			marshalUtil.WriteBytes(pubKeyBytes[:])
			marshalUtil.WriteBytes(signatureBytes[:])

			sign, _, err := signaturescheme.Ed25519SignatureFromBytes(marshalUtil.Bytes())
			if err != nil {
				return nil, ErrWrongSignature
			}
			err = tx.PutSignature(sign)
			if err != nil {
				return nil, ErrWrongSignature
			}

		case address.VersionBLS:
			pubKeyBytes, err := base58.Decode(signature.PublicKey)
			if err != nil || len(pubKeyBytes) != signaturescheme.BLSPublicKeySize {
				return nil, ErrMalformedPublicKey
			}

			signatureBytes, err := base58.Decode(signature.Signature)
			if err != nil || len(signatureBytes) != signaturescheme.BLSSignatureSize {
				return nil, ErrMalformedSignature
			}

			marshalUtil := marshalutil.New(signaturescheme.BLSFullSignatureSize)
			marshalUtil.WriteByte(address.VersionBLS)
			marshalUtil.WriteBytes(pubKeyBytes[:])
			marshalUtil.WriteBytes(signatureBytes[:])

			sign, _, err := signaturescheme.BLSSignatureFromBytes(marshalUtil.Bytes())
			if err != nil {
				return nil, ErrWrongSignature
			}
			err = tx.PutSignature(sign)
			if err != nil {
				return nil, ErrWrongSignature
			}

		default:
			return nil, ErrSignatureVersion
		}
	}

	return tx, nil
}

// SendTransactionByJSONRequest holds the transaction object(json) to send.
// e.g.,
// {
// 	"inputs": string[],
// 	"outputs": {
// 	   "address": string,
// 	   "balances": {
// 		   "value": number,
// 		   "color": string
// 	   }[];
// 	 }[],
// 	 "data": []byte,
// 	 "signatures": {
// 		"version": number,
// 		"publicKey": string,
// 		"signature": string
// 	   }[]
//  }
type SendTransactionByJSONRequest struct {
	Inputs     []string    `json:"inputs"`
	Outputs    []Output    `json:"outputs"`
	Data       []byte      `json:"data,omitempty"`
	Signatures []Signature `json:"signatures"`
}

// SendTransactionByJSONResponse is the HTTP response from sending transaction.
type SendTransactionByJSONResponse struct {
	TransactionID string `json:"transaction_id,omitempty"`
	Error         string `json:"error,omitempty"`
}
