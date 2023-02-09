package epoch

import (
	"strconv"
	"sync"

	"github.com/cockroachdb/errors"

	"github.com/iotaledger/goshimmer/packages/app/jsonmodels"
	"github.com/iotaledger/goshimmer/packages/app/retainer"
	"github.com/iotaledger/goshimmer/packages/core/commitment"
	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/protocol"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/notarization"
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/node"

	"net/http"

	"github.com/labstack/echo"
	"go.uber.org/dig"
)

// PluginName is the name of the web API epoch endpoint plugin.
const PluginName = "WebAPIEpochEndpoint"

var (
	// Plugin is the plugin instance of the web API epoch endpoint plugin.
	Plugin *node.Plugin
	deps   = new(dependencies)

	currentEC     *commitment.Commitment
	currentECLock sync.RWMutex
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
	deps.Server.GET("ec", getCurrentEC)
	deps.Server.GET("epochs/:ei", getCommittedEpoch)
	deps.Server.GET("epochs/commitment/:commitment", getCommittedEpochByCommitment)
	deps.Server.GET("epochs/:ei/utxos", getUTXOs)
	deps.Server.GET("epochs/:ei/blocks", getBlocks)
	deps.Server.GET("epochs/:ei/transactions", getTransactions)
	// deps.Server.GET("epochs/:ei/voters-weight", getVotersWeight)

	deps.Protocol.Engine().NotarizationManager.Events.EpochCommitted.Attach(event.NewClosure(func(e *notarization.EpochCommittedDetails) {
		currentECLock.Lock()
		defer currentECLock.Unlock()

		currentEC = e.Commitment
	}))
}

func getCurrentEC(c echo.Context) error {
	currentECLock.RLock()
	defer currentECLock.RUnlock()

	return c.JSON(http.StatusOK, jsonmodels.EpochInfoFromRecord(currentEC))
}

func getCommittedEpoch(c echo.Context) error {
	ei, err := getEI(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels.NewErrorResponse(err))
	}

	cc, exist := deps.Retainer.Commitment(ei)
	if !exist {
		return c.JSON(http.StatusBadRequest, jsonmodels.NewErrorResponse(errors.New("commitment not exists")))
	}

	return c.JSON(http.StatusOK, jsonmodels.EpochInfoFromRecord(cc.M.Commitment))
}

func getCommittedEpochByCommitment(c echo.Context) error {
	var ID commitment.ID
	err := ID.FromBase58(c.Param("commitment"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels.NewErrorResponse(err))
	}

	cc, exist := deps.Retainer.CommitmentbyID(ID)
	if !exist {
		return c.JSON(http.StatusBadRequest, jsonmodels.NewErrorResponse(errors.New("commitment not exists")))
	}

	return c.JSON(http.StatusOK, jsonmodels.EpochInfoFromRecord(cc.M.Commitment))
}

func getUTXOs(c echo.Context) error {
	ei, err := getEI(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels.NewErrorResponse(err))
	}

	cc, exist := deps.Retainer.Commitment(ei)
	if !exist {
		return c.JSON(http.StatusBadRequest, jsonmodels.NewErrorResponse(errors.New("commitment not exists")))
	}

	var (
		spent   []string
		created []string
	)
	for _, s := range cc.M.SpentOutputs.Slice() {
		spent = append(spent, s.Base58())
	}
	for _, c := range cc.M.CreatedOutputs.Slice() {
		created = append(created, c.Base58())
	}

	return c.JSON(http.StatusOK, jsonmodels.EpochUTXOsResponse{SpentOutputs: spent, CreatedOutputs: created})
}

func getBlocks(c echo.Context) error {
	ei, err := getEI(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels.EpochBlocksResponse{Error: err.Error()})
	}

	cc, exist := deps.Retainer.Commitment(ei)
	if !exist {
		return c.JSON(http.StatusBadRequest, jsonmodels.NewErrorResponse(errors.New("commitment not exists")))
	}

	return c.JSON(http.StatusOK, jsonmodels.EpochBlocksResponse{Blocks: cc.M.AcceptedBlocks.Base58()})
}

func getTransactions(c echo.Context) error {
	ei, err := getEI(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels.EpochBlocksResponse{Error: err.Error()})
	}

	cc, exist := deps.Retainer.Commitment(ei)
	if !exist {
		return c.JSON(http.StatusBadRequest, jsonmodels.NewErrorResponse(errors.New("commitment not exists")))
	}

	var txs []string
	for _, t := range cc.M.AcceptedTransactions.Slice() {
		txs = append(txs, t.Base58())
	}

	return c.JSON(http.StatusOK, jsonmodels.EpochTransactionsResponse{Transactions: txs})
}

func getEI(c echo.Context) (epoch.Index, error) {
	eiText := c.Param("ei")
	eiNumber, err := strconv.Atoi(eiText)
	if err != nil {
		return 0, errors.Wrap(err, "can't parse Index from URL param")
	}
	return epoch.Index(uint64(eiNumber)), nil
}

// func getVotersWeight(c echo.Context) error {
// 	ei, err := getEI(c)
// 	if err != nil {
// 		return c.JSON(http.StatusBadRequest, jsonmodels.EpochBlocksResponse{Error: err.Error()})
// 	}

// 	cc, exist := deps.Retainer.Commitment(ei)
// 	if !exist {
// 		return c.JSON(http.StatusBadRequest, jsonmodels.NewErrorResponse(errors.New("commitment not exists")))
// 	}

// 	return c.JSON(http.StatusOK, jsonmodels.EpochVotersWeightResponse{})
// }
