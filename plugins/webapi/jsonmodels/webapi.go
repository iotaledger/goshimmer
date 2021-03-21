package jsonmodels

import (
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/tangle"
)

// region GetAddressResponse ///////////////////////////////////////////////////////////////////////////////////////////

// GetAddressResponse represents the JSON model of Outputs that are associated to an Address.
type GetAddressResponse struct {
	Address *Address  `json:"address"`
	Outputs []*Output `json:"outputs"`
}

// NewGetAddressResponse creates a JSON model of Outputs that are associated to an Address.
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

// GetBranchChildrenResponse represents the JSON model of a collection of ChildBranches.
type GetBranchChildrenResponse struct {
	BranchID      string         `json:"branchID"`
	ChildBranches []*ChildBranch `json:"childBranches"`
}

// NewGetBranchChildrenResponse returns a GetBranchChildrenResponse from the given collection of ledgerstate.ChildBranches.
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

// GetBranchConflictsResponse represents the JSON model of a collection of Conflicts that a ledgerstate.ConflictBranch is part of.
type GetBranchConflictsResponse struct {
	BranchID  string      `json:"branchID"`
	Conflicts []*Conflict `json:"conflicts"`
}

// NewGetBranchConflictsResponse returns the GetBranchConflictsResponse that a ledgerstate.ConflictBranch is part of.
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

// GetOutputConsumersResponse is the JSON model of a collection of Consumers of an Output.
type GetOutputConsumersResponse struct {
	OutputID  *OutputID   `json:"outputID"`
	Consumers []*Consumer `json:"consumers"`
}

// NewGetOutputConsumersResponse creates an GetOutputConsumersResponse from the given details.
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

// GetTransactionAttachmentsResponse represents the JSON model of a collection of MessageIDs that attached a particular Transaction.
type GetTransactionAttachmentsResponse struct {
	TransactionID string   `json:"transactionID"`
	MessageIDs    []string `json:"messageIDs"`
}

// NewGetTransactionAttachmentsResponse returns a collection of MessageIDs that attached a particular Transaction from the given details.
func NewGetTransactionAttachmentsResponse(transactionID ledgerstate.TransactionID, messageIDs tangle.MessageIDs) *GetTransactionAttachmentsResponse {
	var messageIDsBase58 []string
	for _, messageID := range messageIDs {
		messageIDsBase58 = append(messageIDsBase58, messageID.String())
	}

	return &GetTransactionAttachmentsResponse{
		TransactionID: transactionID.Base58(),
		MessageIDs:    messageIDsBase58,
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region ErrorResponse ////////////////////////////////////////////////////////////////////////////////////////////////

// ErrorResponse is the response that is returned when an error occurred in any of the endpoints.
type ErrorResponse struct {
	Error string `json:"error"`
}

// NewErrorResponse returns an ErrorResponse from the given error.
func NewErrorResponse(err error) ErrorResponse {
	return ErrorResponse{
		Error: err.Error(),
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
