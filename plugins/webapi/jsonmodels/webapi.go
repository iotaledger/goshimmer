package jsonmodels

import (
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/tangle"
)

// region GetAddressResponse ///////////////////////////////////////////////////////////////////////////////////////////

// GetAddressResponse represents the JSON model of a response from the GetAddress endpoint.
type GetAddressResponse struct {
	Address *Address  `json:"address"`
	Outputs []*Output `json:"outputs"`
}

// NewGetAddressResponse returns a GetAddressResponse from the given details.
func NewGetAddressResponse(address ledgerstate.Address, outputs ledgerstate.Outputs) *GetAddressResponse {
	return &GetAddressResponse{
		Address: NewAddress(address),
		Outputs: func() (mappedOutputs []*Output) {
			mappedOutputs = make([]*Output, 0)
			for _, output := range outputs {
				if output != nil {
					mappedOutputs = append(mappedOutputs, NewOutput(output))
				}
			}

			return
		}(),
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region GetBranchChildrenResponse ////////////////////////////////////////////////////////////////////////////////////

// GetBranchChildrenResponse represents the JSON model of a response from the GetBranchChildren endpoint.
type GetBranchChildrenResponse struct {
	BranchID      string         `json:"branchID"`
	ChildBranches []*ChildBranch `json:"childBranches"`
}

// NewGetBranchChildrenResponse returns a GetBranchChildrenResponse from the given details.
func NewGetBranchChildrenResponse(branchID ledgerstate.BranchID, childBranches []*ledgerstate.ChildBranch) *GetBranchChildrenResponse {
	return &GetBranchChildrenResponse{
		BranchID: branchID.Base58(),
		ChildBranches: func() (mappedChildBranches []*ChildBranch) {
			mappedChildBranches = make([]*ChildBranch, 0)
			for _, childBranch := range childBranches {
				mappedChildBranches = append(mappedChildBranches, NewChildBranch(childBranch))
			}

			return
		}(),
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region GetBranchConflictsResponse ///////////////////////////////////////////////////////////////////////////////////

// GetBranchConflictsResponse represents the JSON model of a response from the GetBranchConflicts endpoint.
type GetBranchConflictsResponse struct {
	BranchID  string      `json:"branchID"`
	Conflicts []*Conflict `json:"conflicts"`
}

// NewGetBranchConflictsResponse returns a GetBranchConflictsResponse from the given details.
func NewGetBranchConflictsResponse(branchID ledgerstate.BranchID, branchIDsPerConflictID map[ledgerstate.ConflictID][]ledgerstate.BranchID) *GetBranchConflictsResponse {
	return &GetBranchConflictsResponse{
		BranchID: branchID.Base58(),
		Conflicts: func() (mappedConflicts []*Conflict) {
			mappedConflicts = make([]*Conflict, 0)
			for conflictID, branchIDs := range branchIDsPerConflictID {
				mappedConflicts = append(mappedConflicts, NewConflict(conflictID, branchIDs))
			}

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
func NewGetOutputConsumersResponse(outputID ledgerstate.OutputID, consumers []*ledgerstate.Consumer) *GetOutputConsumersResponse {
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
	MessageIDs    []string `json:"messageIDs"`
}

// NewGetTransactionAttachmentsResponse returns a GetTransactionAttachmentsResponse from the given details.
func NewGetTransactionAttachmentsResponse(transactionID ledgerstate.TransactionID, messageIDs tangle.MessageIDs) *GetTransactionAttachmentsResponse {
	var messageIDsBase58 []string
	for _, messageID := range messageIDs {
		messageIDsBase58 = append(messageIDsBase58, messageID.Base58())
	}

	return &GetTransactionAttachmentsResponse{
		TransactionID: transactionID.Base58(),
		MessageIDs:    messageIDsBase58,
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

// NewPostPayloadResponse returns a PostPayloadResponse from the given tangle.Message.
func NewPostPayloadResponse(message *tangle.Message) *PostPayloadResponse {
	return &PostPayloadResponse{
		ID: message.ID().Base58(),
	}
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
