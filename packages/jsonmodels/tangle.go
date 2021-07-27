package jsonmodels

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
