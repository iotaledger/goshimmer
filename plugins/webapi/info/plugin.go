package info

import (
	"net/http"
	"sort"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/mr-tron/base58/base58"
	"go.uber.org/dig"

	"github.com/iotaledger/goshimmer/packages/app/blockissuer"
	"github.com/iotaledger/goshimmer/packages/app/collector"
	"github.com/iotaledger/goshimmer/packages/app/jsonmodels"
	"github.com/iotaledger/goshimmer/packages/core/latestblocktracker"
	"github.com/iotaledger/goshimmer/packages/node"
	"github.com/iotaledger/goshimmer/packages/protocol"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/consensus/blockgadget"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
	"github.com/iotaledger/goshimmer/plugins/autopeering/discovery"
	"github.com/iotaledger/goshimmer/plugins/banner"
	"github.com/iotaledger/goshimmer/plugins/dashboardmetrics"
	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/lo"
	"github.com/iotaledger/hive.go/runtime/event"
)

// PluginName is the name of the web API info endpoint plugin.
const PluginName = "WebAPIInfoEndpoint"

type dependencies struct {
	dig.In

	Server      *echo.Echo
	Local       *peer.Local
	Protocol    *protocol.Protocol
	BlockIssuer *blockissuer.BlockIssuer
}

var (
	// Plugin is the plugin instance of the web API info endpoint plugin.
	Plugin             *node.Plugin
	deps               = new(dependencies)
	lastAcceptedBlock  = latestblocktracker.New()
	lastConfirmedBlock = latestblocktracker.New()
)

func init() {
	Plugin = node.NewPlugin(PluginName, deps, node.Enabled, run)
}

func run(plugin *node.Plugin) {
	deps.Protocol.Events.Engine.Consensus.BlockGadget.BlockAccepted.Hook(func(block *blockgadget.Block) {
		lastAcceptedBlock.Update(block.ModelsBlock)
	}, event.WithWorkerPool(plugin.WorkerPool))

	deps.Protocol.Events.Engine.Consensus.BlockGadget.BlockConfirmed.Hook(func(block *blockgadget.Block) {
		lastConfirmedBlock.Update(block.ModelsBlock)
	}, event.WithWorkerPool(plugin.WorkerPool))

	deps.Server.GET("info", getInfo)
}

// getInfo returns the info of the node
// e.g.,
//
//	{
//		"version":"v0.2.0",
//		"tangleTime":{
//			"blockID":"24Uq4UFQ7p5oLyjuXX32jHhNreo5hY9eo8Awh36RhdTHCwFMtct3SE2rhe3ceYz6rjKDjBs3usoHS3ujFEabP5ri",
//			"time":1595528075204868900,
//			"synced":true
//	}
//
//		"identityID":"5bf4aa1d6c47e4ce",
//		"publickey":"CjUsn86jpFHWnSCx3NhWfU4Lk16mDdy1Hr7ERSTv3xn9",
//		"enabledplugins":[
//			"Config",
//			"AutoPeering",
//			"Analysis",
//			"WebAPIDataEndpoint",
//			"Protocol",
//			"CLI",
//			"Database",
//			"WebAPIAutoPeeringEndpoint",
//			"Metrics",
//			"PortCheck",
//			"Dashboard",
//			"WebAPI",
//			"WebAPIInfoEndpoint",
//			"WebAPIBlockEndpoint",
//			"Banner",
//			"Gossip",
//			"GracefulShutdown",
//			"Logger"
//		],
//		"disabledplugins":[
//			"RemoteLog",
//			"Spammer",
//			"WebAPIAuth"
//		]
//	}
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
	tm := deps.Protocol.Engine().Clock
	lastAcceptedBlockID := models.EmptyBlockID
	lastConfirmedBlockID := models.EmptyBlockID

	if lastAcceptedBlock != nil {
		lastAcceptedBlockID = lastAcceptedBlock.BlockID()
	}
	if lastConfirmedBlock != nil {
		lastConfirmedBlockID = lastConfirmedBlock.BlockID()
	}

	tangleTime := jsonmodels.TangleTime{
		Synced:           deps.Protocol.Engine().IsSynced(),
		Bootstrapped:     deps.Protocol.Engine().IsBootstrapped(),
		AcceptedBlockID:  lastAcceptedBlockID.Base58(),
		ConfirmedBlockID: lastConfirmedBlockID.Base58(),
		ConfirmedSlot:    int64(deps.Protocol.Engine().LastConfirmedSlot()),

		ATT:  tm.Accepted().Time().UnixNano(),
		RATT: tm.Accepted().RelativeTime().UnixNano(),
		CTT:  tm.Confirmed().Time().UnixNano(),
		RCTT: tm.Confirmed().RelativeTime().UnixNano(),
	}

	accessMana, _ := deps.Protocol.Engine().ThroughputQuota.Balance(deps.Local.ID())
	consensusMana := lo.Return1(deps.Protocol.Engine().SybilProtection.Weights().Get(deps.Local.ID())).Value
	nodeMana := jsonmodels.Mana{
		Access:             accessMana,
		AccessTimestamp:    time.Now(),
		Consensus:          consensusMana,
		ConsensusTimestamp: time.Now(),
	}

	issuerQueueSizes := make(map[string]int)
	for issuerID, size := range deps.Protocol.CongestionControl.Scheduler().IssuerQueueSizes() {
		issuerQueueSizes[issuerID.String()] = size
	}
	scheduler := deps.Protocol.CongestionControl.Scheduler()
	deficit, _ := scheduler.Deficit(deps.Local.ID()).Float64()

	return c.JSON(http.StatusOK, jsonmodels.InfoResponse{
		Version:               banner.AppVersion,
		NetworkVersion:        discovery.Parameters.NetworkVersion,
		TangleTime:            tangleTime,
		IdentityID:            base58.Encode(lo.PanicOnErr(deps.Local.Identity.ID().Bytes())),
		IdentityIDShort:       deps.Local.Identity.ID().String(),
		PublicKey:             deps.Local.PublicKey().String(),
		BlockRequestQueueSize: int(dashboardmetrics.BlockRequestQueueSize()),
		SolidBlockCount:       int(dashboardmetrics.BlockCountSinceStartPerComponentGrafana()[collector.Solidified]),
		TotalBlockCount:       int(dashboardmetrics.BlockCountSinceStartPerComponentGrafana()[collector.Attached]),
		EnabledPlugins:        enabledPlugins,
		DisabledPlugins:       disabledPlugins,
		Mana:                  nodeMana,
		Scheduler: jsonmodels.Scheduler{
			Running:           scheduler.IsRunning(),
			Rate:              scheduler.Rate().String(),
			MaxBufferSize:     scheduler.MaxBufferSize(),
			CurrentBufferSize: scheduler.BufferSize(),
			Deficit:           deficit,
			NodeQueueSizes:    issuerQueueSizes,
		},
		RateSetter: jsonmodels.RateSetter{
			Rate:     deps.BlockIssuer.Rate(),
			Estimate: deps.BlockIssuer.Estimate(),
		},
	})
}
