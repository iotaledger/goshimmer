package message

import (
	"net/http"

	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
	"github.com/labstack/echo"
)

// FindByIDHandler returns the array of messages for the
// given message ids (MUST be encoded in base58), in the same order as the parameters.
// If a node doesn't have the message for a given ID in its ledger,
// the value at the index of that message ID is empty.
// If an ID is not base58 encoded, an error is returned
func FindByIDHandler(c echo.Context) error {
	var request FindByIDRequest
	if err := c.Bind(&request); err != nil {
		log.Info(err.Error())
		return c.JSON(http.StatusBadRequest, FindByIDResponse{Error: err.Error()})
	}

	var result []Message
	for _, id := range request.IDs {
		log.Info("Received:", id)

		msgID, err := tangle.NewMessageID(id)
		if err != nil {
			log.Info(err)
			return c.JSON(http.StatusBadRequest, FindByIDResponse{Error: err.Error()})
		}

		msgObject := messagelayer.Tangle().Message(msgID)
		msgMetadataObject := messagelayer.Tangle().MessageMetadata(msgID)

		if !msgObject.Exists() || !msgMetadataObject.Exists() {
			result = append(result, Message{})
			continue
		}

		msg := msgObject.Unwrap()
		msgMetadata := msgMetadataObject.Unwrap()

		msgResp := Message{
			Metadata: Metadata{
				Solid:              msgMetadata.IsSolid(),
				SolidificationTime: msgMetadata.SolidificationTime().Unix(),
			},
			ID:              msg.ID().String(),
			Parent1ID:       msg.Parent1ID().String(),
			Parent2ID:       msg.Parent2ID().String(),
			IssuerPublicKey: msg.IssuerPublicKey().String(),
			IssuingTime:     msg.IssuingTime().Unix(),
			SequenceNumber:  msg.SequenceNumber(),
			Payload:         msg.Payload().Bytes(),
			Signature:       msg.Signature().String(),
		}
		result = append(result, msgResp)

		msgMetadataObject.Release()
		msgObject.Release()
	}

	return c.JSON(http.StatusOK, FindByIDResponse{Messages: result})
}

// FindByIDResponse is the HTTP response containing the queried messages.
type FindByIDResponse struct {
	Messages []Message `json:"messages,omitempty"`
	Error    string    `json:"error,omitempty"`
}

// FindByIDRequest holds the message ids to query.
type FindByIDRequest struct {
	IDs []string `json:"ids"`
}

// Message contains information about a given message.
type Message struct {
	Metadata        `json:"metadata,omitempty"`
	ID              string `json:"ID,omitempty"`
	Parent1ID       string `json:"parent1Id,omitempty"`
	Parent2ID       string `json:"parent2Id,omitempty"`
	IssuerPublicKey string `json:"issuerPublicKey,omitempty"`
	IssuingTime     int64  `json:"issuingTime,omitempty"`
	SequenceNumber  uint64 `json:"sequenceNumber,omitempty"`
	Payload         []byte `json:"payload,omitempty"`
	Signature       string `json:"signature,omitempty"`
}

// Metadata contains metadata information of a message.
type Metadata struct {
	Solid              bool  `json:"solid,omitempty"`
	SolidificationTime int64 `json:"solidificationTime,omitempty"`
}
