package jsonmodels

// FaucetResponse contains the ID of the message sent.
type FaucetResponse struct {
	ID    string `json:"id,omitempty"`
	Error string `json:"error,omitempty"`
}

// FaucetRequest contains the address to request funds from faucet.
type FaucetRequest struct {
	Address               string `json:"address"`
	AccessManaPledgeID    string `json:"accessManaPledgeID"`
	ConsensusManaPledgeID string `json:"consensusManaPledgeID"`
	Nonce                 uint64 `json:"nonce"`
}
