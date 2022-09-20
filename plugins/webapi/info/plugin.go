package info

import (
	"net/http"
	"sort"
	"time"

	"github.com/iotaledger/hive.go/core/autopeering/peer"
	"github.com/iotaledger/hive.go/core/node"
	"github.com/labstack/echo"
	"github.com/mr-tron/base58/base58"
	"go.uber.org/dig"

	"github.com/iotaledger/goshimmer/packages/app/jsonmodels"
	"github.com/iotaledger/goshimmer/packages/protocol"

	"github.com/iotaledger/goshimmer/plugins/autopeering/discovery"
	"github.com/iotaledger/goshimmer/plugins/banner"
	"github.com/iotaledger/goshimmer/plugins/blocklayer"
	"github.com/iotaledger/goshimmer/plugins/metrics"
)

// PluginName is the name of the web API info endpoint plugin.
const PluginName = "WebAPIInfoEndpoint"

type dependencies struct {
	dig.In

	Server   *echo.Echo
	Local    *peer.Local
	Protocol *protocol.Protocol
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
// 		"blockID":"24Uq4UFQ7p5oLyjuXX32jHhNreo5hY9eo8Awh36RhdTHCwFMtct3SE2rhe3ceYz6rjKDjBs3usoHS3ujFEabP5ri",
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
// 		"BlockLayer",
// 		"CLI",
// 		"Database",
// 		"WebAPIAutoPeeringEndpoint",
// 		"Metrics",
// 		"PortCheck",
// 		"Dashboard",
// 		"WebAPI",
// 		"WebAPIInfoEndpoint",
// 		"WebAPIBlockEndpoint",
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
	tm := deps.Protocol.Instance().Engine.Clock
	// TODO: figure out where to take last accepted block from
	lcm := tm.LastAcceptedBlock()
	tangleTime := jsonmodels.TangleTime{
		Synced:          deps.Protocol.Instance().Engine.IsSynced(),
		AcceptedBlockID: lcm.BlockID.Base58(),
		ATT:             tm.AcceptedTime().UnixNano(),
		RATT:            tm.RelativeAcceptedTime().UnixNano(),
		CTT:             tm.ConfirmedTime().UnixNano(),
		RCTT:            tm.RelativeConfirmedTime().UnixNano(),
	}

	t := time.Now()
	accessMana, tAccess, _ := blocklayer.GetAccessMana(deps.Local.ID(), t)
	consensusMana, tConsensus, _ := blocklayer.GetConsensusMana(deps.Local.ID(), t)
	nodeMana := jsonmodels.Mana{
		Access:             accessMana,
		AccessTimestamp:    tAccess,
		Consensus:          consensusMana,
		ConsensusTimestamp: tConsensus,
	}

	issuerQueueSizes := make(map[string]int)
	for issuerID, size := range deps.Protocol.Instance().Engine.CongestionControl.Scheduler.IssuerQueueSizes() {
		issuerQueueSizes[issuerID.String()] = size
	}
	scheduler := deps.Protocol.Instance().Engine.CongestionControl.Scheduler
	deficit, _ := scheduler.Deficit(deps.Local.ID()).Float64()

	return c.JSON(http.StatusOK, jsonmodels.InfoResponse{
		Version:               banner.AppVersion,
		NetworkVersion:        discovery.Parameters.NetworkVersion,
		TangleTime:            tangleTime,
		IdentityID:            base58.Encode(deps.Local.Identity.ID().Bytes()),
		IdentityIDShort:       deps.Local.Identity.ID().String(),
		PublicKey:             deps.Local.PublicKey().String(),
		BlockRequestQueueSize: int(metrics.BlockRequestQueueSize()),
		SolidBlockCount: int(metrics.InitialBlockCountPerComponentGrafana()[metrics.Solidifier] +
			metrics.BlockCountSinceStartPerComponentGrafana()[metrics.Solidifier]),
		TotalBlockCount: int(metrics.InitialBlockCountPerComponentGrafana()[metrics.Store] +
			metrics.BlockCountSinceStartPerComponentGrafana()[metrics.Store]),
		EnabledPlugins:  enabledPlugins,
		DisabledPlugins: disabledPlugins,
		Mana:            nodeMana,
		Scheduler: jsonmodels.Scheduler{
			Running:           scheduler.Running(),
			Rate:              scheduler.Rate().String(),
			MaxBufferSize:     scheduler.MaxBufferSize(),
			CurrentBufferSize: scheduler.BufferSize(),
			Deficit:           deficit,
			NodeQueueSizes:    issuerQueueSizes,
		},
		//TODO: finish when ratesetter is available
		//RateSetter: jsonmodels.RateSetter{
		//	Rate:     deps.Tangle.RateSetter.Rate(),
		//	Size:     deps.Tangle.RateSetter.Size(),
		//	Estimate: deps.Tangle.RateSetter.Estimate(),
		//},
	})
}
