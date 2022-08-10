package epoch

import (
	"net/http"
	"sort"
	"strconv"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/core/node"
	"github.com/labstack/echo"
	"go.uber.org/dig"

	"github.com/iotaledger/goshimmer/packages/app/jsonmodels"
	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/notarization"
	"github.com/iotaledger/goshimmer/plugins/epochstorage"
)

// PluginName is the name of the web API epoch endpoint plugin.
const PluginName = "WebAPIEpochEndpoint"

var (
	// Plugin is the plugin instance of the web API epoch endpoint plugin.
	Plugin *node.Plugin
	deps   = new(dependencies)
)

type dependencies struct {
	dig.In

	Server          *echo.Echo
	EpochStorage    *node.Plugin `name:"epochstorage"`
	NotarizationMgr *notarization.Manager
}

func init() {
	Plugin = node.NewPlugin(PluginName, deps, node.Enabled, configure)
}

func configure(_ *node.Plugin) {
	deps.Server.GET("epochs", getAllCommittedEpochs)
	deps.Server.GET("ec", getCurrentEC)
	deps.Server.GET("epoch/:ei", getCommittedEpoch)
	deps.Server.GET("epoch/:ei/utxos", getUTXOs)
	deps.Server.GET("epoch/:ei/blocks", getBlocks)
	deps.Server.GET("epoch/:ei/transactions", getTransactions)
	deps.Server.GET("epoch/:ei/pending-conflict-count", getPendingConflictsCount)
	deps.Server.GET("epoch/:ei/voters-weight", getVotersWeight)
}

func getAllCommittedEpochs(c echo.Context) error {
	allEpochs := epochstorage.GetCommittableEpochs()
	allEpochsInfos := make([]*jsonmodels.EpochInfo, 0, len(allEpochs))
	for _, ecRecord := range allEpochs {
		allEpochsInfos = append(allEpochsInfos, jsonmodels.EpochInfoFromRecord(ecRecord))
	}
	sort.Slice(allEpochsInfos, func(i, j int) bool {
		return allEpochsInfos[i].EI < allEpochsInfos[j].EI
	})
	return c.JSON(http.StatusOK, allEpochsInfos)
}

func getCurrentEC(c echo.Context) error {
	ecRecord, err := deps.NotarizationMgr.GetLatestEC()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, jsonmodels.NewErrorResponse(err))
	}
	ec := ecRecord.ComputeEC()

	return c.JSON(http.StatusOK, ec.Base58())
}

func getCommittedEpoch(c echo.Context) error {
	ei, err := getEI(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels.NewErrorResponse(err))
	}
	allEpochs := epochstorage.GetCommittableEpochs()
	epochInfo := jsonmodels.EpochInfoFromRecord(allEpochs[ei])

	return c.JSON(http.StatusOK, epochInfo)
}

func getUTXOs(c echo.Context) error {
	ei, err := getEI(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels.NewErrorResponse(err))
	}
	spentIDs, createdIDs := epochstorage.GetEpochUTXOs(ei)

	spent := make([]string, len(spentIDs))
	for i, o := range spentIDs {
		spent[i] = o.String()
	}
	created := make([]string, len(createdIDs))
	for i, o := range createdIDs {
		created[i] = o.String()
	}

	resp := jsonmodels.EpochUTXOsResponse{SpentOutputs: spent, CreatedOutputs: created}

	return c.JSON(http.StatusOK, resp)
}

func getBlocks(c echo.Context) error {
	ei, err := getEI(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels.NewErrorResponse(err))
	}
	blockIDs := epochstorage.GetEpochBlockIDs(ei)

	blocks := make([]string, len(blockIDs))
	for i, m := range blockIDs {
		blocks[i] = m.String()
	}
	resp := jsonmodels.EpochBlocksResponse{Blocks: blocks}

	return c.JSON(http.StatusOK, resp)
}

func getTransactions(c echo.Context) error {
	ei, err := getEI(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels.NewErrorResponse(err))
	}
	transactionIDs := epochstorage.GetEpochTransactions(ei)

	transactions := make([]string, len(transactionIDs))
	for i, t := range transactionIDs {
		transactions[i] = t.String()
	}
	resp := jsonmodels.EpochTransactionsResponse{Transactions: transactions}

	return c.JSON(http.StatusOK, resp)
}

func getPendingConflictsCount(c echo.Context) error {
	ei, err := getEI(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels.NewErrorResponse(err))
	}
	allEpochs := epochstorage.GetPendingConflictCount()
	resp := jsonmodels.EpochPendingConflictCountResponse{PendingConflictCount: allEpochs[ei]}

	return c.JSON(http.StatusOK, resp)
}

func getEI(c echo.Context) (epoch.Index, error) {
	eiText := c.Param("ei")
	eiNumber, err := strconv.Atoi(eiText)
	if err != nil {
		return 0, errors.Wrap(err, "can't parse EI from URL param")
	}
	return epoch.Index(uint64(eiNumber)), nil
}

func getVotersWeight(c echo.Context) error {
	ei, err := getEI(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels.NewErrorResponse(err))
	}
	weights := epochstorage.GetEpochVotersWeight(ei)

	respMap := make(map[string]*jsonmodels.NodeWeight)
	for ecr, nw := range weights {
		ws := make(map[string]float64, 0)
		for id, w := range nw {
			ws[id.String()] = w
		}
		nodeWeights := &jsonmodels.NodeWeight{Weights: ws}
		respMap[ecr.Base58()] = nodeWeights
	}
	resp := jsonmodels.EpochVotersWeightResponse{VotersWeight: respMap}

	return c.JSON(http.StatusOK, resp)
}
