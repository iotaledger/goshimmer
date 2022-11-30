package info

import (
	"net/http"
	"sort"
	"time"

	"github.com/iotaledger/hive.go/core/autopeering/peer"
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/node"
	"github.com/labstack/echo"
	"github.com/mr-tron/base58/base58"
	"go.uber.org/dig"

	"github.com/iotaledger/goshimmer/packages/app/blockissuer"
	"github.com/iotaledger/goshimmer/packages/app/jsonmodels"
	"github.com/iotaledger/goshimmer/packages/protocol"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/consensus/blockgadget"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
	"github.com/iotaledger/goshimmer/plugins/autopeering/discovery"
	"github.com/iotaledger/goshimmer/plugins/banner"
	"github.com/iotaledger/goshimmer/plugins/metrics"
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
	lastAcceptedBlock  *blockgadget.Block
	lastConfirmedBlock *blockgadget.Block
)

func init() {
	Plugin = node.NewPlugin(PluginName, deps, node.Enabled, configure)
}

func configure(_ *node.Plugin) {
	deps.Protocol.Events.Engine.Consensus.BlockGadget.BlockAccepted.Attach(event.NewClosure(func(block *blockgadget.Block) {
		if lastAcceptedBlock == nil || lastAcceptedBlock.IssuingTime().Before(block.IssuingTime()) {
			lastAcceptedBlock = block
		}
	}))
	deps.Protocol.Events.Engine.Consensus.BlockGadget.BlockConfirmed.Attach(event.NewClosure(func(block *blockgadget.Block) {
		if lastConfirmedBlock == nil || lastConfirmedBlock.IssuingTime().Before(block.IssuingTime()) {
			lastConfirmedBlock = block
		}
	}))
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
		lastAcceptedBlockID = lastAcceptedBlock.ID()
	}
	if lastConfirmedBlock != nil {
		lastConfirmedBlockID = lastConfirmedBlock.ID()
	}

	tangleTime := jsonmodels.TangleTime{
		Synced:           deps.Protocol.Engine().IsSynced(),
		Bootstrapped:     deps.Protocol.Engine().IsBootstrapped(),
		AcceptedBlockID:  lastAcceptedBlockID.Base58(),
		ConfirmedBlockID: lastConfirmedBlockID.Base58(),
		ConfirmedEpoch:   int64(deps.Protocol.Engine().LastConfirmedEpoch()),

		ATT:  tm.AcceptedTime().UnixNano(),
		RATT: tm.RelativeAcceptedTime().UnixNano(),
		CTT:  tm.ConfirmedTime().UnixNano(),
		RCTT: tm.RelativeConfirmedTime().UnixNano(),
	}

	accessMana, _ := deps.Protocol.Engine().ManaTracker.Mana(deps.Local.ID())
	consensusMana := lo.Return1(deps.Protocol.Engine().SybilProtection.Weights().Weight(deps.Local.ID())).Value
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
		BlockRequestQueueSize: int(metrics.BlockRequestQueueSize()),
		SolidBlockCount: int(metrics.InitialBlockCountPerComponentGrafana()[metrics.Solidified] +
			metrics.BlockCountSinceStartPerComponentGrafana()[metrics.Solidified]),
		TotalBlockCount: int(metrics.InitialBlockCountPerComponentGrafana()[metrics.Attached] +
			metrics.BlockCountSinceStartPerComponentGrafana()[metrics.Attached]),
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
		RateSetter: jsonmodels.RateSetter{
			Rate:     deps.BlockIssuer.Rate(),
			Size:     deps.BlockIssuer.Size(),
			Estimate: deps.BlockIssuer.Estimate(),
		},
	})
}
