package api

type BroadcastDataResponse struct {
	Hash  string `json:"hash,omitempty"`
	Error string `json:"error,omitempty"`
}

type BroadcastDataRequest struct {
	Data string `json:"data"`
}

type GetMessageByHashResponse struct {
	Messages []Message `json:"messages,omitempty"`
	Error    string    `json:"error,omitempty"`
}

type GetMessageByHashRequest struct {
	Hashes []string `json:"hashes"`
}

type Message struct {
	MessageId           string `json:"messageId,omitempty"`
	TrunkTransactionId  string `json:"trunkTransactionId,omitempty"`
	BranchTransactionId string `json:"branchTransactionId,omitempty"`
	IssuerPublicKey     string `json:"issuerPublicKey,omitempty"`
	IssuingTime         string `json:"issuingTime,omitempty"`
	SequenceNumber      uint64 `json:"sequenceNumber,omitempty"`
	Payload             string `json:"payload,omitempty"`
	Signature           string `json:"signature,omitempty"`
}

type GetNeighborResponse struct {
	KnownPeers []Neighbor `json:"known,omitempty"`
	Chosen     []Neighbor `json:"chosen"`
	Accepted   []Neighbor `json:"accepted"`
	Error      string     `json:"error,omitempty"`
}

type Neighbor struct {
	ID        string        `json:"id"`        // comparable node identifier
	PublicKey string        `json:"publicKey"` // public key used to verify signatures
	Services  []PeerService `json:"services,omitempty"`
}

type PeerService struct {
	ID      string `json:"id"`      // ID of the service
	Address string `json:"address"` // network address of the service
}
