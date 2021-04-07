package value

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/iotaledger/hive.go/crypto/bls"
	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/identity"
	"github.com/labstack/echo"
	"github.com/mr-tron/base58/base58"

	"github.com/iotaledger/goshimmer/packages/clock"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/goshimmer/packages/tangle/payload"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
	"github.com/iotaledger/goshimmer/plugins/webapi/jsonmodels/value"
)

var (
	sendTxByJSONMu sync.Mutex

	// ErrMalformedIdentityID defines a malformed identityID error.
	ErrMalformedIdentityID = fmt.Errorf("malformed identityID")
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

	var request value.SendTransactionByJSONRequest
	if err := c.Bind(&request); err != nil {
		return c.JSON(http.StatusBadRequest, value.SendTransactionByJSONResponse{Error: err.Error()})
	}

	tx, err := NewTransactionFromJSON(request)
	if err != nil {
		return c.JSON(http.StatusBadRequest, value.SendTransactionByJSONResponse{Error: err.Error()})
	}

	// check balances validity
	consumedOutputs := make(ledgerstate.Outputs, len(tx.Essence().Inputs()))
	for i, consumedOutputID := range tx.Essence().Inputs() {
		referencedOutputID := consumedOutputID.(*ledgerstate.UTXOInput).ReferencedOutputID()
		messagelayer.Tangle().LedgerState.Output(referencedOutputID).Consume(func(output ledgerstate.Output) {
			consumedOutputs[i] = output
		})
	}
	if !ledgerstate.TransactionBalancesValid(consumedOutputs, tx.Essence().Outputs()) {
		return c.JSON(http.StatusBadRequest, value.SendTransactionResponse{Error: "sum of consumed and spent balances is not 0"})
	}

	// check unlock blocks validity
	if !ledgerstate.UnlockBlocksValid(consumedOutputs, tx) {
		return c.JSON(http.StatusBadRequest, value.SendTransactionResponse{Error: "spending of referenced consumedOutputs is not authorized"})
	}

	// check if transaction is too old
	if tx.Essence().Timestamp().Before(clock.SyncedTime().Add(-tangle.MaxReattachmentTimeMin)) {
		return c.JSON(http.StatusBadRequest, value.SendTransactionByJSONResponse{Error: fmt.Sprintf("transaction timestamp is older than MaxReattachmentTime (%s) and cannot be issued", tangle.MaxReattachmentTimeMin)})
	}

	// if transaction is in the future we wait until the time arrives
	if tx.Essence().Timestamp().After(clock.SyncedTime()) {
		time.Sleep(tx.Essence().Timestamp().Sub(clock.SyncedTime()) + 1*time.Nanosecond)
	}

	// send tx message
	issueTransaction := func() (*tangle.Message, error) {
		msg, e := messagelayer.Tangle().IssuePayload(tx)
		if e != nil {
			return nil, c.JSON(http.StatusBadRequest, value.SendTransactionResponse{Error: e.Error()})
		}
		return msg, nil
	}

	_, err = messagelayer.AwaitMessageToBeBooked(issueTransaction, tx.ID(), maxBookedAwaitTime)
	if err != nil {
		return c.JSON(http.StatusBadRequest, value.SendTransactionByJSONResponse{Error: err.Error()})
	}

	return c.JSON(http.StatusOK, value.SendTransactionByJSONResponse{TransactionID: tx.ID().Base58()})
}

// NewTransactionFromJSON returns a new transaction from a given JSON request or an error.
func NewTransactionFromJSON(request value.SendTransactionByJSONRequest) (*ledgerstate.Transaction, error) {
	// prepare inputs
	var inputs []ledgerstate.Input
	for _, input := range request.Inputs {
		in, err := ledgerstate.OutputIDFromBase58(input)
		if err != nil {
			return nil, ErrMalformedInputs
		}
		inputs = append(inputs, ledgerstate.NewUTXOInput(in))
	}

	// prepare outputs
	outputs := []ledgerstate.Output{}
	for _, output := range request.Outputs {
		outputType := output.Type
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
				} else if b.Color == "MINT" {
					color = ledgerstate.ColorMint
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

	aManaPledgeID, err := identity.ParseID(request.AManaPledgeID)
	if err != nil {
		return nil, ErrMalformedIdentityID
	}
	cManaPledgeID, err := identity.ParseID(request.CManaPledgeID)
	if err != nil {
		return nil, ErrMalformedIdentityID
	}

	txEssence := ledgerstate.NewTransactionEssence(
		0,
		time.Now(),
		aManaPledgeID,
		cManaPledgeID,
		ledgerstate.NewInputs(inputs...),
		ledgerstate.NewOutputs(outputs...),
	)

	// add data payload
	payload, _, err := payload.FromBytes(request.Payload)
	if err != nil {
		return nil, ErrMalformedData
	}

	txEssence.SetPayload(payload)

	// add signatures
	unlockBlocks := make([]ledgerstate.UnlockBlock, len(txEssence.Inputs()))
	for i, signature := range request.Signatures {
		switch ledgerstate.SignatureType(signature.Type) {
		case ledgerstate.ED25519SignatureType:
			pubKeyBytes, err := base58.Decode(signature.PublicKey)
			if err != nil || len(pubKeyBytes) != ed25519.PublicKeySize {
				return nil, ErrMalformedPublicKey
			}

			sig, err := ledgerstate.SignatureFromBase58EncodedString(signature.Signature)
			if err != nil {
				return nil, ErrMalformedSignature
			}

			unlockBlocks[i] = ledgerstate.NewSignatureUnlockBlock(sig)

		case ledgerstate.BLSSignatureType:
			pubKeyBytes, err := base58.Decode(signature.PublicKey)
			if err != nil || len(pubKeyBytes) != bls.PublicKeySize {
				return nil, ErrMalformedPublicKey
			}

			sig, err := ledgerstate.SignatureFromBase58EncodedString(signature.Signature)
			if err != nil {
				return nil, ErrMalformedSignature
			}

			unlockBlocks[i] = ledgerstate.NewSignatureUnlockBlock(sig)

		default:
			return nil, ErrSignatureVersion
		}
	}

	return ledgerstate.NewTransaction(txEssence, unlockBlocks), nil
}
