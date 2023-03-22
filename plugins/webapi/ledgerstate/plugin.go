package ledgerstate

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/pkg/errors"
	"go.uber.org/dig"

	"github.com/iotaledger/goshimmer/packages/app/blockissuer"
	"github.com/iotaledger/goshimmer/packages/app/jsonmodels"
	"github.com/iotaledger/goshimmer/packages/core/shutdown"
	"github.com/iotaledger/goshimmer/packages/node"
	"github.com/iotaledger/goshimmer/packages/protocol"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool/conflictdag"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/vm/devnetvm"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/vm/devnetvm/indexer"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
	"github.com/iotaledger/goshimmer/plugins/webapi"
	"github.com/iotaledger/hive.go/app/daemon"
	"github.com/iotaledger/hive.go/lo"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/runtime/event"
)

// region Plugin ///////////////////////////////////////////////////////////////////////////////////////////////////////

// PluginName is the name of the web API plugin.
const (
	PluginName                       = "WebAPILedgerstateEndpoint"
	DoubleSpendFilterCleanupInterval = 10 * time.Second
)

type dependencies struct {
	dig.In

	Server      *echo.Echo
	Protocol    *protocol.Protocol
	BlockIssuer *blockissuer.BlockIssuer
	Indexer     *indexer.Indexer
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

	// Hook to the transaction confirmation event.
	onTransactionAccepted *event.Hook[func(*mempool.TransactionEvent)]

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
func FilterHasConflict(outputs devnetvm.Inputs) (bool, utxo.TransactionID) {
	if filterEnabled {
		has, conflictingID := doubleSpendFilter.HasConflict(outputs)
		return has, conflictingID
	}
	return false, utxo.TransactionID{}
}

// FilterAdd Adds transaction to the doubleSpendFilter if it is enabled.
func FilterAdd(tx *devnetvm.Transaction) {
	if filterEnabled {
		doubleSpendFilter.Add(tx)
	}
}

// FilterRemove Removes transaction id from the doubleSpendFilter if it is enabled.
func FilterRemove(txID utxo.TransactionID) {
	if filterEnabled {
		doubleSpendFilter.Remove(txID)
	}
}

func configure(plugin *node.Plugin) {
	if webapi.Parameters.EnableDSFilter {
		doubleSpendFilter = Filter()
		deps.Protocol.Events.Engine.Ledger.MemPool.TransactionAccepted.Hook(func(event *mempool.TransactionEvent) {
			doubleSpendFilter.Remove(event.Metadata.ID())
		}, event.WithWorkerPool(plugin.WorkerPool))
	}

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
	deps.Server.POST("ledgerstate/addresses/unspentOutputs", PostAddressUnspentOutputs)
	deps.Server.GET("ledgerstate/conflicts/:conflictID", GetConflict)
	deps.Server.GET("ledgerstate/conflicts/:conflictID/children", GetConflictChildren)
	deps.Server.GET("ledgerstate/conflicts/:conflictID/conflicts", GetConflictConflicts)
	deps.Server.GET("ledgerstate/conflicts/:conflictID/voters", GetConflictVoters)
	deps.Server.GET("ledgerstate/conflicts/:conflictID/sequenceids", GetConflictSequenceIDs)
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
	if onTransactionAccepted != nil {
		onTransactionAccepted.Unhook()
		onTransactionAccepted = nil
	}
}

func outputsOnAddress(address devnetvm.Address) (outputs devnetvm.Outputs) {
	deps.Indexer.CachedAddressOutputMappings(address).Consume(func(mapping *indexer.AddressOutputMapping) {
		deps.Protocol.Engine().Ledger.MemPool().Storage().CachedOutput(mapping.OutputID()).Consume(func(output utxo.Output) {
			if typedOutput, ok := output.(devnetvm.Output); ok {
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
	spentOutputs, unspentOutputs := devnetvm.Outputs{}, devnetvm.Outputs{}
	for _, output := range outputs {
		deps.Protocol.Engine().Ledger.MemPool().Storage().CachedOutputMetadata(output.ID()).Consume(func(outputMetadata *mempool.OutputMetadata) {
			if outputMetadata.IsSpent() {
				spentOutputs = append(spentOutputs, output)
				return
			}
			unspentOutputs = append(unspentOutputs, output)
		})
	}

	return c.JSON(http.StatusOK, jsonmodels.NewGetAddressResponse(address, spentOutputs, unspentOutputs))
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
			deps.Protocol.Engine().Ledger.MemPool().Storage().CachedOutputMetadata(output.ID()).Consume(func(outputMetadata *mempool.OutputMetadata) {
				isUnspent = !outputMetadata.IsSpent()
			})
			return
		}) {
			deps.Protocol.Engine().Ledger.MemPool().Storage().CachedOutputMetadata(output.ID()).Consume(func(outputMetadata *mempool.OutputMetadata) {
				if !outputMetadata.IsSpent() {
					deps.Protocol.Engine().Ledger.MemPool().Storage().CachedOutput(output.ID()).Consume(func(ledgerOutput utxo.Output) {
						var timestamp time.Time
						deps.Protocol.Engine().Ledger.MemPool().Storage().CachedTransaction(ledgerOutput.ID().TransactionID).Consume(func(tx utxo.Transaction) {
							timestamp = tx.(*devnetvm.Transaction).Essence().Timestamp()
						})
						res.UnspentOutputs[i].Outputs = append(res.UnspentOutputs[i].Outputs, jsonmodels.WalletOutput{
							Output:            *jsonmodels.NewOutput(output),
							ConfirmationState: outputMetadata.ConfirmationState(),
							Metadata:          jsonmodels.WalletOutputMetadata{Timestamp: timestamp},
						})
					})
				}
			})
		}
	}

	return c.JSON(http.StatusOK, res)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region GetConflict ////////////////////////////////////////////////////////////////////////////////////////////////////

// GetConflict is the handler for the /ledgerstate/conflict/:conflictID endpoint.
func GetConflict(c echo.Context) (err error) {
	conflictID, err := conflictIDFromContext(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels.NewErrorResponse(err))
	}

	conflict, exists := deps.Protocol.Engine().Ledger.MemPool().ConflictDAG().Conflict(conflictID)
	if !exists {
		return c.JSON(http.StatusNotFound, jsonmodels.NewErrorResponse(errors.Errorf("failed to load Conflict with %s", conflictID)))
	}

	return c.JSON(http.StatusOK, jsonmodels.NewConflictWeight(conflict, conflict.ConfirmationState(), deps.Protocol.Engine().Tangle.Booker().VirtualVoting().ConflictVotersTotalWeight(conflictID)))
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region GetConflictChildren ////////////////////////////////////////////////////////////////////////////////////////////

// GetConflictChildren is the handler for the /ledgerstate/conflict/:conflictID/childConflicts endpoint.
func GetConflictChildren(c echo.Context) (err error) {
	conflictID, err := conflictIDFromContext(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels.NewErrorResponse(err))
	}

	conflict, exists := deps.Protocol.Engine().Ledger.MemPool().ConflictDAG().Conflict(conflictID)
	if !exists {
		return c.JSON(http.StatusNotFound, jsonmodels.NewErrorResponse(fmt.Errorf("failed to load Conflict with %s", conflictID)))
	}

	return c.JSON(http.StatusOK, jsonmodels.NewGetConflictChildrenResponse(conflictID, conflict.Children()))
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region GetConflictConflicts ///////////////////////////////////////////////////////////////////////////////////////////

// GetConflictConflicts is the handler for the /ledgerstate/conflict/:conflictID/conflicts endpoint.
func GetConflictConflicts(c echo.Context) (err error) {
	conflictID, err := conflictIDFromContext(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels.NewErrorResponse(err))
	}

	conflict, exists := deps.Protocol.Engine().Ledger.MemPool().ConflictDAG().Conflict(conflictID)
	if !exists {
		return c.JSON(http.StatusNotFound, jsonmodels.NewErrorResponse(errors.Errorf("failed to load Conflict with %s", conflictID)))
	}
	conflictIDsPerConflictSet := make(map[utxo.OutputID][]utxo.TransactionID)
	for it := conflict.ConflictSets().Iterator(); it.HasNext(); {
		conflictSet := it.Next()
		conflictIDsPerConflictSet[conflictSet.ID()] = lo.Map(conflictSet.Conflicts().Slice(), func(c *conflictdag.Conflict[utxo.TransactionID, utxo.OutputID]) utxo.TransactionID {
			return c.ID()
		})
	}

	return c.JSON(http.StatusOK, jsonmodels.NewGetConflictConflictsResponse(conflictID, conflictIDsPerConflictSet))
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region GetConflictVoters ///////////////////////////////////////////////////////////////////////////////////////////////

// GetConflictVoters is the handler for the /ledgerstate/conflicts/:conflictID/voters endpoint.
func GetConflictVoters(c echo.Context) (err error) {
	conflictID, err := conflictIDFromContext(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels.NewErrorResponse(err))
	}

	voters := deps.Protocol.Engine().Tangle.Booker().VirtualVoting().ConflictVoters(conflictID)
	defer voters.Detach()

	return c.JSON(http.StatusOK, jsonmodels.NewGetConflictVotersResponse(conflictID, voters))
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region GetConflictSequenceIDs /////////////////////////////////////////////////////////////////////////////////////////

// GetConflictSequenceIDs is the handler for the /ledgerstate/conflict/:conflictID endpoint.
func GetConflictSequenceIDs(c echo.Context) (err error) {
	// conflictID, err := conflictIDFromContext(c)
	// if err != nil {
	//	return c.JSON(http.StatusBadRequest, jsonmodels.NewErrorResponse(err))
	// }
	//
	// sequenceIDs := make([]string, 0)
	// deps.Tangle.booker.MarkersManager.SequenceAliasMapping(markers.NewSequenceAlias(conflictID.Bytes())).Consume(func(sequenceAliasMapping *markers.SequenceAliasMapping) {
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

	if !deps.Protocol.Engine().Ledger.MemPool().Storage().CachedOutput(outputID).Consume(func(output utxo.Output) {
		err = c.JSON(http.StatusOK, jsonmodels.NewOutput(output.(devnetvm.Output)))
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

	cachedConsumers := deps.Protocol.Engine().Ledger.MemPool().Storage().CachedConsumers(outputID)
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

	if !deps.Protocol.Engine().Ledger.MemPool().Storage().CachedOutputMetadata(outputID).Consume(func(outputMetadata *mempool.OutputMetadata) {
		confirmedConsumerID := deps.Protocol.Engine().Ledger.MemPool().Utils().ConfirmedConsumer(outputID)

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
	if !deps.Protocol.Engine().Ledger.MemPool().Storage().CachedTransaction(transactionID).Consume(func(transaction utxo.Transaction) {
		tx = transaction.(*devnetvm.Transaction)
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

	if !deps.Protocol.Engine().Ledger.MemPool().Storage().CachedTransactionMetadata(transactionID).Consume(func(transactionMetadata *mempool.TransactionMetadata) {
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

	blockIDs := models.NewBlockIDs()
	_ = deps.Protocol.Engine().Tangle.Booker().GetAllAttachments(transactionID).ForEach(func(attachment *booker.Block) error {
		blockIDs.Add(attachment.ID())
		return nil
	})

	return c.JSON(http.StatusOK, jsonmodels.NewGetTransactionAttachmentsResponse(transactionID, blockIDs))
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region conflictIDFromContext //////////////////////////////////////////////////////////////////////////////////////////

// conflictIDFromContext determines the ConflictID from the conflictID parameter in an echo.Context. It expects it to either
// be a base58 encoded string or one of the builtin aliases (MasterConflictID, LazyBookedConflictsConflictID or
// InvalidConflictID).
func conflictIDFromContext(c echo.Context) (conflictID utxo.TransactionID, err error) {
	switch conflictIDString := c.Param("conflictID"); conflictIDString {
	case "MasterConflictID":
		conflictID = utxo.EmptyTransactionID
	default:
		err = conflictID.FromBase58(conflictIDString)
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
	tx := new(devnetvm.Transaction)
	err := tx.FromBytes(request.TransactionBytes)
	if err != nil {
		return c.JSON(http.StatusBadRequest, &jsonmodels.PostTransactionResponse{Error: err.Error()})
	}

	// if filter is enabled check if it would introduce a double spend known to the node locally
	has, conflictingID := FilterHasConflict(tx.Essence().Inputs())
	if has {
		err = errors.Errorf("transaction is conflicting with previously submitted transaction %s", conflictingID.Base58())
		return c.JSON(http.StatusBadRequest, &jsonmodels.PostTransactionResponse{Error: err.Error()})
	}

	// check transaction validity
	if transactionErr := deps.Protocol.Engine().Ledger.MemPool().CheckTransaction(context.Background(), tx); transactionErr != nil {
		return c.JSON(http.StatusBadRequest, &jsonmodels.PostTransactionResponse{Error: transactionErr.Error()})
	}

	// TODO: check if transaction is too old
	// if tx.Essence().Timestamp().Before(time.Now().Add(-models.MaxReattachmentTimeMin)) {
	//	return c.JSON(http.StatusBadRequest, &jsonmodels.PostTransactionResponse{Error: fmt.Sprintf("transaction timestamp is older than MaxReattachmentTime (%s) and cannot be issued", tangleold.MaxReattachmentTimeMin)})
	// }

	// if transaction is in the future we wait until the time arrives
	if tx.Essence().Timestamp().After(time.Now()) {
		if time.Until(tx.Essence().Timestamp()) > time.Minute {
			return c.JSON(http.StatusBadRequest, &jsonmodels.PostTransactionResponse{Error: "transaction timestamp is in the future and cannot be issued; please readjust local clock"})
		}
		time.Sleep(time.Until(tx.Essence().Timestamp()) + 1*time.Nanosecond)
	}

	block, err := deps.BlockIssuer.CreateBlock(tx)
	if err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels.PostTransactionResponse{Error: err.Error()})
	}
	// add tx to double spend doubleSpendFilter
	FilterAdd(tx)
	if err = deps.BlockIssuer.IssueBlockAndAwaitBlockToBeBooked(block, maxBookedAwaitTime); err != nil {
		// if we failed to issue the transaction, we remove it
		FilterRemove(tx.ID())
		return c.JSON(http.StatusBadRequest, jsonmodels.PostTransactionResponse{Error: err.Error()})
	}

	return c.JSON(http.StatusOK, &jsonmodels.PostTransactionResponse{TransactionID: tx.ID().Base58(), BlockID: block.ID().Base58()})
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
