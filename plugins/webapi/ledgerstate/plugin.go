package ledgerstate

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/node"
	"github.com/labstack/echo"

	"github.com/iotaledger/goshimmer/packages/clock"
	"github.com/iotaledger/goshimmer/packages/consensus/fcob"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/mana"
	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
	"github.com/iotaledger/goshimmer/plugins/webapi"
	"github.com/iotaledger/goshimmer/plugins/webapi/jsonmodels"
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
			webapi.Server().POST("ledgerstate/addresses/unspentOutputs", PostAddressUnspentOutputs)
			webapi.Server().GET("ledgerstate/branches/:branchID", GetBranch)
			webapi.Server().GET("ledgerstate/branches/:branchID/children", GetBranchChildren)
			webapi.Server().GET("ledgerstate/branches/:branchID/conflicts", GetBranchConflicts)
			webapi.Server().GET("ledgerstate/outputs/:outputID", GetOutput)
			webapi.Server().GET("ledgerstate/outputs/:outputID/consumers", GetOutputConsumers)
			webapi.Server().GET("ledgerstate/outputs/:outputID/metadata", GetOutputMetadata)
			webapi.Server().GET("ledgerstate/transactions/:transactionID", GetTransaction)
			webapi.Server().GET("ledgerstate/transactions/:transactionID/metadata", GetTransactionMetadata)
			webapi.Server().GET("ledgerstate/transactions/:transactionID/inclusionState", GetTransactionInclusionState)
			webapi.Server().GET("ledgerstate/transactions/:transactionID/consensus", GetTransactionConsensusMetadata)
			webapi.Server().GET("ledgerstate/transactions/:transactionID/attachments", GetTransactionAttachments)
			webapi.Server().POST("ledgerstate/transactions", PostTransaction)
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

	cachedOutputs := messagelayer.Tangle().LedgerState.CachedOutputsOnAddress(address)
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

	cachedOutputs := messagelayer.Tangle().LedgerState.CachedOutputsOnAddress(address)
	defer cachedOutputs.Release()

	return c.JSON(http.StatusOK, jsonmodels.NewGetAddressResponse(address, cachedOutputs.Unwrap().Filter(func(output ledgerstate.Output) (isUnspent bool) {
		messagelayer.Tangle().LedgerState.CachedOutputMetadata(output.ID()).Consume(func(outputMetadata *ledgerstate.OutputMetadata) {
			isUnspent = outputMetadata.ConsumerCount() == 0
		})

		return
	})))
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region PostAddressUnspentOutputs /////////////////////////////////////////////////////////////////////////////////////

// PostAddressUnspentOutputs is the handler for the /ledgerstate/addresses/unspentOutputs endpoint.
func PostAddressUnspentOutputs(c echo.Context) error {
	req := &jsonmodels.PostAddressesUnspentOutputsRequest{}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels.NewErrorResponse(err))
	}
	addresses := make([]ledgerstate.Address, len(req.Addresses))
	for i, addressString := range req.Addresses {
		var err error
		addresses[i], err = ledgerstate.AddressFromBase58EncodedString(addressString)
		if err != nil {
			return c.JSON(http.StatusBadRequest, jsonmodels.NewErrorResponse(err))
		}
	}

	res := &jsonmodels.PostAddressesUnspentOutputsResponse{
		UnspentOutputs: make([]*jsonmodels.WalletOutputsOnAddress, len(addresses)),
	}
	for i, addy := range addresses {
		res.UnspentOutputs[i] = &jsonmodels.WalletOutputsOnAddress{}
		cachedOutputs := messagelayer.Tangle().LedgerState.OutputsOnAddress(addy)
		res.UnspentOutputs[i].Address = jsonmodels.Address{
			Type:   addy.Type().String(),
			Base58: addy.Base58(),
		}
		res.UnspentOutputs[i].Outputs = make([]jsonmodels.WalletOutput, 0)

		for _, output := range cachedOutputs.Unwrap().Filter(func(output ledgerstate.Output) (isUnspent bool) {
			messagelayer.Tangle().LedgerState.OutputMetadata(output.ID()).Consume(func(outputMetadata *ledgerstate.OutputMetadata) {
				isUnspent = outputMetadata.ConsumerCount() == 0
			})
			return
		}) {
			cachedOutputMetadata := messagelayer.Tangle().LedgerState.OutputMetadata(output.ID())
			cachedOutputMetadata.Consume(func(outputMetadata *ledgerstate.OutputMetadata) {
				if outputMetadata.ConsumerCount() == 0 {
					inclusionState := jsonmodels.InclusionState{}
					txID := output.ID().TransactionID()
					txInclusionState, err := messagelayer.Tangle().LedgerState.TransactionInclusionState(txID)
					if err != nil {
						return
					}
					inclusionState.Confirmed = txInclusionState == ledgerstate.Confirmed
					inclusionState.Rejected = txInclusionState == ledgerstate.Rejected
					inclusionState.Conflicting = len(messagelayer.Tangle().LedgerState.ConflictSet(txID)) != 0

					cachedTx := messagelayer.Tangle().LedgerState.Transaction(output.ID().TransactionID())
					var timestamp time.Time
					cachedTx.Consume(func(tx *ledgerstate.Transaction) {
						timestamp = tx.Essence().Timestamp()
					})
					res.UnspentOutputs[i].Outputs = append(res.UnspentOutputs[i].Outputs, jsonmodels.WalletOutput{
						Output:         *jsonmodels.NewOutput(output),
						InclusionState: inclusionState,
						Metadata:       jsonmodels.WalletOutputMetadata{Timestamp: timestamp},
					})
				}
			})
		}
		cachedOutputs.Release()
	}

	return c.JSON(http.StatusOK, res)
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

	return c.JSON(http.StatusOK, jsonmodels.NewGetBranchChildrenResponse(branchID, cachedChildBranches.Unwrap()))
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

		err = c.JSON(http.StatusOK, jsonmodels.NewGetBranchConflictsResponse(branchID, branchIDsPerConflictID))
	}) {
		return
	}

	return c.JSON(http.StatusNotFound, jsonmodels.NewErrorResponse(fmt.Errorf("failed to load Branch with %s", branchID)))
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region GetOutput ////////////////////////////////////////////////////////////////////////////////////////////////////

// GetOutput is the handler for the /ledgerstate/outputs/:outputID endpoint.
func GetOutput(c echo.Context) (err error) {
	outputID, err := ledgerstate.OutputIDFromBase58(c.Param("outputID"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels.NewErrorResponse(err))
	}

	if !messagelayer.Tangle().LedgerState.CachedOutput(outputID).Consume(func(output ledgerstate.Output) {
		err = c.JSON(http.StatusOK, jsonmodels.NewOutput(output))
	}) {
		return c.JSON(http.StatusNotFound, jsonmodels.NewErrorResponse(errors.Errorf("failed to load Output with %s", outputID)))
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

	return c.JSON(http.StatusOK, jsonmodels.NewGetOutputConsumersResponse(outputID, cachedConsumers.Unwrap()))
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region GetOutputMetadata ////////////////////////////////////////////////////////////////////////////////////////////

// GetOutputMetadata is the handler for the /ledgerstate/outputs/:outputID/metadata endpoint.
func GetOutputMetadata(c echo.Context) (err error) {
	outputID, err := ledgerstate.OutputIDFromBase58(c.Param("outputID"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels.NewErrorResponse(err))
	}

	if !messagelayer.Tangle().LedgerState.CachedOutputMetadata(outputID).Consume(func(outputMetadata *ledgerstate.OutputMetadata) {
		err = c.JSON(http.StatusOK, jsonmodels.NewOutputMetadata(outputMetadata))
	}) {
		return c.JSON(http.StatusNotFound, jsonmodels.NewErrorResponse(errors.Errorf("failed to load OutputMetadata with %s", outputID)))
	}

	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region GetTransaction ///////////////////////////////////////////////////////////////////////////////////////////////

// GetTransaction is the handler for the /ledgerstate/transactions/:transactionID endpoint.
func GetTransaction(c echo.Context) (err error) {
	transactionID, err := ledgerstate.TransactionIDFromBase58(c.Param("transactionID"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels.NewErrorResponse(err))
	}

	if !messagelayer.Tangle().LedgerState.Transaction(transactionID).Consume(func(transaction *ledgerstate.Transaction) {
		err = c.JSON(http.StatusOK, jsonmodels.NewTransaction(transaction))
	}) {
		err = c.JSON(http.StatusNotFound, jsonmodels.NewErrorResponse(errors.Errorf("failed to load Transaction with %s", transactionID)))
	}

	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region GetTransactionMetadata ///////////////////////////////////////////////////////////////////////////////////////

// GetTransactionMetadata is the handler for the ledgerstate/transactions/:transactionID/metadata endpoint.
func GetTransactionMetadata(c echo.Context) (err error) {
	transactionID, err := ledgerstate.TransactionIDFromBase58(c.Param("transactionID"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels.NewErrorResponse(err))
	}

	if !messagelayer.Tangle().LedgerState.TransactionMetadata(transactionID).Consume(func(transactionMetadata *ledgerstate.TransactionMetadata) {
		err = c.JSON(http.StatusOK, jsonmodels.NewTransactionMetadata(transactionMetadata))
	}) {
		return c.JSON(http.StatusNotFound, jsonmodels.NewErrorResponse(errors.Errorf("failed to load TransactionMetadata of Transaction with %s", transactionID)))
	}

	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region GetTransactionInclusionState /////////////////////////////////////////////////////////////////////////////////

// GetTransactionInclusionState is the handler for the ledgerstate/transactions/:transactionID/inclusionState endpoint.
func GetTransactionInclusionState(c echo.Context) (err error) {
	transactionID, err := ledgerstate.TransactionIDFromBase58(c.Param("transactionID"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels.NewErrorResponse(err))
	}

	inclusionState, err := messagelayer.Tangle().LedgerState.TransactionInclusionState(transactionID)
	if err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels.NewErrorResponse(err))
	}

	return c.JSON(http.StatusOK, jsonmodels.NewTransactionInclusionState(inclusionState, transactionID))
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region GetTransactionConsensusMetadata //////////////////////////////////////////////////////////////////////////////

// GetTransactionConsensusMetadata is the handler for the ledgerstate/transactions/:transactionID/consensus endpoint.
func GetTransactionConsensusMetadata(c echo.Context) (err error) {
	transactionID, err := ledgerstate.TransactionIDFromBase58(c.Param("transactionID"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels.NewErrorResponse(err))
	}

	consensusMechanism := messagelayer.Tangle().Options.ConsensusMechanism.(*fcob.ConsensusMechanism)
	if consensusMechanism != nil {
		if consensusMechanism.Storage.Opinion(transactionID).Consume(func(opinion *fcob.Opinion) {
			err = c.JSON(http.StatusOK, jsonmodels.NewTransactionConsensusMetadata(transactionID, opinion))
		}) {
			return
		}
	}

	return c.JSON(http.StatusNotFound, jsonmodels.NewErrorResponse(errors.Errorf("failed to load TransactionConsensusMetadata of Transaction with %s", transactionID)))
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region GetTransactionAttachments ////////////////////////////////////////////////////////////////////////////////////

// GetTransactionAttachments is the handler for the ledgerstate/transactions/:transactionID/attachments endpoint.
func GetTransactionAttachments(c echo.Context) (err error) {
	transactionID, err := ledgerstate.TransactionIDFromBase58(c.Param("transactionID"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels.NewErrorResponse(err))
	}

	var messageIDs tangle.MessageIDs
	if !messagelayer.Tangle().Storage.Attachments(transactionID).Consume(func(attachment *tangle.Attachment) {
		messageIDs = append(messageIDs, attachment.MessageID())
	}) {
		return c.JSON(http.StatusNotFound, jsonmodels.NewErrorResponse(errors.Errorf("failed to load GetTransactionAttachmentsResponse of Transaction with %s", transactionID)))
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

// region postTransaction //////////////////////////////////////////////////////////////////////////////////////////////

const maxBookedAwaitTime = 5 * time.Second

var (
	sendTxMu sync.Mutex
	// ErrNotAllowedToPledgeManaToNode defines an unsupported node to pledge mana to.
	ErrNotAllowedToPledgeManaToNode = errors.New("not allowed to pledge mana to node")
)

// PostTransaction sends a transaction.
func PostTransaction(c echo.Context) error {
	sendTxMu.Lock()
	defer sendTxMu.Unlock()

	var request jsonmodels.PostTransactionRequest
	if err := c.Bind(&request); err != nil {
		return c.JSON(http.StatusBadRequest, &jsonmodels.PostTransactionResponse{Error: err.Error()})
	}

	// parse tx
	tx, _, err := ledgerstate.TransactionFromBytes(request.TransactionBytes)
	if err != nil {
		return c.JSON(http.StatusBadRequest, &jsonmodels.PostTransactionResponse{Error: err.Error()})
	}

	// validate allowed mana pledge nodes.
	allowedAccessMana := messagelayer.GetAllowedPledgeNodes(mana.AccessMana)
	if allowedAccessMana.IsFilterEnabled {
		if !allowedAccessMana.Allowed.Has(tx.Essence().AccessPledgeID()) {
			return c.JSON(http.StatusBadRequest, &jsonmodels.PostTransactionResponse{
				Error: fmt.Errorf("not allowed to pledge access mana to %s: %w", tx.Essence().AccessPledgeID().String(), ErrNotAllowedToPledgeManaToNode).Error(),
			})
		}
	}
	allowedConsensusMana := messagelayer.GetAllowedPledgeNodes(mana.ConsensusMana)
	if allowedConsensusMana.IsFilterEnabled {
		if !allowedConsensusMana.Allowed.Has(tx.Essence().ConsensusPledgeID()) {
			return c.JSON(http.StatusBadRequest, &jsonmodels.PostTransactionResponse{
				Error: fmt.Errorf("not allowed to pledge consensus mana to %s: %w", tx.Essence().ConsensusPledgeID().String(), ErrNotAllowedToPledgeManaToNode).Error(),
			})
		}
	}

	// check transaction validity
	if transactionErr := messagelayer.Tangle().LedgerState.CheckTransaction(tx); transactionErr != nil {
		return c.JSON(http.StatusBadRequest, &jsonmodels.PostTransactionResponse{Error: transactionErr.Error()})
	}

	// check if transaction is too old
	if tx.Essence().Timestamp().Before(clock.SyncedTime().Add(-tangle.MaxReattachmentTimeMin)) {
		return c.JSON(http.StatusBadRequest, &jsonmodels.PostTransactionResponse{Error: fmt.Sprintf("transaction timestamp is older than MaxReattachmentTime (%s) and cannot be issued", tangle.MaxReattachmentTimeMin)})
	}

	// if transaction is in the future we wait until the time arrives
	if tx.Essence().Timestamp().After(clock.SyncedTime()) {
		time.Sleep(tx.Essence().Timestamp().Sub(clock.SyncedTime()) + 1*time.Nanosecond)
	}

	issueTransaction := func() (*tangle.Message, error) {
		return messagelayer.Tangle().IssuePayload(tx)
	}

	if _, err := messagelayer.AwaitMessageToBeBooked(issueTransaction, tx.ID(), maxBookedAwaitTime); err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels.PostTransactionResponse{Error: err.Error()})
	}
	return c.JSON(http.StatusOK, &jsonmodels.PostTransactionResponse{TransactionID: tx.ID().Base58()})
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
