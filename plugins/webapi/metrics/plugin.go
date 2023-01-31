package metrics

import (
	"errors"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/iotaledger/hive.go/core/autopeering/discover"
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/node"
	"github.com/labstack/echo"
	"go.uber.org/dig"

	"github.com/iotaledger/goshimmer/packages/app/collector"
	"github.com/iotaledger/goshimmer/packages/app/jsonmodels"
	"github.com/iotaledger/goshimmer/packages/protocol"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/throughputquota/mana1/manamodels"
	"github.com/iotaledger/goshimmer/plugins/dashboardmetrics"
)

var (
	// Plugin holds the singleton instance of the plugin.
	Plugin *node.Plugin

	deps   = new(dependencies)
	maxBPS float64

	bps atomic.Uint64
)

type dependencies struct {
	dig.In

	Server    *echo.Echo
	Protocol  *protocol.Protocol
	Discovery *discover.Protocol
	Collector *collector.Collector
}

func init() {
	Plugin = node.NewPlugin("WebAPIMetricsEndpoint", deps, node.Enabled, configure)
}

func configure(_ *node.Plugin) {
	dashboardmetrics.Events.AttachedBPSUpdated.Attach(event.NewClosure(func(e *dashboardmetrics.AttachedBPSUpdatedEvent) {
		bps.Store(e.BPS)
	}))

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
	cmanas := dashboardmetrics.ConsensusManaMap().ToIssuerStrList()

	if maxBPS == 0 {
		maxBPS = 1 / deps.Protocol.CongestionControl.Scheduler().Rate().Seconds()
	}

	return c.JSON(http.StatusOK, jsonmodels.NodesMetricsResponse{
		Cmanas:          cmanas,
		ActiveManaRatio: activeManaRatio(),
		OnlineNodes:     len(deps.Discovery.GetVerifiedPeers()),
		MaxBPS:          maxBPS,
		BlockScheduled:  dashboardmetrics.BlockCountSinceStartPerComponentGrafana()[collector.Scheduled],
	})
}

func activeManaRatio() float64 {
	var totalActive int64
	for _, am := range dashboardmetrics.ConsensusManaMap() {
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
	return float64(totalActive) / float64(totalConsensusMana)
}
