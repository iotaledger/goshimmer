package jsonmodels

// FaucetRequestResponse contains the ID of the block sent.
type FaucetRequestResponse struct {
	ID    string `json:"id,omitempty"`
	Error string `json:"error,omitempty"`
}

// FaucetAPIResponse contains the status of facet request through web API.
type FaucetAPIResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

// FaucetRequest contains the address to request funds from faucet.
type FaucetRequest struct {
	Address               string `json:"address"`
	AccessManaPledgeID    string `json:"accessManaPledgeID"`
	ConsensusManaPledgeID string `json:"consensusManaPledgeID"`
	Nonce                 uint64 `json:"nonce"`
}
