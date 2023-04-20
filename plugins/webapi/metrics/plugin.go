package metrics

import (
	"net/http"
	"sync/atomic"
	"time"

	"github.com/labstack/echo/v4"
	"go.uber.org/dig"

	"github.com/iotaledger/goshimmer/packages/app/collector"
	"github.com/iotaledger/goshimmer/packages/app/jsonmodels"
	"github.com/iotaledger/goshimmer/packages/node"
	"github.com/iotaledger/goshimmer/packages/protocol"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/throughputquota/mana1/manamodels"
	"github.com/iotaledger/goshimmer/plugins/dashboardmetrics"
	"github.com/iotaledger/hive.go/autopeering/discover"
	"github.com/iotaledger/hive.go/lo"
)

type dependencies struct {
	dig.In

	Server    *echo.Echo
	Protocol  *protocol.Protocol
	Discovery *discover.Protocol
}

var (
	// Plugin holds the singleton instance of the plugin.
	Plugin *node.Plugin

	deps   = new(dependencies)
	maxBPS float64

	bps atomic.Uint64
)

func init() {
	Plugin = node.NewPlugin("WebAPIMetricsEndpoint", deps, node.Enabled, configure)
}

func configure(_ *node.Plugin) {
	dashboardmetrics.Events.AttachedBPSUpdated.Hook(func(e *dashboardmetrics.AttachedBPSUpdatedEvent) {
		bps.Store(e.BPS)
	})

	deps.Server.GET("metrics/global", GetGlobalMetrics)
	deps.Server.GET("metrics/nodes", GetNodesMetrics)
}

// GetGlobalMetrics is the handler for the /metrics/global endpoint.
func GetGlobalMetrics(c echo.Context) (err error) {
	blkStored := dashboardmetrics.BlockCountSinceStartPerComponentGrafana()[collector.Attached]
	var finalizedBlk uint64
	for _, num := range dashboardmetrics.FinalizedBlockCountPerType() {
		finalizedBlk += num
	}
	inclusionRate := float64(finalizedBlk) / float64(blkStored)

	var totalDelay uint64
	for _, t := range dashboardmetrics.BlockFinalizationTotalTimeSinceIssuedPerType() {
		totalDelay += t
	}
	confirmationDelay := time.Duration(totalDelay / finalizedBlk)

	return c.JSON(http.StatusOK, jsonmodels.GlobalMetricsResponse{
		BPS:                bps.Load(),
		BookedTransactions: dashboardmetrics.BookedTransactions(),
		InclusionRate:      inclusionRate,
		ConfirmationDelay:  confirmationDelay.String(),
		ActiveManaRatio:    activeManaRatio(),
		OnlineNodes:        len(deps.Discovery.GetVerifiedPeers()),
		ConflictsResolved:  dashboardmetrics.FinalizedConflictCountDB(),
		TotalConflicts:     dashboardmetrics.TotalConflictCountDB(),
	})
}

// GetNodesMetrics is the handler for the /metrics/nodes endpoint.
func GetNodesMetrics(c echo.Context) (err error) {
	cManaMap := lo.PanicOnErr(deps.Protocol.Engine().SybilProtection.Weights().Map())
	cManaIssuerMap := manamodels.IssuerMap{}
	for k, v := range cManaMap {
		cManaIssuerMap[k] = v
	}

	if maxBPS == 0 {
		maxBPS = 1 / deps.Protocol.CongestionControl.Scheduler().Rate().Seconds()
	}

	return c.JSON(http.StatusOK, jsonmodels.NodesMetricsResponse{
		Cmanas:          cManaIssuerMap.ToIssuerStrList(),
		ActiveManaRatio: activeManaRatio(),
		OnlineNodes:     len(deps.Discovery.GetVerifiedPeers()),
		MaxBPS:          maxBPS,
		BlockScheduled:  dashboardmetrics.BlockCountSinceStartPerComponentGrafana()[collector.Scheduled],
	})
}

func activeManaRatio() float64 {
	totalActive := deps.Protocol.Engine().SybilProtection.Validators().TotalWeight()
	totalCMana := deps.Protocol.Engine().SybilProtection.Weights().TotalWeight()

	return float64(totalActive) / float64(totalCMana)
}
