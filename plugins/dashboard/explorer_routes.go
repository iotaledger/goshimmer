package dashboard

import (
	"fmt"
	"net/http"
	"sync"

	"github.com/iotaledger/goshimmer/dapps/valuetransfers"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/address"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/tangle"
	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/message"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
	"github.com/iotaledger/goshimmer/plugins/webapi/value/utils"
	"github.com/labstack/echo"
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
	// TrunkMessageId is the Trunk ID of the message.
	TrunkMessageID string `json:"trunk_message_id"`
	// BranchMessageId is the Branch ID of the message.
	BranchMessageID string `json:"branch_message_id"`
	// Solid defines the solid status of the message.
	Solid bool `json:"solid"`
	// PayloadType defines the type of the payload.
	PayloadType uint32 `json:"payload_type"`
	// Payload is the content of the payload.
	Payload interface{} `json:"payload"`
}

func createExplorerMessage(msg *message.Message) (*ExplorerMessage, error) {
	messageID := msg.Id()
	cachedMessageMetadata := messagelayer.Tangle().MessageMetadata(messageID)
	defer cachedMessageMetadata.Release()
	messageMetadata := cachedMessageMetadata.Unwrap()
	t := &ExplorerMessage{
		ID:                      messageID.String(),
		SolidificationTimestamp: messageMetadata.SolidificationTime().Unix(),
		IssuanceTimestamp:       msg.IssuingTime().Unix(),
		IssuerPublicKey:         msg.IssuerPublicKey().String(),
		Signature:               msg.Signature().String(),
		SequenceNumber:          msg.SequenceNumber(),
		TrunkMessageID:          msg.TrunkId().String(),
		BranchMessageID:         msg.BranchId().String(),
		Solid:                   cachedMessageMetadata.Unwrap().IsSolid(),
		PayloadType:             msg.Payload().Type(),
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
	ID                 string               `json:"id"`
	Balances           []utils.Balance      `json:"balances"`
	InclusionState     utils.InclusionState `json:"inclusion_state"`
	SolidificationTime int64                `json:"solidification_time"`
	ConsumerCount      int                  `json:"consumer_count"`
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
		messageID, err := message.NewId(c.Param("id"))
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
		wg := sync.WaitGroup{}

		switch len(search) {

		case address.Length:
			wg.Add(1)
			go func() {
				defer wg.Done()
				addr, err := findAddress(search)
				if err == nil {
					result.Address = addr
				}
			}()

		case message.IdLength:
			wg.Add(1)
			go func() {
				defer wg.Done()

				messageID, err := message.NewId(search)
				if err != nil {
					return
				}

				msg, err := findMessage(messageID)
				if err == nil {
					result.Message = msg
				}
			}()

		default:
			return fmt.Errorf("%w: search ID %s", ErrInvalidParameter, search)
		}

		wg.Wait()

		return c.JSON(http.StatusOK, result)
	})
}

func findMessage(messageID message.Id) (explorerMsg *ExplorerMessage, err error) {
	if !messagelayer.Tangle().Message(messageID).Consume(func(msg *message.Message) {
		explorerMsg, err = createExplorerMessage(msg)
	}) {
		err = fmt.Errorf("%w: message %s", ErrNotFound, messageID.String())
	}

	return
}

func findAddress(strAddress string) (*ExplorerAddress, error) {

	address, err := address.FromBase58(strAddress)
	if err != nil {
		return nil, fmt.Errorf("%w: address %s", ErrNotFound, strAddress)
	}

	outputids := make([]ExplorerOutput, 0)
	// get outputids by address
	for id, cachedOutput := range valuetransfers.Tangle().OutputsOnAddress(address) {

		cachedOutput.Consume(func(output *tangle.Output) {

			// iterate balances
			var b []utils.Balance
			for _, balance := range output.Balances() {
				b = append(b, utils.Balance{
					Value: balance.Value,
					Color: balance.Color.String(),
				})
			}

			valuetransfers.Tangle().TransactionMetadata(output.TransactionID()).Consume(func(txMeta *tangle.TransactionMetadata) {

				inclusionState := utils.InclusionState{}
				inclusionState.Confirmed = txMeta.Confirmed()
				inclusionState.Liked = txMeta.Liked()
				inclusionState.Rejected = txMeta.Rejected()
				inclusionState.Finalized = txMeta.Finalized()
				inclusionState.Conflicting = txMeta.Conflicting()
				inclusionState.Confirmed = txMeta.Confirmed()

				outputids = append(outputids, ExplorerOutput{
					ID:                 id.String(),
					Balances:           b,
					InclusionState:     inclusionState,
					ConsumerCount:      output.ConsumerCount(),
					SolidificationTime: txMeta.SolidificationTime().Unix(),
				})
			})
		})
	}

	if len(outputids) == 0 {
		return nil, fmt.Errorf("%w: address %s", ErrNotFound, strAddress)
	}

	return &ExplorerAddress{
		Address:   strAddress,
		OutputIDs: outputids,
	}, nil

}
