package dashboard

import (
	"fmt"
	"net/http"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
	valueutils "github.com/iotaledger/goshimmer/plugins/webapi/value"
	"github.com/labstack/echo"
	"github.com/mr-tron/base58/base58"
)

// ExplorerMessage defines the struct of the ExplorerMessage.
type ExplorerMessage struct {
	// ID is the message ID.
	ID string `json:"id"`
	// SolidificationTimestamp is the timestamp of the message.
	SolidificationTimestamp int64 `json:"solidification_timestamp"`
	// The time when this message was issued
	IssuanceTimestamp int64 `json:"issuance_timestamp"`
	// The issuer's sequence number of this message.
	SequenceNumber uint64 `json:"sequence_number"`
	// The public key of the issuer who issued this message.
	IssuerPublicKey string `json:"issuer_public_key"`
	// The signature of the message.
	Signature string `json:"signature"`
	// StrongParents are the strong parents (references) of the message.
	StrongParents []string `json:"strongParents"`
	// WeakParents are the weak parents (references) of the message.
	WeakParents []string `json:"weakParents"`
	// Solid defines the solid status of the message.
	Solid bool `json:"solid"`
	// PayloadType defines the type of the payload.
	PayloadType uint32 `json:"payload_type"`
	// Payload is the content of the payload.
	Payload interface{} `json:"payload"`
}

func createExplorerMessage(msg *tangle.Message) (*ExplorerMessage, error) {
	messageID := msg.ID()
	cachedMessageMetadata := messagelayer.Tangle().Storage.MessageMetadata(messageID)
	defer cachedMessageMetadata.Release()
	messageMetadata := cachedMessageMetadata.Unwrap()
	t := &ExplorerMessage{
		ID:                      messageID.String(),
		SolidificationTimestamp: messageMetadata.SolidificationTime().Unix(),
		IssuanceTimestamp:       msg.IssuingTime().Unix(),
		IssuerPublicKey:         msg.IssuerPublicKey().String(),
		Signature:               msg.Signature().String(),
		SequenceNumber:          msg.SequenceNumber(),
		StrongParents:           msg.StrongParents().ToStrings(),
		WeakParents:             msg.WeakParents().ToStrings(),
		Solid:                   cachedMessageMetadata.Unwrap().IsSolid(),
		PayloadType:             uint32(msg.Payload().Type()),
		Payload:                 ProcessPayload(msg.Payload()),
	}

	return t, nil
}

// ExplorerAddress defines the struct of the ExplorerAddress.
type ExplorerAddress struct {
	Address   string           `json:"address"`
	OutputIDs []ExplorerOutput `json:"output_ids"`
}

// ExplorerOutput defines the struct of the ExplorerOutput.
type ExplorerOutput struct {
	ID                 string                    `json:"id"`
	Balances           []valueutils.Balance      `json:"balances"`
	InclusionState     valueutils.InclusionState `json:"inclusion_state"`
	SolidificationTime int64                     `json:"solidification_time"`
	ConsumerCount      int                       `json:"consumer_count"`
}

// SearchResult defines the struct of the SearchResult.
type SearchResult struct {
	// Message is the *ExplorerMessage.
	Message *ExplorerMessage `json:"message"`
	// Address is the *ExplorerAddress.
	Address *ExplorerAddress `json:"address"`
}

func setupExplorerRoutes(routeGroup *echo.Group) {
	routeGroup.GET("/message/:id", func(c echo.Context) (err error) {
		messageID, err := tangle.NewMessageID(c.Param("id"))
		if err != nil {
			return
		}

		t, err := findMessage(messageID)
		if err != nil {
			return
		}

		return c.JSON(http.StatusOK, t)
	})

	routeGroup.GET("/address/:id", func(c echo.Context) error {
		addr, err := findAddress(c.Param("id"))
		if err != nil {
			return err
		}
		return c.JSON(http.StatusOK, addr)
	})

	routeGroup.GET("/search/:search", func(c echo.Context) error {
		search := c.Param("search")
		result := &SearchResult{}

		searchInByte, err := base58.Decode(search)
		if err != nil {
			return fmt.Errorf("%w: search ID %s", ErrInvalidParameter, search)
		}

		switch len(searchInByte) {

		case ledgerstate.AddressLength:
			addr, err := findAddress(search)
			if err == nil {
				result.Address = addr
			}

		case tangle.MessageIDLength:
			messageID, err := tangle.NewMessageID(search)
			if err != nil {
				return fmt.Errorf("%w: search ID %s", ErrInvalidParameter, search)
			}

			msg, err := findMessage(messageID)
			if err == nil {
				result.Message = msg
			}

		default:
			return fmt.Errorf("%w: search ID %s", ErrInvalidParameter, search)
		}

		return c.JSON(http.StatusOK, result)
	})
}

func findMessage(messageID tangle.MessageID) (explorerMsg *ExplorerMessage, err error) {
	if !messagelayer.Tangle().Storage.Message(messageID).Consume(func(msg *tangle.Message) {
		explorerMsg, err = createExplorerMessage(msg)
	}) {
		err = fmt.Errorf("%w: message %s", ErrNotFound, messageID.String())
	}

	return
}

func findAddress(strAddress string) (*ExplorerAddress, error) {

	address, err := ledgerstate.AddressFromBase58EncodedString(strAddress)
	if err != nil {
		return nil, fmt.Errorf("%w: address %s", ErrNotFound, strAddress)
	}

	outputids := make([]ExplorerOutput, 0)
	inclusionState := valueutils.InclusionState{}

	// get outputids by address
	messagelayer.Tangle().LedgerState.OutputsOnAddress(address).Consume(func(output ledgerstate.Output) {
		// iterate balances
		var b []valueutils.Balance
		output.Balances().ForEach(func(color ledgerstate.Color, balance uint64) bool {
			b = append(b, valueutils.Balance{
				Value: int64(balance),
				Color: color.String(),
			})
			return true
		})
		transactionID := output.ID().TransactionID()
		txInclusionState, _ := messagelayer.Tangle().LedgerState.TransactionInclusionState(transactionID)
		var consumerCount int
		var branch ledgerstate.Branch
		messagelayer.Tangle().LedgerState.OutputMetadata(output.ID()).Consume(func(outputMetadata *ledgerstate.OutputMetadata) {
			consumerCount = outputMetadata.ConsumerCount()
			messagelayer.Tangle().LedgerState.Branch(outputMetadata.BranchID()).Consume(func(b ledgerstate.Branch) {
				branch = b
			})
		})
		var solidificationTime int64
		messagelayer.Tangle().LedgerState.TransactionMetadata(transactionID).Consume(func(txMeta *ledgerstate.TransactionMetadata) {
			inclusionState.Confirmed = txInclusionState == ledgerstate.Confirmed
			inclusionState.Liked = branch.Liked()
			inclusionState.Rejected = txInclusionState == ledgerstate.Rejected
			inclusionState.Finalized = txMeta.Finalized()
			inclusionState.Conflicting = messagelayer.Tangle().LedgerState.TransactionConflicting(transactionID)
			solidificationTime = txMeta.SolidificationTime().Unix()
		})

		outputids = append(outputids, ExplorerOutput{
			ID:                 output.ID().String(),
			Balances:           b,
			InclusionState:     inclusionState,
			ConsumerCount:      consumerCount,
			SolidificationTime: solidificationTime,
		})
	})

	if len(outputids) == 0 {
		return nil, fmt.Errorf("%w: address %s", ErrNotFound, strAddress)
	}

	return &ExplorerAddress{
		Address:   strAddress,
		OutputIDs: outputids,
	}, nil

}
