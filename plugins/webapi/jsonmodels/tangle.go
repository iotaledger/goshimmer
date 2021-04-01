package jsonmodels

import (
	"github.com/iotaledger/goshimmer/packages/tangle"
)

// region Message ///////////////////////////////////////////////////////////////////////////////////////////////////////

// Message represents the JSON model of a tangle.Message.
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

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region MessageMetadata //////////////////////////////////////////////////////////////////////////////////////////////

// MessageMetadata represents the JSON model of the tangle.MessageMetadata.
type MessageMetadata struct {
	ID                 string            `json:"id"`
	ReceivedTime       int64             `json:"receivedTime"`
	Solid              bool              `json:"solid"`
	SolidificationTime int64             `json:"solidificationTime"`
	StructureDetails   *StructureDetails `json:"structureDetails,omitempty"`
	BranchID           string            `json:"branchID"`
	Scheduled          bool              `json:"scheduled"`
	Booked             bool              `json:"booked"`
	Eligible           bool              `json:"eligible"`
	Invalid            bool              `json:"invalid"`
}

// NewMessageMetadata returns MessageMetadata from the given tangle.MessageMetadata.
func NewMessageMetadata(metadata *tangle.MessageMetadata) MessageMetadata {
	return MessageMetadata{
		ID:                 metadata.ID().String(),
		ReceivedTime:       metadata.ReceivedTime().Unix(),
		Solid:              metadata.IsSolid(),
		SolidificationTime: metadata.SolidificationTime().Unix(),
		StructureDetails:   NewStructureDetails(metadata.StructureDetails()),
		BranchID:           metadata.BranchID().String(),
		Scheduled:          metadata.Scheduled(),
		Booked:             metadata.IsBooked(),
		Eligible:           metadata.IsEligible(),
		Invalid:            metadata.IsInvalid(),
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
