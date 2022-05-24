package ledgerstate

import (
	"context"
	"fmt"
	"github.com/iotaledger/goshimmer/plugins/webapi"
	"net/http"
	"sync"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
	"github.com/labstack/echo"
	"go.uber.org/dig"

	"github.com/iotaledger/goshimmer/packages/clock"
	"github.com/iotaledger/goshimmer/packages/jsonmodels"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/mana"
	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
)

// region Plugin ///////////////////////////////////////////////////////////////////////////////////////////////////////

// PluginName is the name of the web API plugin.
const (
	PluginName                       = "WebAPILedgerstateEndpoint"
	DoubleSpendFilterCleanupInterval = 10 * time.Second
)

type dependencies struct {
	dig.In

	Server *echo.Echo
	Tangle *tangle.Tangle
}

var (
	// Plugin holds the singleton instance of the plugin.
	Plugin *node.Plugin

	deps = new(dependencies)

	// filterEnabled whether doubleSpendFilter is enabled.
	filterEnabled bool

	// doubleSpendFilter helps to filter out double spends locally.
	doubleSpendFilter *DoubleSpendFilter

	// doubleSpendFilterOnce ensures that doubleSpendFilter is a singleton.
	doubleSpendFilterOnce sync.Once

	// closure to be executed on transaction confirmation.
	onTransactionConfirmed *events.Closure

	log *logger.Logger
)

func init() {
	Plugin = node.NewPlugin(PluginName, deps, node.Enabled, configure, run)
}

// Filter returns the double spend filter singleton.
func Filter() *DoubleSpendFilter {
	doubleSpendFilterOnce.Do(func() {
		doubleSpendFilter = NewDoubleSpendFilter()
	})
	return doubleSpendFilter
}

// FilterHasConflict checks if the outputs are conflicting if doubleSpendFilter is enabled.
func FilterHasConflict(outputs ledgerstate.Inputs) (bool, ledgerstate.TransactionID) {
	if filterEnabled {
		has, conflictingID := doubleSpendFilter.HasConflict(outputs)
		return has, conflictingID
	}
	return false, ledgerstate.TransactionID{}
}

// FilterAdd Adds transaction to the doubleSpendFilter if it is enabled.
func FilterAdd(tx *ledgerstate.Transaction) {
	if filterEnabled {
		doubleSpendFilter.Add(tx)
	}
}

// FilterRemove Removes transaction id from the doubleSpendFilter if it is enabled.
func FilterRemove(txID ledgerstate.TransactionID) {
	if filterEnabled {
		doubleSpendFilter.Remove(txID)
	}
}

func configure(_ *node.Plugin) {
	filterEnabled = webapi.Parameters.EnableDSFilter
	if filterEnabled {
		doubleSpendFilter = Filter()
		onTransactionConfirmed = events.NewClosure(func(transactionID ledgerstate.TransactionID) {
			doubleSpendFilter.Remove(transactionID)
		})
	}
	deps.Tangle.ConfirmationOracle.Events().TransactionConfirmed.Attach(onTransactionConfirmed)
	log = logger.NewLogger(PluginName)
}

func run(*node.Plugin) {
	if filterEnabled {
		if err := daemon.BackgroundWorker("WebAPIDoubleSpendFilter", worker, shutdown.PriorityWebAPI); err != nil {
			log.Panicf("Failed to start as daemon: %s", err)
		}
	}

	// register endpoints
	deps.Server.GET("ledgerstate/addresses/:address", GetAddress)
	deps.Server.GET("ledgerstate/addresses/:address/unspentOutputs", GetAddressUnspentOutputs)
	deps.Server.POST("ledgerstate/addresses/unspentOutputs", PostAddressUnspentOutputs)
	deps.Server.GET("ledgerstate/branches/:branchID", GetBranch)
	deps.Server.GET("ledgerstate/branches/:branchID/children", GetBranchChildren)
	deps.Server.GET("ledgerstate/branches/:branchID/conflicts", GetBranchConflicts)
	deps.Server.GET("ledgerstate/branches/:branchID/voters", GetBranchVoters)
	deps.Server.GET("ledgerstate/branches/:branchID/sequenceids", GetBranchSequenceIDs)
	deps.Server.GET("ledgerstate/outputs/:outputID", GetOutput)
	deps.Server.GET("ledgerstate/outputs/:outputID/consumers", GetOutputConsumers)
	deps.Server.GET("ledgerstate/outputs/:outputID/metadata", GetOutputMetadata)
	deps.Server.GET("ledgerstate/transactions/:transactionID", GetTransaction)
	deps.Server.GET("ledgerstate/transactions/:transactionID/metadata", GetTransactionMetadata)
	deps.Server.GET("ledgerstate/transactions/:transactionID/attachments", GetTransactionAttachments)
	deps.Server.POST("ledgerstate/transactions", PostTransaction)
}

func worker(ctx context.Context) {
	defer log.Infof("Stopping %s ... done", PluginName)
	func() {
		ticker := time.NewTicker(DoubleSpendFilterCleanupInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				doubleSpendFilter.CleanUp()
			}
		}
	}()
	log.Infof("Stopping %s ...", PluginName)
	deps.Tangle.ConfirmationOracle.Events().TransactionConfirmed.Detach(onTransactionConfirmed)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region GetAddress ///////////////////////////////////////////////////////////////////////////////////////////////////

// GetAddress is the handler for the /ledgerstate/addresses/:address endpoint.
func GetAddress(c echo.Context) error {
	address, err := ledgerstate.AddressFromBase58EncodedString(c.Param("address"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels.NewErrorResponse(err))
	}

	cachedOutputs := deps.Tangle.LedgerState.CachedOutputsOnAddress(address)
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

	cachedOutputs := deps.Tangle.LedgerState.CachedOutputsOnAddress(address)
	defer cachedOutputs.Release()

	return c.JSON(http.StatusOK, jsonmodels.NewGetAddressResponse(address, ledgerstate.Outputs(cachedOutputs.Unwrap()).Filter(func(output ledgerstate.Output) (isUnspent bool) {
		deps.Tangle.LedgerState.CachedOutputMetadata(output.ID()).Consume(func(outputMetadata *ledgerstate.OutputMetadata) {
			isUnspent = outputMetadata.ConsumerCount() == 0
		})

		return
	})))
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region PostAddressUnspentOutputs /////////////////////////////////////////////////////////////////////////////////////

// PostAddressUnspentOutputs is the handler for the /ledgerstate/addresses/unspentOutputs endpoint.
func PostAddressUnspentOutputs(c echo.Context) error {
	req := new(jsonmodels.PostAddressesUnspentOutputsRequest)
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
		res.UnspentOutputs[i] = new(jsonmodels.WalletOutputsOnAddress)
		cachedOutputs := deps.Tangle.LedgerState.CachedOutputsOnAddress(addy)
		res.UnspentOutputs[i].Address = jsonmodels.Address{
			Type:   addy.Type().String(),
			Base58: addy.Base58(),
		}
		res.UnspentOutputs[i].Outputs = make([]jsonmodels.WalletOutput, 0)

		for _, output := range ledgerstate.Outputs(cachedOutputs.Unwrap()).Filter(func(output ledgerstate.Output) (isUnspent bool) {
			deps.Tangle.LedgerState.CachedOutputMetadata(output.ID()).Consume(func(outputMetadata *ledgerstate.OutputMetadata) {
				isUnspent = outputMetadata.ConsumerCount() == 0
			})
			return
		}) {
			cachedOutputMetadata := deps.Tangle.LedgerState.CachedOutputMetadata(output.ID())
			cachedOutputMetadata.Consume(func(outputMetadata *ledgerstate.OutputMetadata) {
				if outputMetadata.ConsumerCount() == 0 {
					cachedTx := deps.Tangle.LedgerState.Transaction(output.ID().TransactionID())
					var timestamp time.Time
					cachedTx.Consume(func(tx *ledgerstate.Transaction) {
						timestamp = tx.Essence().Timestamp()
					})
					res.UnspentOutputs[i].Outputs = append(res.UnspentOutputs[i].Outputs, jsonmodels.WalletOutput{
						Output:          *jsonmodels.NewOutput(output),
						GradeOfFinality: outputMetadata.GradeOfFinality(),
						Metadata:        jsonmodels.WalletOutputMetadata{Timestamp: timestamp},
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

	if deps.Tangle.LedgerState.BranchDAG.Branch(branchID).Consume(func(branch *ledgerstate.Branch) {
		branchGoF, _ := deps.Tangle.LedgerState.UTXODAG.BranchGradeOfFinality(branch.ID())

		err = c.JSON(http.StatusOK, jsonmodels.NewBranch(branch, branchGoF, deps.Tangle.ApprovalWeightManager.WeightOfBranch(branchID)))
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

	cachedChildBranches := deps.Tangle.LedgerState.BranchDAG.ChildBranches(branchID)
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

	if deps.Tangle.LedgerState.BranchDAG.Branch(branchID).Consume(func(branch *ledgerstate.Branch) {
		branchIDsPerConflictID := make(map[ledgerstate.ConflictID][]ledgerstate.BranchID)
		for conflictID := range branch.Conflicts() {
			branchIDsPerConflictID[conflictID] = make([]ledgerstate.BranchID, 0)
			deps.Tangle.LedgerState.BranchDAG.ConflictMembers(conflictID).Consume(func(conflictMember *ledgerstate.ConflictMember) {
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

// region GetBranchVoters ///////////////////////////////////////////////////////////////////////////////////////////////

// GetBranchVoters is the handler for the /ledgerstate/branches/:branchID/voters endpoint.
func GetBranchVoters(c echo.Context) (err error) {
	branchID, err := branchIDFromContext(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels.NewErrorResponse(err))
	}

	voters := tangle.NewVoters()
	voters.AddAll(deps.Tangle.ApprovalWeightManager.VotersOfBranch(branchID))

	return c.JSON(http.StatusOK, jsonmodels.NewGetBranchVotersResponse(branchID, voters))
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region GetBranchSequenceIDs /////////////////////////////////////////////////////////////////////////////////////////

// GetBranchSequenceIDs is the handler for the /ledgerstate/branch/:branchID endpoint.
func GetBranchSequenceIDs(c echo.Context) (err error) {
	// branchID, err := branchIDFromContext(c)
	// if err != nil {
	//	return c.JSON(http.StatusBadRequest, jsonmodels.NewErrorResponse(err))
	// }
	//
	// sequenceIDs := make([]string, 0)
	// deps.Tangle.Booker.MarkersManager.SequenceAliasMapping(markers.NewSequenceAlias(branchID.Bytes())).Consume(func(sequenceAliasMapping *markers.SequenceAliasMapping) {
	//	sequenceAliasMapping.ForEachSequenceID(func(sequenceID markers.SequenceID) bool {
	//		sequenceIDs = append(sequenceIDs, strconv.FormatUint(uint64(sequenceID), 10))
	//		return true
	//	})
	// })

	return c.JSON(http.StatusOK, "ok")
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region GetOutput ////////////////////////////////////////////////////////////////////////////////////////////////////

// GetOutput is the handler for the /ledgerstate/outputs/:outputID endpoint.
func GetOutput(c echo.Context) (err error) {
	outputID, err := ledgerstate.OutputIDFromBase58(c.Param("outputID"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels.NewErrorResponse(err))
	}

	if !deps.Tangle.LedgerState.CachedOutput(outputID).Consume(func(output ledgerstate.Output) {
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

	cachedConsumers := deps.Tangle.LedgerState.Consumers(outputID)
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

	if !deps.Tangle.LedgerState.CachedOutputMetadata(outputID).Consume(func(outputMetadata *ledgerstate.OutputMetadata) {
		confirmedConsumerID := deps.Tangle.LedgerState.ConfirmedConsumer(outputID)

		jsonOutputMetadata := jsonmodels.NewOutputMetadata(outputMetadata, confirmedConsumerID)

		err = c.JSON(http.StatusOK, jsonOutputMetadata)
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

	var tx *ledgerstate.Transaction
	// retrieve transaction
	if !deps.Tangle.LedgerState.Transaction(transactionID).Consume(func(transaction *ledgerstate.Transaction) {
		tx = transaction
	}) {
		err = c.JSON(http.StatusNotFound, jsonmodels.NewErrorResponse(errors.Errorf("failed to load Transaction with %s", transactionID)))
		return
	}
	return c.JSON(http.StatusOK, jsonmodels.NewTransaction(tx))
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region GetTransactionMetadata ///////////////////////////////////////////////////////////////////////////////////////

// GetTransactionMetadata is the handler for the ledgerstate/transactions/:transactionID/metadata endpoint.
func GetTransactionMetadata(c echo.Context) (err error) {
	transactionID, err := ledgerstate.TransactionIDFromBase58(c.Param("transactionID"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels.NewErrorResponse(err))
	}

	if !deps.Tangle.LedgerState.TransactionMetadata(transactionID).Consume(func(transactionMetadata *ledgerstate.TransactionMetadata) {
		err = c.JSON(http.StatusOK, jsonmodels.NewTransactionMetadata(transactionMetadata))
	}) {
		return c.JSON(http.StatusNotFound, jsonmodels.NewErrorResponse(errors.Errorf("failed to load TransactionMetadata of Transaction with %s", transactionID)))
	}

	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region GetTransactionAttachments ////////////////////////////////////////////////////////////////////////////////////

// GetTransactionAttachments is the handler for the ledgerstate/transactions/:transactionID/attachments endpoint.
func GetTransactionAttachments(c echo.Context) (err error) {
	transactionID, err := ledgerstate.TransactionIDFromBase58(c.Param("transactionID"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels.NewErrorResponse(err))
	}

	messageIDs := tangle.NewMessageIDs()
	if !deps.Tangle.Storage.Attachments(transactionID).Consume(func(attachment *tangle.Attachment) {
		messageIDs.Add(attachment.MessageID())
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
	default:
		branchID, err = ledgerstate.BranchIDFromBase58(branchIDString)
	}

	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region PostTransaction //////////////////////////////////////////////////////////////////////////////////////////////

const maxBookedAwaitTime = 5 * time.Second

// ErrNotAllowedToPledgeManaToNode defines an unsupported node to pledge mana to.
var ErrNotAllowedToPledgeManaToNode = errors.New("not allowed to pledge mana to node")

// PostTransaction sends a transaction.
func PostTransaction(c echo.Context) error {
	var request jsonmodels.PostTransactionRequest
	if err := c.Bind(&request); err != nil {
		return c.JSON(http.StatusBadRequest, &jsonmodels.PostTransactionResponse{Error: err.Error()})
	}

	// parse tx
	tx, err := new(ledgerstate.Transaction).FromBytes(request.TransactionBytes)
	if err != nil {
		return c.JSON(http.StatusBadRequest, &jsonmodels.PostTransactionResponse{Error: err.Error()})
	}

	// if filter is enabled check if it would introduce a double spend known to the node locally
	has, conflictingID := FilterHasConflict(tx.Essence().Inputs())
	if has {
		err = errors.Errorf("transaction is conflicting with previously submitted transaction %s", conflictingID.Base58())
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
	if transactionErr := deps.Tangle.LedgerState.CheckTransaction(tx); transactionErr != nil {
		return c.JSON(http.StatusBadRequest, &jsonmodels.PostTransactionResponse{Error: transactionErr.Error()})
	}

	// check if transaction is too old
	if tx.Essence().Timestamp().Before(clock.SyncedTime().Add(-tangle.MaxReattachmentTimeMin)) {
		return c.JSON(http.StatusBadRequest, &jsonmodels.PostTransactionResponse{Error: fmt.Sprintf("transaction timestamp is older than MaxReattachmentTime (%s) and cannot be issued", tangle.MaxReattachmentTimeMin)})
	}

	// if transaction is in the future we wait until the time arrives
	if tx.Essence().Timestamp().After(clock.SyncedTime()) {
		if tx.Essence().Timestamp().Sub(clock.SyncedTime()) > time.Minute {
			return c.JSON(http.StatusBadRequest, &jsonmodels.PostTransactionResponse{Error: "transaction timestamp is in the future and cannot be issued; please readjust local clock"})
		}
		time.Sleep(tx.Essence().Timestamp().Sub(clock.SyncedTime()) + 1*time.Nanosecond)
	}

	issueTransaction := func() (*tangle.Message, error) {
		return deps.Tangle.IssuePayload(tx)
	}

	// add tx to double spend doubleSpendFilter
	FilterAdd(tx)
	if _, err := messagelayer.AwaitMessageToBeBooked(issueTransaction, tx.ID(), maxBookedAwaitTime); err != nil {
		// if we failed to issue the transaction, we remove it
		FilterRemove(tx.ID())
		return c.JSON(http.StatusBadRequest, jsonmodels.PostTransactionResponse{Error: err.Error()})
	}
	return c.JSON(http.StatusOK, &jsonmodels.PostTransactionResponse{TransactionID: tx.ID().Base58()})
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
