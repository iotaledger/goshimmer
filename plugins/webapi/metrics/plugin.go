package metrics

import (
	"errors"
	"net/http"
	"time"

	"github.com/iotaledger/hive.go/core/autopeering/discover"
	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/node"
	"github.com/labstack/echo"
	"go.uber.org/dig"

	"github.com/iotaledger/goshimmer/packages/app/jsonmodels"
	"github.com/iotaledger/goshimmer/packages/app/retainer"
	"github.com/iotaledger/goshimmer/packages/protocol"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/throughputquota/mana1/manamodels"
	"github.com/iotaledger/goshimmer/plugins/metrics"
)

var (
	// Plugin holds the singleton instance of the plugin.
	Plugin *node.Plugin

	deps   = new(dependencies)
	maxBPS float64
)

type dependencies struct {
	dig.In

	Server    *echo.Echo
	Retainer  *retainer.Retainer
	Protocol  *protocol.Protocol
	Discovery *discover.Protocol
}

func init() {
	Plugin = node.NewPlugin("WebAPIMetricsEndpoint", deps, node.Enabled, configure)
}

func configure(_ *node.Plugin) {
	deps.Server.GET("metrics/global", GetGlobalMetrics)
	deps.Server.GET("metrics/nodes", GetNodesMetrics)
}

// GetGlobalMetrics is the handler for the /metrics/global endpoint.
func GetGlobalMetrics(c echo.Context) (err error) {
	blkStored := metrics.BlockCountSinceStartPerComponentGrafana()[metrics.Attached]

	var finalizedBlk uint64
	for _, num := range metrics.FinalizedBlockCountPerType() {
		finalizedBlk += num
	}
	inclusionRate := finalizedBlk / blkStored

	var totalDelay uint64
	for _, t := range metrics.BlockFinalizationTotalTimeSinceIssuedPerType() {
		totalDelay += t
	}
	confirmationDelay := time.Duration(totalDelay / finalizedBlk)

	return c.JSON(http.StatusOK, jsonmodels.GlobalMetricsResponse{
		BlockStored:        blkStored,
		BookedTransactions: metrics.BookedTransactions(),
		InclusionRate:      float64(inclusionRate),
		ConfirmationDelay:  confirmationDelay.String(),
		ActiveManaRatio:    activeManaRatio(),
		OnlineNodes:        len(deps.Discovery.GetVerifiedPeers()),
		ConflictsResolved:  metrics.FinalizedConflictCountDB(),
		TotalConflicts:     metrics.TotalConflictCountDB(),
	})
}

// GetNodesMetrics is the handler for the /metrics/nodes endpoint.
func GetNodesMetrics(c echo.Context) (err error) {
	cmanas := metrics.ConsensusManaMap().ToIssuerStrList()

	if maxBPS == 0 {
		maxBPS = 1 / deps.Protocol.CongestionControl.Scheduler().Rate().Seconds()
	}

	return c.JSON(http.StatusOK, jsonmodels.NodesMetricsResponse{
		Cmanas:          cmanas,
		ActiveManaRatio: activeManaRatio(),
		OnlineNodes:     len(deps.Discovery.GetVerifiedPeers()),
		MaxBPS:          maxBPS,
		BlockScheduled:  metrics.BlockCountSinceStartPerComponentGrafana()[metrics.Scheduled],
	})
}

func activeManaRatio() float64 {
	var totalActive int64
	for _, am := range metrics.ConsensusManaMap() {
		totalActive += am
	}

	consensusManaList, _, err := manamodels.GetHighestManaIssuers(0, lo.PanicOnErr(deps.Protocol.Engine().SybilProtection.Weights().Map()))
	if err != nil && !errors.Is(err, manamodels.ErrQueryNotAllowed) {
		return 0
	}

	var totalConsensusMana int64
	for i := 0; i < len(consensusManaList); i++ {
		totalConsensusMana += consensusManaList[i].Mana
	}
	return float64(totalActive / totalConsensusMana)
}
