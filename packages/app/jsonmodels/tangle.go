package jsonmodels

// region Block ///////////////////////////////////////////////////////////////////////////////////////////////////////

// Block represents the JSON model of a tangleold.Block.
type Block struct {
	ID                   string   `json:"id"`
	StrongParents        []string `json:"strongParents"`
	WeakParents          []string `json:"weakParents"`
	ShallowLikeParents   []string `json:"shallowLikeParents"`
	StrongChildren       []string `json:"strongChildren"`
	WeakChildren         []string `json:"weakChildren"`
	LikedInsteadChildren []string `json:"likedInsteadChildren"`
	IssuerPublicKey      string   `json:"issuerPublicKey"`
	IssuingTime          int64    `json:"issuingTime"`
	SequenceNumber       uint64   `json:"sequenceNumber"`
	PayloadType          string   `json:"payloadType"`
	TransactionID        string   `json:"transactionID,omitempty"`
	Payload              []byte   `json:"payload"`
	CommitmentID         string   `json:"commitmentID"`
	SlotIndex            uint64   `json:"slotIndex"`
	CommitmentRootsID    string   `json:"commitmentRootsID"`
	PrevCommitmentID     string   `json:"PrevCommitmentID"`
	Signature            string   `json:"signature"`
	LatestConfirmedSlot  uint64   `json:"latestConfirmedSlot"`
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
