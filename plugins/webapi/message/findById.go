package message

import (
	"net/http"

	"github.com/iotaledger/goshimmer/packages/markers"
	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
	"github.com/labstack/echo"
)

// findByIDHandler returns the array of messages for the
// given message ids (MUST be encoded in base58), in the same order as the parameters.
// If a node doesn't have the message for a given ID in its ledger,
// the value at the index of that message ID is empty.
// If an ID is not base58 encoded, an error is returned
func findByIDHandler(c echo.Context) error {
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

		msgObject := messagelayer.Tangle().Storage.Message(msgID)
		msgMetadataObject := messagelayer.Tangle().Storage.MessageMetadata(msgID)

		if !msgObject.Exists() || !msgMetadataObject.Exists() {
			result = append(result, Message{})

			msgObject.Release()
			msgMetadataObject.Release()

			continue
		}

		msg := msgObject.Unwrap()
		msgMetadata := msgMetadataObject.Unwrap()

		msgResp := Message{
			Metadata: Metadata{
				ReceivedTime:       msgMetadata.ReceivedTime().Unix(),
				Solid:              msgMetadata.IsSolid(),
				SolidificationTime: msgMetadata.SolidificationTime().Unix(),
				StructureDetails: StructureDetails{
					Rank:          msgMetadata.StructureDetails().Rank,
					IsPastMarker:  msgMetadata.StructureDetails().IsPastMarker,
					PastMarkers:   newMarkers(msgMetadata.StructureDetails().PastMarkers),
					FutureMarkers: newMarkers(msgMetadata.StructureDetails().FutureMarkers),
				},
				BranchID:  msgMetadata.BranchID().String(),
				Scheduled: msgMetadata.Scheduled(),
				Booked:    msgMetadata.IsBooked(),
				Eligible:  msgMetadata.IsEligible(),
				Invalid:   msgMetadata.IsInvalid(),
			},
			ID:              msg.ID().String(),
			StrongParents:   msg.StrongParents().ToStrings(),
			WeakParents:     msg.WeakParents().ToStrings(),
			IssuerPublicKey: msg.IssuerPublicKey().String(),
			IssuingTime:     msg.IssuingTime().Unix(),
			SequenceNumber:  msg.SequenceNumber(),
			PayloadType:     msg.Payload().Type().String(),
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
	Metadata        `json:"metadata"`
	ID              string   `json:"ID"`
	StrongParents   []string `json:"strongParents"`
	WeakParents     []string `json:"weakParents"`
	IssuerPublicKey string   `json:"issuerPublicKey"`
	IssuingTime     int64    `json:"issuingTime"`
	SequenceNumber  uint64   `json:"sequenceNumber"`
	PayloadType     string   `json:"payloadType"`
	Payload         []byte   `json:"payload"`
	Signature       string   `json:"signature"`
}

// Metadata contains metadata information of a message.
type Metadata struct {
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
