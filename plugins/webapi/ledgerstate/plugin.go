package ledgerstate

import (
	"fmt"
	"net/http"
	"sync"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
	"github.com/iotaledger/goshimmer/plugins/webapi"
	"github.com/iotaledger/goshimmer/plugins/webapi/jsonmodels"
	"github.com/iotaledger/hive.go/node"
	"github.com/labstack/echo"
	"golang.org/x/xerrors"
)

// region Plugin ///////////////////////////////////////////////////////////////////////////////////////////////////////

var (
	// plugin holds the singleton instance of the plugin.
	plugin *node.Plugin

	// pluginOnce is used to ensure that the plugin is a singleton.
	pluginOnce sync.Once
)

// Plugin returns the plugin as a singleton.
func Plugin() *node.Plugin {
	pluginOnce.Do(func() {
		plugin = node.NewPlugin("WebAPI ledgerstate Endpoint", node.Enabled, func(*node.Plugin) {
			webapi.Server().GET("ledgerstate/addresses/:address", GetAddress)
			webapi.Server().GET("ledgerstate/addresses/:address/unspentOutputs", GetAddressUnspentOutputs)
			webapi.Server().GET("ledgerstate/branches/:branchID", GetBranch)
			webapi.Server().GET("ledgerstate/branches/:branchID/children", GetBranchChildren)
			webapi.Server().GET("ledgerstate/branches/:branchID/conflicts", GetBranchConflicts)
			webapi.Server().GET("ledgerstate/outputs/:outputID", GetOutput)
			webapi.Server().GET("ledgerstate/outputs/:outputID/consumers", GetOutputConsumers)
			webapi.Server().GET("ledgerstate/outputs/:outputID/metadata", GetOutputMetadata)
			webapi.Server().GET("ledgerstate/transactions/:transactionID", GetTransaction)
			webapi.Server().GET("ledgerstate/transactions/:transactionID/metadata", GetTransactionMetadata)
			webapi.Server().GET("ledgerstate/transactions/:transactionID/attachments", GetTransactionAttachments)
		})
	})

	return plugin
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region GetAddress ///////////////////////////////////////////////////////////////////////////////////////////////////

// GetAddress is the handler for the /ledgerstate/addresses/:address endpoint.
func GetAddress(c echo.Context) error {
	address, err := ledgerstate.AddressFromBase58EncodedString(c.Param("address"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels.NewErrorResponse(err))
	}

	cachedOutputs := messagelayer.Tangle().LedgerState.OutputsOnAddress(address)
	defer cachedOutputs.Release()

	return c.JSON(http.StatusOK, jsonmodels.NewGetAddressResponse(address, cachedOutputs.Unwrap()))
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region GetAddressUnspentOutputs /////////////////////////////////////////////////////////////////////////////////////

// GetAddressUnspentOutputs is the handler for the /ledgerstate/addresses/:address/unspentOutputs endpoint.
func GetAddressUnspentOutputs(c echo.Context) error {
	address, err := ledgerstate.AddressFromBase58EncodedString(c.Param("address"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels.NewErrorResponse(err))
	}

	cachedOutputs := messagelayer.Tangle().LedgerState.OutputsOnAddress(address)
	defer cachedOutputs.Release()

	return c.JSON(http.StatusOK, jsonmodels.NewGetAddressResponse(address, cachedOutputs.Unwrap().Filter(func(output ledgerstate.Output) (isUnspent bool) {
		messagelayer.Tangle().LedgerState.OutputMetadata(output.ID()).Consume(func(outputMetadata *ledgerstate.OutputMetadata) {
			isUnspent = outputMetadata.ConsumerCount() == 0
		})

		return
	})))
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region GetBranch ////////////////////////////////////////////////////////////////////////////////////////////////////

// GetBranch is the handler for the /ledgerstate/branch/:branchID endpoint.
func GetBranch(c echo.Context) (err error) {
	branchID, err := branchIDFromContext(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels.NewErrorResponse(err))
	}

	if messagelayer.Tangle().LedgerState.BranchDAG.Branch(branchID).Consume(func(branch ledgerstate.Branch) {
		err = c.JSON(http.StatusOK, jsonmodels.NewBranch(branch))
	}) {
		return
	}

	return c.JSON(http.StatusNotFound, jsonmodels.NewErrorResponse(fmt.Errorf("failed to load Branch with %s", branchID)))
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region GetBranchChildren ////////////////////////////////////////////////////////////////////////////////////////////

// GetBranchChildren is the handler for the /ledgerstate/branch/:branchID/childBranches endpoint.
func GetBranchChildren(c echo.Context) (err error) {
	branchID, err := branchIDFromContext(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels.NewErrorResponse(err))
	}

	cachedChildBranches := messagelayer.Tangle().LedgerState.BranchDAG.ChildBranches(branchID)
	defer cachedChildBranches.Release()

	return c.JSON(http.StatusOK, NewGetBranchChildrenResponse(branchID, cachedChildBranches.Unwrap()))
}

// GetBranchChildrenResponse represents the JSON model of a collection of ChildBranch objects.
type GetBranchChildrenResponse struct {
	BranchID      string                    `json:"branchID"`
	ChildBranches []*jsonmodels.ChildBranch `json:"childBranches"`
}

// NewGetBranchChildrenResponse returns GetBranchChildrenResponse from the given collection of ledgerstate.ChildBranch objects.
func NewGetBranchChildrenResponse(branchID ledgerstate.BranchID, childBranches []*ledgerstate.ChildBranch) *GetBranchChildrenResponse {
	return &GetBranchChildrenResponse{
		BranchID: branchID.Base58(),
		ChildBranches: func() (mappedChildBranches []*jsonmodels.ChildBranch) {
			mappedChildBranches = make([]*jsonmodels.ChildBranch, 0)
			for _, childBranch := range childBranches {
				mappedChildBranches = append(mappedChildBranches, jsonmodels.NewChildBranch(childBranch))
			}

			return
		}(),
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region GetBranchConflicts ///////////////////////////////////////////////////////////////////////////////////////////

// GetBranchConflicts is the handler for the /ledgerstate/branch/:branchID/conflicts endpoint.
func GetBranchConflicts(c echo.Context) (err error) {
	branchID, err := branchIDFromContext(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels.NewErrorResponse(err))
	}

	if messagelayer.Tangle().LedgerState.BranchDAG.Branch(branchID).Consume(func(branch ledgerstate.Branch) {
		if branch.Type() != ledgerstate.ConflictBranchType {
			err = c.JSON(http.StatusBadRequest, jsonmodels.NewErrorResponse(fmt.Errorf("the Branch with %s is not a ConflictBranch", branchID)))
			return
		}

		branchIDsPerConflictID := make(map[ledgerstate.ConflictID][]ledgerstate.BranchID)
		for conflictID := range branch.(*ledgerstate.ConflictBranch).Conflicts() {
			branchIDsPerConflictID[conflictID] = make([]ledgerstate.BranchID, 0)
			messagelayer.Tangle().LedgerState.BranchDAG.ConflictMembers(conflictID).Consume(func(conflictMember *ledgerstate.ConflictMember) {
				branchIDsPerConflictID[conflictID] = append(branchIDsPerConflictID[conflictID], conflictMember.BranchID())
			})
		}

		err = c.JSON(http.StatusOK, NewGetBranchConflictsResponse(branchID, branchIDsPerConflictID))
	}) {
		return
	}

	return c.JSON(http.StatusNotFound, jsonmodels.NewErrorResponse(fmt.Errorf("failed to load Branch with %s", branchID)))
}

// GetBranchConflictsResponse represents the JSON model of a collection of Conflicts that a ledgerstate.ConflictBranch is part of.
type GetBranchConflictsResponse struct {
	BranchID  string                 `json:"branchID"`
	Conflicts []*jsonmodels.Conflict `json:"conflicts"`
}

// NewGetBranchConflictsResponse returns the GetBranchConflictsResponse that a ledgerstate.ConflictBranch is part of.
func NewGetBranchConflictsResponse(branchID ledgerstate.BranchID, branchIDsPerConflictID map[ledgerstate.ConflictID][]ledgerstate.BranchID) *GetBranchConflictsResponse {
	return &GetBranchConflictsResponse{
		BranchID: branchID.Base58(),
		Conflicts: func() (mappedConflicts []*jsonmodels.Conflict) {
			mappedConflicts = make([]*jsonmodels.Conflict, 0)
			for conflictID, branchIDs := range branchIDsPerConflictID {
				mappedConflicts = append(mappedConflicts, jsonmodels.NewConflict(conflictID, branchIDs))
			}

			return
		}(),
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region GetOutput ////////////////////////////////////////////////////////////////////////////////////////////////////

// GetOutput is the handler for the /ledgerstate/outputs/:outputID endpoint.
func GetOutput(c echo.Context) (err error) {
	outputID, err := ledgerstate.OutputIDFromBase58(c.Param("outputID"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels.NewErrorResponse(err))
	}

	if !messagelayer.Tangle().LedgerState.Output(outputID).Consume(func(output ledgerstate.Output) {
		err = c.JSON(http.StatusOK, jsonmodels.NewOutput(output))
	}) {
		return c.JSON(http.StatusNotFound, jsonmodels.NewErrorResponse(xerrors.Errorf("failed to load Output with %s", outputID)))
	}

	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region GetOutputConsumers ///////////////////////////////////////////////////////////////////////////////////////////

// GetOutputConsumers is the handler for the /ledgerstate/outputs/:outputID/consumers endpoint.
func GetOutputConsumers(c echo.Context) (err error) {
	outputID, err := ledgerstate.OutputIDFromBase58(c.Param("outputID"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels.NewErrorResponse(err))
	}

	cachedConsumers := messagelayer.Tangle().LedgerState.Consumers(outputID)
	defer cachedConsumers.Release()

	return c.JSON(http.StatusOK, NewGetOutputConsumersResponse(outputID, cachedConsumers.Unwrap()))
}

// GetOutputConsumersResponse is the JSON model of a collection of Consumers of an Output.
type GetOutputConsumersResponse struct {
	OutputID  *jsonmodels.OutputID   `json:"outputID"`
	Consumers []*jsonmodels.Consumer `json:"consumers"`
}

// NewGetOutputConsumersResponse creates an GetOutputConsumersResponse object from the given details.
func NewGetOutputConsumersResponse(outputID ledgerstate.OutputID, consumers []*ledgerstate.Consumer) *GetOutputConsumersResponse {
	return &GetOutputConsumersResponse{
		OutputID: jsonmodels.NewOutputID(outputID),
		Consumers: func() []*jsonmodels.Consumer {
			consumingTransactions := make([]*jsonmodels.Consumer, 0)
			for _, consumer := range consumers {
				if consumer != nil {
					consumingTransactions = append(consumingTransactions, jsonmodels.NewConsumer(consumer))
				}
			}

			return consumingTransactions
		}(),
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region GetOutputMetadata ////////////////////////////////////////////////////////////////////////////////////////////

// GetOutputMetadata is the handler for the /ledgerstate/outputs/:outputID/metadata endpoint.
func GetOutputMetadata(c echo.Context) (err error) {
	outputID, err := ledgerstate.OutputIDFromBase58(c.Param("outputID"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels.NewErrorResponse(err))
	}

	if !messagelayer.Tangle().LedgerState.OutputMetadata(outputID).Consume(func(outputMetadata *ledgerstate.OutputMetadata) {
		err = c.JSON(http.StatusOK, jsonmodels.NewOutputMetadata(outputMetadata))
	}) {
		return c.JSON(http.StatusNotFound, jsonmodels.NewErrorResponse(xerrors.Errorf("failed to load OutputMetadata with %s", outputID)))
	}

	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region GetTransaction ///////////////////////////////////////////////////////////////////////////////////////////////

// GetTransaction is the handler for /ledgerstate/transactions/:transactionID endpoint.
func GetTransaction(c echo.Context) (err error) {
	transactionID, err := ledgerstate.TransactionIDFromBase58(c.Param("transactionID"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels.NewErrorResponse(err))
	}

	if !messagelayer.Tangle().LedgerState.Transaction(transactionID).Consume(func(transaction *ledgerstate.Transaction) {
		err = c.JSON(http.StatusOK, jsonmodels.NewTransaction(transaction))
	}) {
		err = c.JSON(http.StatusNotFound, jsonmodels.NewErrorResponse(xerrors.Errorf("failed to load Transaction with %s", transactionID)))
	}

	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region GetTransactionMetadata ///////////////////////////////////////////////////////////////////////////////////////

// GetTransactionMetadata is the handler for ledgerstate/transactions/:transactionID/metadata
func GetTransactionMetadata(c echo.Context) (err error) {
	transactionID, err := ledgerstate.TransactionIDFromBase58(c.Param("transactionID"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels.NewErrorResponse(err))
	}

	if !messagelayer.Tangle().LedgerState.TransactionMetadata(transactionID).Consume(func(transactionMetadata *ledgerstate.TransactionMetadata) {
		err = c.JSON(http.StatusOK, jsonmodels.NewTransactionMetadata(transactionMetadata))
	}) {
		return c.JSON(http.StatusNotFound, jsonmodels.NewErrorResponse(xerrors.Errorf("failed to load TransactionMetadata of Transaction with %s", transactionID)))
	}

	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region GetTransactionAttachments ////////////////////////////////////////////////////////////////////////////////////

// GetTransactionAttachments is the handler for ledgerstate/transactions/:transactionID/attachments endpoint.
func GetTransactionAttachments(c echo.Context) (err error) {
	transactionID, err := ledgerstate.TransactionIDFromBase58(c.Param("transactionID"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels.NewErrorResponse(err))
	}

	var messageIDs tangle.MessageIDs
	if !messagelayer.Tangle().Storage.Attachments(transactionID).Consume(func(attachment *tangle.Attachment) {
		messageIDs = append(messageIDs, attachment.MessageID())
	}) {
		return c.JSON(http.StatusNotFound, jsonmodels.NewErrorResponse(xerrors.Errorf("failed to load GetTransactionAttachmentsResponse of Transaction with %s", transactionID)))
	}

	return c.JSON(http.StatusOK, jsonmodels.NewGetTransactionAttachmentsResponse(transactionID, messageIDs))
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region branchIDFromContext //////////////////////////////////////////////////////////////////////////////////////////

// branchIDFromContext determines the BranchID from the branchID parameter in an echo.Context. It expects it to either
// be a base58 encoded string or one of the builtin aliases (MasterBranchID, LazyBookedConflictsBranchID or
// InvalidBranchID)
func branchIDFromContext(c echo.Context) (branchID ledgerstate.BranchID, err error) {
	switch branchIDString := c.Param("branchID"); branchIDString {
	case "MasterBranchID":
		branchID = ledgerstate.MasterBranchID
	case "LazyBookedConflictsBranchID":
		branchID = ledgerstate.LazyBookedConflictsBranchID
	case "InvalidBranchID":
		branchID = ledgerstate.InvalidBranchID
	default:
		branchID, err = ledgerstate.BranchIDFromBase58(branchIDString)
	}

	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
