package slot

import (
	"net/http"
	"strconv"
	"sync"

	"github.com/labstack/echo/v4"
	"github.com/pkg/errors"
	"go.uber.org/dig"

	"github.com/iotaledger/goshimmer/packages/app/jsonmodels"
	"github.com/iotaledger/goshimmer/packages/app/retainer"
	"github.com/iotaledger/goshimmer/packages/core/commitment"
	"github.com/iotaledger/goshimmer/packages/node"
	"github.com/iotaledger/goshimmer/packages/protocol"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/notarization"
	"github.com/iotaledger/hive.go/core/slot"
)

// PluginName is the name of the web API slot endpoint plugin.
const PluginName = "WebAPISlotEndpoint"

var (
	// Plugin is the plugin instance of the web API slot endpoint plugin.
	Plugin *node.Plugin
	deps   = new(dependencies)

	currentSlotCommitment     *commitment.Commitment
	currentSlotCommitmentLock sync.RWMutex
)

type dependencies struct {
	dig.In

	Server   *echo.Echo
	Retainer *retainer.Retainer
	Protocol *protocol.Protocol
}

func init() {
	Plugin = node.NewPlugin(PluginName, deps, node.Enabled, configure)
}

func configure(_ *node.Plugin) {
	deps.Server.GET("sc", GetCurrentSC)
	deps.Server.GET("slots/:index", GetCommittedSlot)
	deps.Server.GET("slots/commitment/:commitment", GetCommittedSlotByCommitment)
	deps.Server.GET("slots/:index/utxos", GetUTXOs)
	deps.Server.GET("slots/:index/blocks", GetBlocks)
	deps.Server.GET("slots/:index/transactions", GetTransactions)
	// deps.Server.GET("slots/:index/voters-weight", getVotersWeight)

	deps.Protocol.Events.Engine.Notarization.SlotCommitted.Hook(func(e *notarization.SlotCommittedDetails) {
		currentSlotCommitmentLock.Lock()
		defer currentSlotCommitmentLock.Unlock()

		currentSlotCommitment = e.Commitment
	})
}

func GetCurrentSC(c echo.Context) error {
	currentSlotCommitmentLock.RLock()
	defer currentSlotCommitmentLock.RUnlock()

	return c.JSON(http.StatusOK, jsonmodels.SlotInfoFromRecord(currentSlotCommitment))
}

func GetCommittedSlot(c echo.Context) error {
	index, err := getIndex(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels.NewErrorResponse(err))
	}

	cc, exist := deps.Retainer.Commitment(index)
	if !exist {
		return c.JSON(http.StatusBadRequest, jsonmodels.NewErrorResponse(errors.New("commitment not exists")))
	}

	return c.JSON(http.StatusOK, jsonmodels.SlotInfoFromRecord(cc.M.Commitment))
}

func GetCommittedSlotByCommitment(c echo.Context) error {
	var ID commitment.ID
	err := ID.FromBase58(c.Param("commitment"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels.NewErrorResponse(err))
	}

	cc, exist := deps.Retainer.CommitmentByID(ID)
	if !exist {
		return c.JSON(http.StatusBadRequest, jsonmodels.NewErrorResponse(errors.New("commitment not exists")))
	}

	return c.JSON(http.StatusOK, jsonmodels.SlotInfoFromRecord(cc.M.Commitment))
}

func GetUTXOs(c echo.Context) error {
	index, err := getIndex(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels.NewErrorResponse(err))
	}

	cc, exist := deps.Retainer.Commitment(index)
	if !exist {
		return c.JSON(http.StatusBadRequest, jsonmodels.NewErrorResponse(errors.New("commitment not exists")))
	}

	var (
		spent   = make([]string, 0)
		created = make([]string, 0)
	)
	for _, s := range cc.M.SpentOutputs.Slice() {
		spent = append(spent, s.Base58())
	}
	for _, c := range cc.M.CreatedOutputs.Slice() {
		created = append(created, c.Base58())
	}

	return c.JSON(http.StatusOK, jsonmodels.SlotUTXOsResponse{SpentOutputs: spent, CreatedOutputs: created})
}

func GetBlocks(c echo.Context) error {
	index, err := getIndex(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels.SlotBlocksResponse{Error: err.Error()})
	}

	cc, exist := deps.Retainer.Commitment(index)
	if !exist {
		return c.JSON(http.StatusBadRequest, jsonmodels.NewErrorResponse(errors.New("commitment not exists")))
	}

	return c.JSON(http.StatusOK, jsonmodels.SlotBlocksResponse{Blocks: cc.M.AcceptedBlocks.Base58()})
}

func GetTransactions(c echo.Context) error {
	index, err := getIndex(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels.SlotBlocksResponse{Error: err.Error()})
	}

	cc, exist := deps.Retainer.Commitment(index)
	if !exist {
		return c.JSON(http.StatusBadRequest, jsonmodels.NewErrorResponse(errors.New("commitment not exists")))
	}

	txs := make([]string, 0)
	for _, t := range cc.M.AcceptedTransactions.Slice() {
		txs = append(txs, t.Base58())
	}

	return c.JSON(http.StatusOK, jsonmodels.SlotTransactionsResponse{Transactions: txs})
}

func getIndex(c echo.Context) (slot.Index, error) {
	indexText := c.Param("index")
	indexNumber, err := strconv.Atoi(indexText)
	if err != nil {
		return 0, errors.Wrap(err, "can't parse Index from URL param")
	}
	return slot.Index(uint64(indexNumber)), nil
}

// func getVotersWeight(c echo.Context) error {
// 	ei, err := getIndex(c)
// 	if err != nil {
// 		return c.JSON(http.StatusBadRequest, jsonmodels.SlotBlocksResponse{Error: err.Error()})
// 	}

// 	cc, exist := deps.Retainer.Commitment(ei)
// 	if !exist {
// 		return c.JSON(http.StatusBadRequest, jsonmodels.NewErrorResponse(errors.New("commitment not exists")))
// 	}

// 	return c.JSON(http.StatusOK, jsonmodels.SlotVotersWeightResponse{})
// }
