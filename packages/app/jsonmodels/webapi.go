package jsonmodels

import (
	"strconv"

	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool/conflictdag"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/vm/devnetvm"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/sybilprotection"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
	"github.com/iotaledger/hive.go/crypto/identity"
	"github.com/iotaledger/hive.go/ds/advancedset"
)

// region GetAddressResponse ///////////////////////////////////////////////////////////////////////////////////////////

// GetAddressResponse represents the JSON model of a response from the GetAddress endpoint.
type GetAddressResponse struct {
	Address        *Address  `json:"address"`
	SpentOutputs   []*Output `json:"spentOutputs"`
	UnspentOutputs []*Output `json:"unspentOutputs"`
}

// NewGetAddressResponse returns a GetAddressResponse from the given details.
func NewGetAddressResponse(address devnetvm.Address, spent, unspent devnetvm.Outputs) *GetAddressResponse {
	mappedOutput := func(outputs devnetvm.Outputs) (mappedOutputs []*Output) {
		mappedOutputs = make([]*Output, 0)
		for _, output := range outputs {
			if output != nil {
				mappedOutputs = append(mappedOutputs, NewOutput(output))
			}
		}

		return
	}

	return &GetAddressResponse{
		Address:        NewAddress(address),
		SpentOutputs:   mappedOutput(spent),
		UnspentOutputs: mappedOutput(unspent),
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region PostAddressesUnspentOutputsRequest

// PostAddressesUnspentOutputsRequest is a the request object for the /ledgerstate/addresses/unspentOutputs endpoint.
type PostAddressesUnspentOutputsRequest struct {
	Addresses []string `json:"addresses"`
}

// endregion

// region PostAddressesUnspentOutputsResponse

// PostAddressesUnspentOutputsResponse is a the response object for the /ledgerstate/addresses/unspentOutputs endpoint.
type PostAddressesUnspentOutputsResponse struct {
	UnspentOutputs []*WalletOutputsOnAddress `json:"unspentOutputs"`
}

// endregion

// region GetConflictChildrenResponse ////////////////////////////////////////////////////////////////////////////////////

// GetConflictChildrenResponse represents the JSON model of a response from the GetConflictChildren endpoint.
type GetConflictChildrenResponse struct {
	ConflictID     string           `json:"conflictID"`
	ChildConflicts []*ChildConflict `json:"childConflicts"`
}

// NewGetConflictChildrenResponse returns a GetConflictChildrenResponse from the given details.
func NewGetConflictChildrenResponse(conflictID utxo.TransactionID, childConflicts *advancedset.AdvancedSet[*conflictdag.Conflict[utxo.TransactionID, utxo.OutputID]]) *GetConflictChildrenResponse {
	return &GetConflictChildrenResponse{
		ConflictID: conflictID.Base58(),
		ChildConflicts: func() (mappedChildConflicts []*ChildConflict) {
			mappedChildConflicts = make([]*ChildConflict, 0)
			for it := childConflicts.Iterator(); it.HasNext(); {
				mappedChildConflicts = append(mappedChildConflicts, NewChildConflict(it.Next()))
			}

			return
		}(),
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region GetConflictConflictsResponse ///////////////////////////////////////////////////////////////////////////////////

// GetConflictConflictsResponse represents the JSON model of a response from the GetConflictConflicts endpoint.
type GetConflictConflictsResponse struct {
	ConflictID string      `json:"conflictID"`
	Conflicts  []*Conflict `json:"conflicts"`
}

// NewGetConflictConflictsResponse returns a GetConflictConflictsResponse from the given details.
func NewGetConflictConflictsResponse(conflictID utxo.TransactionID, conflictIDsPerConflictID map[utxo.OutputID][]utxo.TransactionID) *GetConflictConflictsResponse {
	return &GetConflictConflictsResponse{
		ConflictID: conflictID.Base58(),
		Conflicts: func() (mappedConflicts []*Conflict) {
			mappedConflicts = make([]*Conflict, 0)
			for conflictID, conflictIDs := range conflictIDsPerConflictID {
				mappedConflicts = append(mappedConflicts, NewConflict(conflictID, conflictIDs))
			}

			return
		}(),
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region GetConflictVotersResponse //////////////////////////////////////////////////////////////////////////////////////

// GetConflictVotersResponse represents the JSON model of a response from the GetConflictVoters endpoint.
type GetConflictVotersResponse struct {
	ConflictID string   `json:"conflictID"`
	Voters     []string `json:"voters"`
}

// NewGetConflictVotersResponse returns a GetConflictVotersResponse from the given details.
func NewGetConflictVotersResponse(conflictID utxo.TransactionID, voters *sybilprotection.WeightedSet) *GetConflictVotersResponse {
	return &GetConflictVotersResponse{
		ConflictID: conflictID.Base58(),
		Voters: func() (votersStr []string) {
			votersStr = make([]string, 0)
			_ = voters.ForEachWeighted(func(id identity.ID, weight int64) error {
				votersStr = append(votersStr, id.String()+", "+strconv.FormatInt(weight, 10))
				return nil
			})
			return
		}(),
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region GetOutputConsumersResponse ///////////////////////////////////////////////////////////////////////////////////

// GetOutputConsumersResponse represents the JSON model of a response from the GetOutputConsumers endpoint.
type GetOutputConsumersResponse struct {
	OutputID  *OutputID   `json:"outputID"`
	Consumers []*Consumer `json:"consumers"`
}

// NewGetOutputConsumersResponse returns a GetOutputConsumersResponse from the given details.
func NewGetOutputConsumersResponse(outputID utxo.OutputID, consumers []*mempool.Consumer) *GetOutputConsumersResponse {
	return &GetOutputConsumersResponse{
		OutputID: NewOutputID(outputID),
		Consumers: func() []*Consumer {
			consumingTransactions := make([]*Consumer, 0)
			for _, consumer := range consumers {
				if consumer != nil {
					consumingTransactions = append(consumingTransactions, NewConsumer(consumer))
				}
			}

			return consumingTransactions
		}(),
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region GetTransactionAttachmentsResponse ////////////////////////////////////////////////////////////////////////////

// GetTransactionAttachmentsResponse represents the JSON model of a response from the GetTransactionAttachments endpoint.
type GetTransactionAttachmentsResponse struct {
	TransactionID string   `json:"transactionID"`
	BlockIDs      []string `json:"blockIDs"`
}

// NewGetTransactionAttachmentsResponse returns a GetTransactionAttachmentsResponse from the given details.
func NewGetTransactionAttachmentsResponse(transactionID utxo.TransactionID, blockIDs models.BlockIDs) *GetTransactionAttachmentsResponse {
	var blockIDsBase58 []string
	for blockID := range blockIDs {
		blockIDsBase58 = append(blockIDsBase58, blockID.Base58())
	}

	return &GetTransactionAttachmentsResponse{
		TransactionID: transactionID.Base58(),
		BlockIDs:      blockIDsBase58,
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region PostPayloadRequest ///////////////////////////////////////////////////////////////////////////////////////////

// PostPayloadRequest represents the JSON model of a PostPayload request.
type PostPayloadRequest struct {
	Payload []byte `json:"payload"`
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region PostPayloadResponse //////////////////////////////////////////////////////////////////////////////////////////

// PostPayloadResponse represents the JSON model of a PostPayload response.
type PostPayloadResponse struct {
	ID string `json:"id"`
}

// NewPostPayloadResponse returns a PostPayloadResponse from the given tangleold.Block.
func NewPostPayloadResponse(block *models.Block) *PostPayloadResponse {
	return &PostPayloadResponse{
		ID: block.ID().Base58(),
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region PostTransaction Req/Resp /////////////////////////////////////////////////////////////////////////////////////

// PostTransactionRequest holds the transaction object(bytes) to send.
type PostTransactionRequest struct {
	TransactionBytes []byte `json:"txn_bytes"`
}

// PostTransactionResponse is the HTTP response from sending transaction.
type PostTransactionResponse struct {
	TransactionID string `json:"transaction_id,omitempty"`
	BlockID       string `json:"block_id,omitempty"`
	Error         string `json:"error,omitempty"`
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region ErrorResponse ////////////////////////////////////////////////////////////////////////////////////////////////

// ErrorResponse represents the JSON model of an error response from an API endpoint.
type ErrorResponse struct {
	Error string `json:"error"`
}

// NewErrorResponse returns am ErrorResponse from the given error.
func NewErrorResponse(err error) *ErrorResponse {
	return &ErrorResponse{
		Error: err.Error(),
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region GetUnspentOutputsResponse ////////////////////////////////////////////////////////////////////////////////////

// GetUnspentOutputResponse represents the JSON model of a response from the GetUnspentOutput endpoint.
type GetUnspentOutputResponse struct {
	Address *Address  `json:"address"`
	Outputs []*Output `json:"outputs"`
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// GetSnapshotRequest represents the JSON model of a GetSnapshot request.
type GetSnapshotRequest struct {
	SlotIndex uint64 `query:"index"`
}
