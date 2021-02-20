package value

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/plugins/issuer"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
	"github.com/iotaledger/hive.go/crypto/bls"
	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/identity"
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

	// send tx message
	msg, err := issuer.IssuePayload(tx, messagelayer.Tangle())
	if err != nil {
		return c.JSON(http.StatusBadRequest, SendTransactionByJSONResponse{Error: err.Error()})
	}

	return c.JSON(http.StatusOK, SendTransactionByJSONResponse{TransactionID: tx.ID().String(), MessageID: msg.ID().String()})
}

// NewTransactionFromJSON returns a new transaction from a given JSON request or an error.
func NewTransactionFromJSON(request SendTransactionByJSONRequest) (*ledgerstate.Transaction, error) {
	// prepare inputs
	inputs := make(ledgerstate.Inputs, len(request.Inputs))
	for i, input := range request.Inputs {
		b, err := base58.Decode(input)
		in, _, err := ledgerstate.InputFromBytes(b)
		if err != nil {
			return nil, ErrMalformedInputs
		}
		inputs[i] = in
	}

	// prepare outputs
	outputs := []ledgerstate.Output{}
	for _, output := range request.Outputs {
		outputType := ledgerstate.OutputType(output.Type)
		address, err := ledgerstate.AddressFromBase58EncodedString(output.Address)
		if err != nil {
			return nil, ErrMalformedOutputs
		}

		switch outputType {
		case ledgerstate.SigLockedSingleOutputType:
			o := ledgerstate.NewSigLockedSingleOutput(uint64(output.Balances[0].Value), address)
			outputs = append(outputs, o)

		case ledgerstate.SigLockedColoredOutputType:
			balances := make(map[ledgerstate.Color]uint64)
			for _, b := range output.Balances {
				var color ledgerstate.Color
				if b.Color == "IOTA" {
					color = ledgerstate.ColorIOTA
				} else {
					color, err = ledgerstate.ColorFromBase58EncodedString(b.Color)
					if err != nil {
						return nil, ErrMalformedColor
					}
				}
				balances[color] += uint64(b.Value)
			}
			o := ledgerstate.NewSigLockedColoredOutput(ledgerstate.NewColoredBalances(balances), address)
			outputs = append(outputs, o)

		default:
			return nil, ErrMalformedOutputs
		}
	}

	txEssence := ledgerstate.NewTransactionEssence(
		0,
		time.Now(),
		identity.ID{},
		identity.ID{},
		ledgerstate.NewInputs(inputs...),
		ledgerstate.NewOutputs(outputs...),
	)

	// add signatures
	unlockBlocks := make([]ledgerstate.UnlockBlock, len(txEssence.Inputs()))
	for i, signature := range request.Signatures {
		switch ledgerstate.SignatureType(signature.Version) {

		case ledgerstate.ED25519SignatureType:
			pubKeyBytes, err := base58.Decode(signature.PublicKey)
			if err != nil || len(pubKeyBytes) != ed25519.PublicKeySize {
				return nil, ErrMalformedPublicKey
			}

			signatureBytes, err := base58.Decode(signature.Signature)
			if err != nil || len(signatureBytes) != ed25519.SignatureSize {
				return nil, ErrMalformedSignature
			}

			marshalUtil := marshalutil.New()
			marshalUtil.WriteByte(byte(ledgerstate.ED25519SignatureType))
			marshalUtil.WriteBytes(pubKeyBytes[:])
			marshalUtil.WriteBytes(signatureBytes[:])
			sign, _, err := ledgerstate.ED25519SignatureFromBytes(marshalUtil.Bytes())
			if err != nil {
				return nil, ErrWrongSignature
			}

			unlockBlocks[i] = ledgerstate.NewSignatureUnlockBlock(sign)

		case ledgerstate.BLSSignatureType:
			pubKeyBytes, err := base58.Decode(signature.PublicKey)
			if err != nil || len(pubKeyBytes) != bls.PublicKeySize {
				return nil, ErrMalformedPublicKey
			}

			signatureBytes, err := base58.Decode(signature.Signature)
			if err != nil || len(signatureBytes) != bls.SignatureSize {
				return nil, ErrMalformedSignature
			}

			marshalUtil := marshalutil.New()
			marshalUtil.WriteByte(byte(ledgerstate.BLSSignatureType))
			marshalUtil.WriteBytes(pubKeyBytes[:])
			marshalUtil.WriteBytes(signatureBytes[:])

			sign, _, err := ledgerstate.BLSSignatureFromBytes(marshalUtil.Bytes())
			if err != nil {
				return nil, ErrWrongSignature
			}
			unlockBlocks[i] = ledgerstate.NewSignatureUnlockBlock(sign)

		default:
			return nil, ErrSignatureVersion
		}
	}

	return ledgerstate.NewTransaction(txEssence, unlockBlocks), nil
}

// SendTransactionByJSONRequest holds the transaction object(json) to send.
// e.g.,
// {
// 	"inputs": string[],
// 	"outputs": {
//	   "type": number,
// 	   "address": string,
// 	   "balances": {
// 		   "value": number,
// 		   "color": string
// 	   }[];
// 	 }[],
// 	 "signatures": {
// 		"version": number,
// 		"publicKey": string,
// 		"signature": string
// 	   }[]
//  }
type SendTransactionByJSONRequest struct {
	Inputs     []string    `json:"inputs"`
	Outputs    []Output    `json:"outputs"`
	Signatures []Signature `json:"signatures"`
}

// SendTransactionByJSONResponse is the HTTP response from sending transaction.
type SendTransactionByJSONResponse struct {
	TransactionID string `json:"transaction_id,omitempty"`
	MessageID     string `json:"message_id,omitempty"`
	Error         string `json:"error,omitempty"`
}
