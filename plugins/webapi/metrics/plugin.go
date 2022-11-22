package metrics

import (
	"net/http"
	"time"

	"github.com/iotaledger/hive.go/core/autopeering/discover"
	"github.com/iotaledger/hive.go/core/node"
	"github.com/labstack/echo"
	"go.uber.org/dig"

	"github.com/iotaledger/goshimmer/packages/app/jsonmodels"
	"github.com/iotaledger/goshimmer/packages/app/retainer"
	"github.com/iotaledger/goshimmer/packages/protocol"
	"github.com/iotaledger/goshimmer/plugins/metrics"
)

var (
	// Plugin holds the singleton instance of the plugin.
	Plugin *node.Plugin

	deps = new(dependencies)
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
	blkStored := metrics.BlockCountSinceStartPerComponentGrafana()[metrics.Store]

	var finalizedBlk uint64
	for _, num := range metrics.FinalizedBlockCountPerType() {
		finalizedBlk += num
	}
	inclusionRate := finalizedBlk / blkStored

	var totalDelay, typeLen uint64
	for _, t := range metrics.BlockFinalizationTotalTimeSinceIssuedPerType() {
		totalDelay += t
		typeLen++
	}
	confirmationDelay := time.Duration(totalDelay / typeLen)

	return c.JSON(http.StatusOK, jsonmodels.GlobalMetricsResponse{
		BlockStored:       blkStored,
		InclusionRate:     float64(inclusionRate),
		ConfirmationDelay: confirmationDelay.String(),
		ActiveManaRatio:   activeManaRatio(),
		OnlineNodes:       len(deps.Discovery.GetVerifiedPeers()),
		ConflictsResolved: metrics.FinalizedConflictCountDB(),
		TotalConflicts:    metrics.TotalConflictCountDB(),
	})
}

func GetNodesMetrics(c echo.Context) (err error) {
	cmanas := metrics.ConsensusManaMap().ToIssuerStrList()

	return c.JSON(http.StatusOK, jsonmodels.NodesMetricsResponse{
		Cmanas:          cmanas,
		ActiveManaRatio: activeManaRatio(),
		OnlineNodes:     len(deps.Discovery.GetVerifiedPeers()),
	})
}

func activeManaRatio() float64 {
	var totalActive int64
	for _, am := range metrics.ConsensusManaMap() {
		totalActive += am
	}
	return float64(totalActive / deps.Protocol.Engine().ManaTracker.TotalMana())
}
