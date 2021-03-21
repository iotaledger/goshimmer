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
