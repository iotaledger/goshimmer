package jsonmodels

import (
	"github.com/iotaledger/goshimmer/packages/consensus/fcob"
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
	ScheduledTime      int64             `json:"scheduledTime"`
	Booked             bool              `json:"booked"`
	BookedTime         int64             `json:"bookedTime"`
	Eligible           bool              `json:"eligible"`
	Invalid            bool              `json:"invalid"`
	Finalized          bool              `json:"finalized"`
	FinalizedTime      int64             `json:"finalizedTime"`
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region MessageConsensusMetadata /////////////////////////////////////////////////////////////////////////////////////

// MessageConsensusMetadata represents the JSON model of a tangle.Message's consensus metadata.
type MessageConsensusMetadata struct {
	ID                      string `json:"id"`
	OpinionFormedTime       int64  `json:"opinionFormedTime"`
	PayloadOpinionFormed    bool   `json:"payloadOpinionFormed"`
	TimestampOpinionFormed  bool   `json:"timestampOpinionFormed"`
	MessageOpinionFormed    bool   `json:"messageOpinionFormed"`
	MessageOpinionTriggered bool   `json:"messageOpinionTriggered"`
	TimestampOpinion        string `json:"timestampOpinion"`
	TimestampLoK            string `json:"timestampLoK"`
}

// NewMessageConsensusMetadata returns MessageConsensusMetadata from the given tangle.MessageMetadata.
func NewMessageConsensusMetadata(metadata *fcob.MessageMetadata, timestampOpinion *fcob.TimestampOpinion) MessageConsensusMetadata {
	return MessageConsensusMetadata{
		ID:                      metadata.ID().String(),
		OpinionFormedTime:       metadata.OpinionFormedTime().Unix(),
		PayloadOpinionFormed:    metadata.PayloadOpinionFormed(),
		TimestampOpinionFormed:  metadata.TimestampOpinionFormed(),
		MessageOpinionFormed:    metadata.MessageOpinionFormed(),
		MessageOpinionTriggered: metadata.MessageOpinionTriggered(),
		TimestampOpinion:        timestampOpinion.Value.String(),
		TimestampLoK:            timestampOpinion.LoK.String(),
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
