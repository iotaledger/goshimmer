package message

import (
	"fmt"
	"net/http"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/markers"
	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
	"github.com/iotaledger/goshimmer/plugins/webapi"
	"github.com/labstack/echo"
)

// region API endpoints ////////////////////////////////////////////////////////////////////////////////////////////////

// GetMessageEndPoint is the handler for the /messages/:messageID endpoint.
func GetMessageEndPoint(c echo.Context) (err error) {
	messageID, err := messageIDFromContext(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, webapi.NewErrorResponse(err))
	}

	if messagelayer.Tangle().Storage.Message(messageID).Consume(func(message *tangle.Message) {
		err = c.JSON(http.StatusOK, NewMessage(message))
	}) {
		return
	}

	return c.JSON(http.StatusBadRequest, webapi.NewErrorResponse(fmt.Errorf("failed to load Message with %s", messageID)))
}

// GetMessageMetadataEndPoint is the handler for the /messages/:messageID/metadata endpoint.
func GetMessageMetadataEndPoint(c echo.Context) (err error) {
	messageID, err := messageIDFromContext(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, webapi.NewErrorResponse(err))
	}

	if messagelayer.Tangle().Storage.MessageMetadata(messageID).Consume(func(messageMetadata *tangle.MessageMetadata) {
		err = c.JSON(http.StatusOK, NewMessageMetadata(messageMetadata))
	}) {
		return
	}

	return c.JSON(http.StatusBadRequest, webapi.NewErrorResponse(fmt.Errorf("failed to load MessageMetadata with %s", messageID)))
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Message ///////////////////////////////////////////////////////////////////////////////////////////////////////

// Message contains information about a given message.
type Message struct {
	ID              string   `json:"id"`
	StrongParents   []string `json:"strongParents"`
	WeakParents     []string `json:"weakParents"`
	StrongApprovers []string `json:"strongApprovers"`
	WeakApprovers   []string `json:"weakApprovers"`
	IssuerPublicKey string   `json:"issuerPublicKey"`
	IssuingTime     int64    `json:"issuingTime"`
	SequenceNumber  uint64   `json:"sequenceNumber"`
	PayloadType     string   `json:"payloadType"`
	TransactionID   string   `json:"transactionID,omitempty"`
	Payload         []byte   `json:"payload"`
	Signature       string   `json:"signature"`
}

// NewMessage returns a Message from the given pointer to a tangle.Message.
func NewMessage(message *tangle.Message) Message {
	return Message{
		ID:              message.ID().String(),
		StrongParents:   message.StrongParents().ToStrings(),
		WeakParents:     message.WeakParents().ToStrings(),
		StrongApprovers: messagelayer.Tangle().Utils.ApprovingMessageIDs(message.ID(), tangle.StrongApprover).ToStrings(),
		WeakApprovers:   messagelayer.Tangle().Utils.ApprovingMessageIDs(message.ID(), tangle.WeakApprover).ToStrings(),
		IssuerPublicKey: message.IssuerPublicKey().String(),
		IssuingTime:     message.IssuingTime().Unix(),
		SequenceNumber:  message.SequenceNumber(),
		PayloadType:     message.Payload().Type().String(),
		TransactionID: func() string {
			if message.Payload().Type() == ledgerstate.TransactionType {
				return message.Payload().(*ledgerstate.Transaction).ID().Base58()
			}

			return ""
		}(),
		Payload:   message.Payload().Bytes(),
		Signature: message.Signature().String(),
	}
}

// messageIDFromContext determines the MessageID from the messageID parameter in an echo.Context. It expects it to either
// be a base58 encoded string or one of the builtin aliases (EmptyMessageID)
func messageIDFromContext(c echo.Context) (messageID tangle.MessageID, err error) {
	switch messageIDString := c.Param("messageID"); messageIDString {
	case "EmptyMessageID":
		messageID = tangle.EmptyMessageID
	default:
		messageID, err = tangle.NewMessageID(messageIDString)
	}

	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region MessageMetadata ///////////////////////////////////////////////////////////////////////////////////////////////////////

// MessageMetadata contains metadata information of a message.
type MessageMetadata struct {
	ID                 string           `json:"id"`
	ReceivedTime       int64            `json:"receivedTime"`
	Solid              bool             `json:"solid"`
	SolidificationTime int64            `json:"solidificationTime"`
	StructureDetails   StructureDetails `json:"structureDetails"`
	BranchID           string           `json:"branchID"`
	Scheduled          bool             `json:"scheduled"`
	Booked             bool             `json:"booked"`
	Eligible           bool             `json:"eligible"`
	Invalid            bool             `json:"invalid"`
}

// StructureDetails represents a container for the complete Marker related information of a node in a DAG that are used
// to interact with the public API of this package.
type StructureDetails struct {
	Rank          uint64  `json:"rank"`
	IsPastMarker  bool    `json:"isPastMarker"`
	PastMarkers   Markers `json:"pastMarkers"`
	FutureMarkers Markers `json:"futureMarkers"`
}

// Markers represents a collection of Markers that can contain exactly one Index per SequenceID.
type Markers struct {
	Markers      map[markers.SequenceID]markers.Index `json:"markers"`
	HighestIndex markers.Index                        `json:"highestIndex"`
	LowestIndex  markers.Index                        `json:"lowestIndex"`
}

// NewMessageMetadata returns a MessageMetadata from the given pointer to a tangle.MessageMetadata.
func NewMessageMetadata(metadata *tangle.MessageMetadata) MessageMetadata {
	return MessageMetadata{
		ID:                 metadata.ID().String(),
		ReceivedTime:       metadata.ReceivedTime().Unix(),
		Solid:              metadata.IsSolid(),
		SolidificationTime: metadata.SolidificationTime().Unix(),
		StructureDetails: func() StructureDetails {
			var structureDetails StructureDetails
			if metadata.StructureDetails() != nil {
				structureDetails = StructureDetails{
					Rank:          metadata.StructureDetails().Rank,
					IsPastMarker:  metadata.StructureDetails().IsPastMarker,
					PastMarkers:   newMarkers(metadata.StructureDetails().PastMarkers),
					FutureMarkers: newMarkers(metadata.StructureDetails().FutureMarkers),
				}
			}
			return structureDetails
		}(),
		BranchID:  metadata.BranchID().String(),
		Scheduled: metadata.Scheduled(),
		Booked:    metadata.IsBooked(),
		Eligible:  metadata.IsEligible(),
		Invalid:   metadata.IsInvalid(),
	}
}

func newMarkers(m *markers.Markers) (newMarkers Markers) {
	newMarkers.Markers = make(map[markers.SequenceID]markers.Index)
	m.ForEach(func(sequenceID markers.SequenceID, index markers.Index) bool {
		newMarkers.Markers[sequenceID] = index
		return true
	})
	newMarkers.HighestIndex = m.HighestIndex()
	newMarkers.LowestIndex = m.LowestIndex()
	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
