package epoch

import (
	"net/http"
	"sort"
	"strconv"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/node"
	"github.com/labstack/echo"
	"go.uber.org/dig"

	"github.com/iotaledger/goshimmer/packages/epoch"
	"github.com/iotaledger/goshimmer/packages/jsonmodels"
	"github.com/iotaledger/goshimmer/plugins/metrics"
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

	Server  *echo.Echo
	Metrics *metrics.EpochCommitmentsMetrics
}

func init() {
	Plugin = node.NewPlugin(PluginName, deps, node.Enabled, configure)
}

func configure(_ *node.Plugin) {
	deps.Server.GET("epochs", getAllCommittedEpochs)
	deps.Server.GET("epoch/:ei", getCommittedEpoch)
	deps.Server.GET("epoch/:ei/voters-weight", getVotersWeight)
	deps.Server.GET("epoch/:ei/utxos", getUTXOs)
	deps.Server.GET("epoch/:ei/messages", getMessages)
	deps.Server.GET("epoch/:ei/transactions", getTransactions)
	deps.Server.GET("epoch/:ei/pending-branches-count", getPendingBranchesCount)
}

func getAllCommittedEpochs(c echo.Context) error {
	allEpochs := deps.Metrics.GetCommittedEpochs()
	allEpochsInfos := make([]*jsonmodels.EpochInfo, 0, len(allEpochs))
	for _, ecr := range allEpochs {
		allEpochsInfos = append(allEpochsInfos, jsonmodels.EpochInfoFromRecord(ecr))
	}
	sort.Slice(allEpochsInfos, func(i, j int) bool {
		return allEpochsInfos[i].EI < allEpochsInfos[j].EI
	})
	return c.JSON(http.StatusOK, allEpochsInfos)
}

func getEI(c echo.Context) (epoch.Index, error) {
	eiText := c.Param("ei")
	eiNumber, err := strconv.Atoi(eiText)
	if err != nil {
		return 0, errors.Wrap(err, "can't parse EI from URL param")
	}
	return epoch.Index(uint64(eiNumber)), nil
}

func getCommittedEpoch(c echo.Context) error {
	ei, err := getEI(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels.NewErrorResponse(err))
	}
	allEpochs := deps.Metrics.GetCommittedEpochs()
	epochInfo := jsonmodels.EpochInfoFromRecord(allEpochs[ei])

	return c.JSON(http.StatusOK, epochInfo)
}

func getVotersWeight(c echo.Context) error {
	ei, err := getEI(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels.NewErrorResponse(err))
	}
	allEpochs := deps.Metrics.GetEpochVotersWeight()
	resp := jsonmodels.EpochVotersWeightResponse{VotersWeight: allEpochs[ei]}

	return c.JSON(http.StatusOK, resp)
}

func getUTXOs(c echo.Context) error {
	ei, err := getEI(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels.NewErrorResponse(err))
	}
	allEpochs := deps.Metrics.GetEpochUTXOs()
	resp := jsonmodels.EpochUTXOsResponse{UTXOs: allEpochs[ei]}

	return c.JSON(http.StatusOK, resp)
}

func getMessages(c echo.Context) error {
	ei, err := getEI(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels.NewErrorResponse(err))
	}
	allEpochs := deps.Metrics.GetEpochMessages()
	resp := jsonmodels.EpochMessagesResponse{Messages: allEpochs[ei]}

	return c.JSON(http.StatusOK, resp)
}

func getTransactions(c echo.Context) error {
	ei, err := getEI(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels.NewErrorResponse(err))
	}
	allEpochs := deps.Metrics.GetEpochTransactions()
	resp := jsonmodels.EpochTransactionsResponse{Transactions: allEpochs[ei]}

	return c.JSON(http.StatusOK, resp)
}

func getPendingBranchesCount(c echo.Context) error {
	ei, err := getEI(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels.NewErrorResponse(err))
	}
	allEpochs := metrics.GetPendingBranchCount()
	resp := jsonmodels.EpochPendingBranchCountResponse{PendingBranchCount: allEpochs[ei]}

	return c.JSON(http.StatusOK, resp)
}
