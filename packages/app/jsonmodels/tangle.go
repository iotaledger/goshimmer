package jsonmodels

import "github.com/iotaledger/hive.go/core/types/confirmation"

// region Block ///////////////////////////////////////////////////////////////////////////////////////////////////////

// Block represents the JSON model of a tangleold.Block.
type Block struct {
	ID                   string   `json:"id"`
	StrongParents        []string `json:"strongParents"`
	WeakParents          []string `json:"weakParents"`
	ShallowLikeParents   []string `json:"shallowLikeParents"`
	StrongChildren       []string `json:"strongChildren"`
	WeakChildren         []string `json:"weakChildren"`
	ShallowLikeChildren  []string `json:"shallowLikeChildren"`
	IssuerPublicKey      string   `json:"issuerPublicKey"`
	IssuingTime          int64    `json:"issuingTime"`
	SequenceNumber       uint64   `json:"sequenceNumber"`
	PayloadType          string   `json:"payloadType"`
	TransactionID        string   `json:"transactionID,omitempty"`
	Payload              []byte   `json:"payload"`
	EC                   string   `json:"ec"`
	EI                   uint64   `json:"ei"`
	ECR                  string   `json:"ecr"`
	PrevEC               string   `json:"prevEC"`
	Signature            string   `json:"signature"`
	LatestConfirmedEpoch uint64   `json:"latestConfirmedEpoch"`
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region BlockMetadata //////////////////////////////////////////////////////////////////////////////////////////////

// BlockMetadata represents the JSON model of the tangleold.BlockMetadata.
type BlockMetadata struct {
	ID                    string             `json:"id"`
	ReceivedTime          int64              `json:"receivedTime"`
	Solid                 bool               `json:"solid"`
	SolidificationTime    int64              `json:"solidificationTime"`
	StructureDetails      *StructureDetails  `json:"structureDetails,omitempty"`
	ConflictIDs           []string           `json:"conflictIDs"`
	AddedConflictIDs      []string           `json:"addedConflictIDs"`
	SubtractedConflictIDs []string           `json:"subtractedConflictIDs"`
	Scheduled             bool               `json:"scheduled"`
	ScheduledTime         int64              `json:"scheduledTime"`
	Booked                bool               `json:"booked"`
	BookedTime            int64              `json:"bookedTime"`
	ObjectivelyInvalid    bool               `json:"objectivelyInvalid"`
	SubjectivelyInvalid   bool               `json:"subjectivelyInvalid"`
	ConfirmationState     confirmation.State `json:"confirmationState"`
	ConfirmationStateTime int64              `json:"confirmationStateTime"`
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
