package ledgerstate

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/generics/event"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
	"github.com/labstack/echo"
	"go.uber.org/dig"

	"github.com/iotaledger/goshimmer/packages/clock"
	"github.com/iotaledger/goshimmer/packages/jsonmodels"
	"github.com/iotaledger/goshimmer/packages/ledger"
	"github.com/iotaledger/goshimmer/packages/ledger/branchdag"
	"github.com/iotaledger/goshimmer/packages/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/ledger/vm/devnetvm"
	"github.com/iotaledger/goshimmer/packages/ledger/vm/devnetvm/indexer"
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

	Server  *echo.Echo
	Tangle  *tangle.Tangle
	Indexer *indexer.Indexer
}

var (
	// Plugin holds the singleton instance of the plugin.
	Plugin *node.Plugin

	deps = new(dependencies)

	// doubleSpendFilter helps to filter out double spends locally.
	doubleSpendFilter *DoubleSpendFilter

	// doubleSpendFilterOnce ensures that doubleSpendFilter is a singleton.
	doubleSpendFilterOnce sync.Once

	// closure to be executed on transaction confirmation.
	onTransactionConfirmed *event.Closure[*tangle.TransactionConfirmedEvent]

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

func configure(_ *node.Plugin) {
	doubleSpendFilter = Filter()
	onTransactionConfirmed = event.NewClosure(func(event *tangle.TransactionConfirmedEvent) {
		doubleSpendFilter.Remove(event.TransactionID)
	})
	deps.Tangle.ConfirmationOracle.Events().TransactionConfirmed.Attach(onTransactionConfirmed)
	log = logger.NewLogger(PluginName)
}

func run(*node.Plugin) {
	if err := daemon.BackgroundWorker("WebAPIDoubleSpendFilter", worker, shutdown.PriorityWebAPI); err != nil {
		log.Panicf("Failed to start as daemon: %s", err)
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

func outputsOnAddress(address devnetvm.Address) (outputs devnetvm.Outputs) {
	deps.Indexer.CachedAddressOutputMappings(address).Consume(func(mapping *indexer.AddressOutputMapping) {
		deps.Tangle.Ledger.Storage.CachedOutput(mapping.OutputID()).Consume(func(output *ledger.Output) {
			if typedOutput, ok := output.Output.(*devnetvm.Output); ok {
				outputs = append(outputs, typedOutput)
			}
		})
	})
	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region GetAddress ///////////////////////////////////////////////////////////////////////////////////////////////////

// GetAddress is the handler for the /ledgerstate/addresses/:address endpoint.
func GetAddress(c echo.Context) error {
	address, err := devnetvm.AddressFromBase58EncodedString(c.Param("address"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels.NewErrorResponse(err))
	}

	outputs := outputsOnAddress(address)

	return c.JSON(http.StatusOK, jsonmodels.NewGetAddressResponse(address, outputs))
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region GetAddressUnspentOutputs /////////////////////////////////////////////////////////////////////////////////////

// GetAddressUnspentOutputs is the handler for the /ledgerstate/addresses/:address/unspentOutputs endpoint.
func GetAddressUnspentOutputs(c echo.Context) error {
	address, err := devnetvm.AddressFromBase58EncodedString(c.Param("address"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels.NewErrorResponse(err))
	}

	outputs := outputsOnAddress(address)

	return c.JSON(http.StatusOK, jsonmodels.NewGetAddressResponse(address, outputs.Filter(func(output devnetvm.Output) (isUnspent bool) {
		deps.Tangle.Ledger.Storage.CachedOutputMetadata(output.ID()).Consume(func(outputMetadata *ledger.OutputMetadata) {
			isUnspent = !outputMetadata.IsSpent()
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
	addresses := make([]devnetvm.Address, len(req.Addresses))
	for i, addressString := range req.Addresses {
		var err error
		addresses[i], err = devnetvm.AddressFromBase58EncodedString(addressString)
		if err != nil {
			return c.JSON(http.StatusBadRequest, jsonmodels.NewErrorResponse(err))
		}
	}

	res := &jsonmodels.PostAddressesUnspentOutputsResponse{
		UnspentOutputs: make([]*jsonmodels.WalletOutputsOnAddress, len(addresses)),
	}
	for i, addy := range addresses {
		res.UnspentOutputs[i] = new(jsonmodels.WalletOutputsOnAddress)
		outputs := outputsOnAddress(addy)
		res.UnspentOutputs[i].Address = jsonmodels.Address{
			Type:   addy.Type().String(),
			Base58: addy.Base58(),
		}
		res.UnspentOutputs[i].Outputs = make([]jsonmodels.WalletOutput, 0)

		for _, output := range outputs.Filter(func(output devnetvm.Output) (isUnspent bool) {
			deps.Tangle.Ledger.Storage.CachedOutputMetadata(output.ID()).Consume(func(outputMetadata *ledger.OutputMetadata) {
				isUnspent = !outputMetadata.IsSpent()
			})
			return
		}) {
			deps.Tangle.Ledger.Storage.CachedOutputMetadata(output.ID()).Consume(func(outputMetadata *ledger.OutputMetadata) {
				if !outputMetadata.IsSpent() {
					deps.Tangle.Ledger.Storage.CachedOutput(output.ID()).Consume(func(ledgerOutput *ledger.Output) {
						var timestamp time.Time
						deps.Tangle.Ledger.Storage.CachedTransaction(ledgerOutput.TransactionID()).Consume(func(tx *ledger.Transaction) {
							timestamp = tx.Transaction.(*devnetvm.Transaction).Essence().Timestamp()
						})
						res.UnspentOutputs[i].Outputs = append(res.UnspentOutputs[i].Outputs, jsonmodels.WalletOutput{
							Output:          *jsonmodels.NewOutput(output),
							GradeOfFinality: outputMetadata.GradeOfFinality(),
							Metadata:        jsonmodels.WalletOutputMetadata{Timestamp: timestamp},
						})
					})
				}
			})
		}
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

	if deps.Tangle.Ledger.BranchDAG.Storage.CachedBranch(branchID).Consume(func(branch *branchdag.Branch) {
		branchGoF, _ := deps.Tangle.Ledger.Utils.BranchGradeOfFinality(branch.ID())

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

	cachedChildBranches := deps.Tangle.Ledger.BranchDAG.Storage.CachedChildBranches(branchID)
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

	if deps.Tangle.Ledger.BranchDAG.Storage.CachedBranch(branchID).Consume(func(branch *branchdag.Branch) {
		branchIDsPerConflictID := make(map[branchdag.ConflictID][]branchdag.BranchID)
		for it := branch.ConflictIDs().Iterator(); it.HasNext(); {
			conflictID := it.Next()
			branchIDsPerConflictID[conflictID] = make([]branchdag.BranchID, 0)
			deps.Tangle.Ledger.BranchDAG.Storage.CachedConflictMembers(conflictID).Consume(func(conflictMember *branchdag.ConflictMember) {
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
	// deps.Tangle.booker.MarkersManager.SequenceAliasMapping(markers.NewSequenceAlias(branchID.Bytes())).Consume(func(sequenceAliasMapping *markers.SequenceAliasMapping) {
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
	var outputID utxo.OutputID
	if err = outputID.FromBase58(c.Param("outputID")); err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels.NewErrorResponse(err))
	}

	if !deps.Tangle.Ledger.Storage.CachedOutput(outputID).Consume(func(output *ledger.Output) {
		err = c.JSON(http.StatusOK, jsonmodels.NewOutput(output.Output.(devnetvm.Output)))
	}) {
		return c.JSON(http.StatusNotFound, jsonmodels.NewErrorResponse(errors.Errorf("failed to load Output with %s", outputID)))
	}

	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region GetOutputConsumers ///////////////////////////////////////////////////////////////////////////////////////////

// GetOutputConsumers is the handler for the /ledgerstate/outputs/:outputID/consumers endpoint.
func GetOutputConsumers(c echo.Context) (err error) {
	var outputID utxo.OutputID
	if err = outputID.FromBase58(c.Param("outputID")); err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels.NewErrorResponse(err))
	}

	cachedConsumers := deps.Tangle.Ledger.Storage.CachedConsumers(outputID)
	defer cachedConsumers.Release()

	return c.JSON(http.StatusOK, jsonmodels.NewGetOutputConsumersResponse(outputID, cachedConsumers.Unwrap()))
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region GetOutputMetadata ////////////////////////////////////////////////////////////////////////////////////////////

// GetOutputMetadata is the handler for the /ledgerstate/outputs/:outputID/metadata endpoint.
func GetOutputMetadata(c echo.Context) (err error) {
	var outputID utxo.OutputID
	if err = outputID.FromBase58(c.Param("outputID")); err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels.NewErrorResponse(err))
	}

	if !deps.Tangle.Ledger.Storage.CachedOutputMetadata(outputID).Consume(func(outputMetadata *ledger.OutputMetadata) {
		confirmedConsumerID := deps.Tangle.Utils.ConfirmedConsumer(outputID)

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
	var transactionID utxo.TransactionID
	if err = transactionID.FromBase58(c.Param("transactionID")); err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels.NewErrorResponse(err))
	}

	var tx *devnetvm.Transaction
	// retrieve transaction
	if !deps.Tangle.Ledger.Storage.CachedTransaction(transactionID).Consume(func(transaction *ledger.Transaction) {
		tx = transaction.Transaction.(*devnetvm.Transaction)
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
	var transactionID utxo.TransactionID
	if err = transactionID.FromBase58(c.Param("transactionID")); err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels.NewErrorResponse(err))
	}

	if !deps.Tangle.Ledger.Storage.CachedTransactionMetadata(transactionID).Consume(func(transactionMetadata *ledger.TransactionMetadata) {
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
	var transactionID utxo.TransactionID
	if err = transactionID.FromBase58(c.Param("transactionID")); err != nil {
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
func branchIDFromContext(c echo.Context) (branchID branchdag.BranchID, err error) {
	switch branchIDString := c.Param("branchID"); branchIDString {
	case "MasterBranchID":
		branchID = branchdag.MasterBranchID
	default:
		err = branchID.FromBase58(branchIDString)
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
	tx, err := new(devnetvm.Transaction).FromBytes(request.TransactionBytes)
	if err != nil {
		return c.JSON(http.StatusBadRequest, &jsonmodels.PostTransactionResponse{Error: err.Error()})
	}

	// check if it would introduce a double spend known to the node locally
	has, conflictingID := doubleSpendFilter.HasConflict(tx.Essence().Inputs())
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
	if transactionErr := deps.Tangle.Ledger.CheckTransaction(context.Background(), tx); transactionErr != nil {
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
	doubleSpendFilter.Add(tx)
	if _, err := messagelayer.AwaitMessageToBeBooked(issueTransaction, tx.ID(), maxBookedAwaitTime); err != nil {
		// if we failed to issue the transaction, we remove it
		doubleSpendFilter.Remove(tx.ID())
		return c.JSON(http.StatusBadRequest, jsonmodels.PostTransactionResponse{Error: err.Error()})
	}
	return c.JSON(http.StatusOK, &jsonmodels.PostTransactionResponse{TransactionID: tx.ID().Base58()})
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
