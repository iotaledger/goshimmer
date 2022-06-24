package info

import (
	"net/http"
	"sort"
	"time"

	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/node"
	"github.com/labstack/echo"
	"github.com/mr-tron/base58/base58"
	"go.uber.org/dig"

	"github.com/iotaledger/goshimmer/packages/jsonmodels"
	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/goshimmer/plugins/autopeering/discovery"
	"github.com/iotaledger/goshimmer/plugins/banner"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
	"github.com/iotaledger/goshimmer/plugins/metrics"
)

// PluginName is the name of the web API info endpoint plugin.
const PluginName = "WebAPIInfoEndpoint"

type dependencies struct {
	dig.In

	Server *echo.Echo
	Local  *peer.Local
	Tangle *tangle.Tangle
}

var (
	// Plugin is the plugin instance of the web API info endpoint plugin.
	Plugin *node.Plugin
	deps   = new(dependencies)
)

func init() {
	Plugin = node.NewPlugin(PluginName, deps, node.Enabled, configure)
}

func configure(_ *node.Plugin) {
	deps.Server.GET("info", getInfo)
}

// getInfo returns the info of the node
// e.g.,
// {
// 	"version":"v0.2.0",
//	"tangleTime":{
// 		"messageID":"24Uq4UFQ7p5oLyjuXX32jHhNreo5hY9eo8Awh36RhdTHCwFMtct3SE2rhe3ceYz6rjKDjBs3usoHS3ujFEabP5ri",
// 		"time":1595528075204868900,
// 		"synced":true
// }
// 	"identityID":"5bf4aa1d6c47e4ce",
// 	"publickey":"CjUsn86jpFHWnSCx3NhWfU4Lk16mDdy1Hr7ERSTv3xn9",
// 	"enabledplugins":[
// 		"Config",
// 		"AutoPeering",
// 		"Analysis",
// 		"WebAPIDataEndpoint",
// 		"MessageLayer",
// 		"CLI",
// 		"Database",
// 		"WebAPIAutoPeeringEndpoint",
// 		"Metrics",
// 		"PortCheck",
// 		"Dashboard",
// 		"WebAPI",
// 		"WebAPIInfoEndpoint",
// 		"WebAPIMessageEndpoint",
// 		"Banner",
// 		"Gossip",
// 		"GracefulShutdown",
// 		"Logger"
// 	],
// 	"disabledplugins":[
// 		"RemoteLog",
// 		"Spammer",
// 		"WebAPIAuth"
// 	]
// }
func getInfo(c echo.Context) error {
	var enabledPlugins []string
	var disabledPlugins []string
	for pluginName, plugin := range node.GetPlugins() {
		if node.IsSkipped(plugin) {
			disabledPlugins = append(disabledPlugins, pluginName)
		} else {
			enabledPlugins = append(enabledPlugins, pluginName)
		}
	}

	sort.Strings(enabledPlugins)
	sort.Strings(disabledPlugins)

	// get TangleTime
	tm := deps.Tangle.TimeManager
	lcm := tm.LastAcceptedMessage()
	tangleTime := jsonmodels.TangleTime{
		Synced:            deps.Tangle.TimeManager.Synced(),
		AcceptedMessageID: lcm.MessageID.Base58(),
		ATT:               tm.ATT().UnixNano(),
		RATT:              tm.RATT().UnixNano(),
		CTT:               tm.CTT().UnixNano(),
		RCTT:              tm.RCTT().UnixNano(),
	}

	t := time.Now()
	accessMana, tAccess, _ := messagelayer.GetAccessMana(deps.Local.ID(), t)
	consensusMana, tConsensus, _ := messagelayer.GetConsensusMana(deps.Local.ID(), t)
	nodeMana := jsonmodels.Mana{
		Access:             accessMana,
		AccessTimestamp:    tAccess,
		Consensus:          consensusMana,
		ConsensusTimestamp: tConsensus,
	}

	nodeQueueSizes := make(map[string]int)
	for nodeID, size := range deps.Tangle.Scheduler.NodeQueueSizes() {
		nodeQueueSizes[nodeID.String()] = size
	}

	deficit, _ := deps.Tangle.Scheduler.GetDeficit(deps.Local.ID()).Float64()

	return c.JSON(http.StatusOK, jsonmodels.InfoResponse{
		Version:                 banner.AppVersion,
		NetworkVersion:          discovery.Parameters.NetworkVersion,
		TangleTime:              tangleTime,
		IdentityID:              base58.Encode(deps.Local.Identity.ID().Bytes()),
		IdentityIDShort:         deps.Local.Identity.ID().String(),
		PublicKey:               deps.Local.PublicKey().String(),
		MessageRequestQueueSize: int(metrics.MessageRequestQueueSize()),
		SolidMessageCount: int(metrics.InitialMessageCountPerComponentGrafana()[metrics.Solidifier] +
			metrics.MessageCountSinceStartPerComponentGrafana()[metrics.Solidifier]),
		TotalMessageCount: int(metrics.InitialMessageCountPerComponentGrafana()[metrics.Store] +
			metrics.MessageCountSinceStartPerComponentGrafana()[metrics.Store]),
		EnabledPlugins:  enabledPlugins,
		DisabledPlugins: disabledPlugins,
		Mana:            nodeMana,
		Scheduler: jsonmodels.Scheduler{
			Running:           deps.Tangle.Scheduler.Running(),
			Rate:              deps.Tangle.Scheduler.Rate().String(),
			MaxBufferSize:     deps.Tangle.Scheduler.MaxBufferSize(),
			CurrentBufferSize: deps.Tangle.Scheduler.BufferSize(),
			Deficit:           deficit,
			NodeQueueSizes:    nodeQueueSizes,
		},

		LastCommittedEpoch: *jsonmodels.EpochInfoFromRecord(metrics.GetLastCommittedEpoch()),
		RateSetter: jsonmodels.RateSetter{
			Rate:     deps.Tangle.RateSetter.Rate(),
			Size:     deps.Tangle.RateSetter.Size(),
			Estimate: deps.Tangle.RateSetter.Estimate(),
		},
	})
}
