package sendtransactionbyjson

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/iotaledger/goshimmer/dapps/valuetransfers"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/address"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/address/signaturescheme"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/balance"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/transaction"
	"github.com/iotaledger/goshimmer/packages/mana"
	"github.com/iotaledger/goshimmer/plugins/autopeering/local"
	"github.com/iotaledger/goshimmer/plugins/issuer"
	manaPlugin "github.com/iotaledger/goshimmer/plugins/mana"
	"github.com/iotaledger/goshimmer/plugins/webapi/value/utils"
	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/identity"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/labstack/echo"
	"github.com/mr-tron/base58/base58"
)

var (
	sendTxMu           sync.Mutex
	maxBookedAwaitTime = 5 * time.Second

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
	// ErrNotAllowedToPledgeManaToNode defines an unsupported node to pledge mana to.
	ErrNotAllowedToPledgeManaToNode = fmt.Errorf("not allowed to pledge mana to node")
)

// Handler sends a transaction.
func Handler(c echo.Context) error {
	sendTxMu.Lock()
	defer sendTxMu.Unlock()

	var request Request
	if err := c.Bind(&request); err != nil {
		return c.JSON(http.StatusBadRequest, Response{Error: err.Error()})
	}

	tx, err := NewTransactionFromJSON(request)
	if err != nil {
		return c.JSON(http.StatusBadRequest, Response{Error: err.Error()})
	}

	// validate transaction
	err = valuetransfers.Tangle().ValidateTransactionToAttach(tx)
	if err != nil {
		return c.JSON(http.StatusBadRequest, Response{Error: err.Error()})
	}

	// Prepare value payload and send the message to tangle
	payload, err := valuetransfers.ValueObjectFactory().IssueTransaction(tx)
	if err != nil {
		return c.JSON(http.StatusBadRequest, Response{Error: err.Error()})
	}
	_, err = issuer.IssuePayload(payload)
	if err != nil {
		return c.JSON(http.StatusBadRequest, Response{Error: err.Error()})
	}

	if err := valuetransfers.AwaitTransactionToBeBooked(tx.ID(), maxBookedAwaitTime); err != nil {
		return c.JSON(http.StatusBadRequest, Response{Error: err.Error()})
	}
	return c.JSON(http.StatusOK, Response{TransactionID: tx.ID().String()})
}

// NewTransactionFromJSON returns a new transaction from a given JSON request or an error.
func NewTransactionFromJSON(request Request) (*transaction.Transaction, error) {
	// prepare inputs
	inputs := make([]transaction.OutputID, len(request.Inputs))
	for i, input := range request.Inputs {
		b, err := base58.Decode(input)
		if err != nil || len(b) != transaction.OutputIDLength {
			return nil, ErrMalformedInputs
		}
		copy(inputs[i][:], b)
	}

	// prepare ouputs
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
	emptyID := identity.ID{}

	pledgeAccessManaNode, err := mana.IDFromStr(request.AccessMana)
	if err != nil {
		return nil, err
	}
	if pledgeAccessManaNode == emptyID {
		pledgeAccessManaNode = local.GetInstance().ID()
	}
	allowedAccessMana := manaPlugin.GetAllowedPledgeNodes(mana.AccessMana)
	if allowedAccessMana.IsFilterEnabled {
		if !allowedAccessMana.Allowed.Has(pledgeAccessManaNode) {
			return nil, fmt.Errorf("not allowed to pledge access mana to %s: %w", request.AccessMana, ErrNotAllowedToPledgeManaToNode)
		}
	}
	tx.SetAccessManaNodeID(pledgeAccessManaNode)

	pledgeConsensusManaNode, err := mana.IDFromStr(request.ConsensusMana)
	if err != nil {
		return nil, err
	}
	if pledgeConsensusManaNode == emptyID {
		pledgeConsensusManaNode = local.GetInstance().ID()
	}
	allowedConsensusMana := manaPlugin.GetAllowedPledgeNodes(mana.ConsensusMana)
	if allowedConsensusMana.IsFilterEnabled {
		if !allowedConsensusMana.Allowed.Has(pledgeConsensusManaNode) {
			return nil, fmt.Errorf("not allowed to pledge consensus mana to %s: %w", request.ConsensusMana, ErrNotAllowedToPledgeManaToNode)
		}
	}
	tx.SetConsensusManaNodeID(pledgeConsensusManaNode)
	tx.SetTimestamp(time.Unix(0, request.Timestamp))

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

// Request holds the transaction object(json) to send.
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
type Request struct {
	Inputs        []string    `json:"inputs"`
	Outputs       []Output    `json:"outputs"`
	Data          []byte      `json:"data,omitempty"`
	Signatures    []Signature `json:"signatures"`
	AccessMana    string      `json:"accessMana"`
	ConsensusMana string      `json:"consensusMana"`
	Timestamp     int64       `json:"timestamp"`
}

// Output defines the struct of an output.
type Output struct {
	Address  string          `json:"address"`
	Balances []utils.Balance `json:"balances"`
}

// Signature defines the struct of a signature.
type Signature struct {
	Version   byte   `json:"version"`
	PublicKey string `json:"publicKey"`
	Signature string `json:"signature"`
}

// Response is the HTTP response from sending transaction.
type Response struct {
	TransactionID string `json:"transaction_id,omitempty"`
	Error         string `json:"error,omitempty"`
}
