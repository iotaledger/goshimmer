package jsonmodels

import (
	"github.com/iotaledger/goshimmer/packages/consensus/fcob"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
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

// NewMessage returns a Message from the given tangle.Message.
func NewMessage(message *tangle.Message) Message {
	return Message{
		ID:              message.ID().Base58(),
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

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region MessageMetadata //////////////////////////////////////////////////////////////////////////////////////////////

// MessageMetadata represents the JSON model of the tangle.MessageMetadata.
type MessageMetadata struct {
	ID                      string            `json:"id"`
	ReceivedTime            int64             `json:"receivedTime"`
	Solid                   bool              `json:"solid"`
	SolidificationTime      int64             `json:"solidificationTime"`
	StructureDetails        *StructureDetails `json:"structureDetails,omitempty"`
	BranchID                string            `json:"branchID"`
	Scheduled               bool              `json:"scheduled"`
	Booked                  bool              `json:"booked"`
	Eligible                bool              `json:"eligible"`
	Invalid                 bool              `json:"invalid"`
	FinalizedApprovalWeight bool              `json:"finalizedApprovalWeight"`
}

// NewMessageMetadata returns MessageMetadata from the given tangle.MessageMetadata.
func NewMessageMetadata(metadata *tangle.MessageMetadata) MessageMetadata {
	branchID, err := messagelayer.Tangle().Booker.MessageBranchID(metadata.ID())
	if err != nil {
		branchID = ledgerstate.BranchID{}
	}

	return MessageMetadata{
		ID:                      metadata.ID().Base58(),
		ReceivedTime:            metadata.ReceivedTime().Unix(),
		Solid:                   metadata.IsSolid(),
		SolidificationTime:      metadata.SolidificationTime().Unix(),
		StructureDetails:        NewStructureDetails(metadata.StructureDetails()),
		BranchID:                branchID.String(),
		Scheduled:               metadata.Scheduled(),
		Booked:                  metadata.IsBooked(),
		Eligible:                metadata.IsEligible(),
		Invalid:                 metadata.IsInvalid(),
		FinalizedApprovalWeight: metadata.IsFinalizedApprovalWeight(),
	}
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
