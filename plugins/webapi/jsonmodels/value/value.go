package value

import (
	"time"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
)

// AttachmentsResponse is the HTTP response from retrieving value objects.
type AttachmentsResponse struct {
	Attachments []ValueObject `json:"attachments,omitempty"`
	Error       string        `json:"error,omitempty"`
}

// ValueObject holds the information of a value object.
type ValueObject struct {
	ID          string      `json:"id"`
	Parents     []string    `json:"parents"`
	Transaction Transaction `json:"transaction"`
}

// TransactionMetadata holds the information of a transaction metadata.
type TransactionMetadata struct {
	BranchID           string `json:"branchID"`
	Solid              bool   `json:"solid"`
	SolidificationTime int64  `json:"solidificationTime"`
	Finalized          bool   `json:"finalized"`
	LazyBooked         bool   `json:"lazyBooked"`
}

// Transaction holds the information of a transaction.
type Transaction struct {
	Version           ledgerstate.TransactionEssenceVersion `json:"version"`
	Timestamp         int64                                 `json:"timestamp"`
	AccessPledgeID    string                                `json:"accessPledgeID"`
	ConsensusPledgeID string                                `json:"consensusPledgeID"`
	Inputs            []Input                               `json:"inputs"`
	Outputs           []Output                              `json:"outputs"`
	UnlockBlocks      []UnlockBlock                         `json:"unlockBlocks"`
	DataPayload       []byte                                `json:"dataPayload"`
}

// Metadata holds metadata about the output.
type Metadata struct {
	Timestamp time.Time `json:"timestamp"`
}

// OutputID holds the output id and its inclusion state
type OutputID struct {
	ID             string         `json:"id"`
	Balances       []Balance      `json:"balances"`
	InclusionState InclusionState `json:"inclusion_state"`
	Metadata       Metadata       `json:"output_metadata"`
}

// UnspentOutput holds the address and the corresponding unspent output ids
type UnspentOutput struct {
	Address   string     `json:"address"`
	OutputIDs []OutputID `json:"output_ids"`
}

// Input holds the consumedOutputID
type Input struct {
	ConsumedOutputID string `json:"consumedOutputID"`
}

// Output consists an address and balances
type Output struct {
	Type     ledgerstate.OutputType `json:"type"`
	Address  string                 `json:"address"`
	Balances []Balance              `json:"balances"`
}

// Balance holds the value and the color of token
type Balance struct {
	Color string `json:"color"`
	Value int64  `json:"value"`
}

// InclusionState represents the different states of an OutputID
type InclusionState struct {
	Solid       bool `json:"solid,omitempty"`
	Confirmed   bool `json:"confirmed,omitempty"`
	Rejected    bool `json:"rejected,omitempty"`
	Liked       bool `json:"liked,omitempty"`
	Conflicting bool `json:"conflicting,omitempty"`
	Finalized   bool `json:"finalized,omitempty"`
	Preferred   bool `json:"preferred,omitempty"`
}

// UnlockBlock defines the struct of a signature.
type UnlockBlock struct {
	Type            ledgerstate.UnlockBlockType `json:"type"`
	ReferencedIndex uint16                      `json:"referencedIndex,omitempty"`
	SignatureType   ledgerstate.SignatureType   `json:"signatureType,omitempty"`
	PublicKey       string                      `json:"publicKey,omitempty"`
	Signature       string                      `json:"signature,omitempty"`
}

// GetTransactionByIDResponse is the HTTP response from retrieving transaction.
type GetTransactionByIDResponse struct {
	TransactionMetadata TransactionMetadata `json:"transactionMetadata"`
	Transaction         Transaction         `json:"transaction"`
	InclusionState      InclusionState      `json:"inclusion_state"`
	Error               string              `json:"error,omitempty"`
}

// SendTransactionByJSONRequest holds the transaction object(json) to send.
// e.g.,
// {
// 	"inputs": string[],
//  "a_mana_pledge": string,
//  "c_mana_pledg": string,
// 	"outputs": {
//	   "type": number,
// 	   "address": string,
// 	   "balances": {
// 		   "value": number,
// 		   "color": string
// 	   }[];
// 	 }[],
// 	 "signature": []string
//  }
type SendTransactionByJSONRequest struct {
	Inputs        []string      `json:"inputs"`
	Outputs       []Output      `json:"outputs"`
	AManaPledgeID string        `json:"a_mana_pledg"`
	CManaPledgeID string        `json:"c_mana_pledg"`
	Signatures    []UnlockBlock `json:"signatures"`
	Payload       []byte        `json:"payload"`
}

// SendTransactionByJSONResponse is the HTTP response from sending transaction.
type SendTransactionByJSONResponse struct {
	TransactionID string `json:"transaction_id,omitempty"`
	Error         string `json:"error,omitempty"`
}

// UnspentOutputsRequest holds the addresses to query.
type UnspentOutputsRequest struct {
	Addresses []string `json:"addresses,omitempty"`
	Error     string   `json:"error,omitempty"`
}

// UnspentOutputsResponse is the HTTP response from retrieving value objects.
type UnspentOutputsResponse struct {
	UnspentOutputs []UnspentOutput `json:"unspent_outputs,omitempty"`
	Error          string          `json:"error,omitempty"`
}

// SendTransactionRequest holds the transaction object(bytes) to send.
type SendTransactionRequest struct {
	TransactionBytes []byte `json:"txn_bytes"`
}

// SendTransactionResponse is the HTTP response from sending transaction.
type SendTransactionResponse struct {
	TransactionID string `json:"transaction_id,omitempty"`
	Error         string `json:"error,omitempty"`
}

// AllowedManaPledgeResponse is the http response.
type AllowedManaPledgeResponse struct {
	Access    AllowedPledge `json:"accessMana"`
	Consensus AllowedPledge `json:"consensusMana"`
	Error     string        `json:"error,omitempty"`
}

// AllowedPledge represents the nodes that mana is allowed to be pledged to.
type AllowedPledge struct {
	IsFilterEnabled bool     `json:"isFilterEnabled"`
	Allowed         []string `json:"allowed,omitempty"`
}
