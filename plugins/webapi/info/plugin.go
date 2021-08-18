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
	"github.com/iotaledger/goshimmer/packages/mana"
	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/goshimmer/plugins/autopeering/discovery"
	"github.com/iotaledger/goshimmer/plugins/banner"
	"github.com/iotaledger/goshimmer/plugins/dependencyinjection"
	"github.com/iotaledger/goshimmer/plugins/manarefresher"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
	"github.com/iotaledger/goshimmer/plugins/metrics"
)

// PluginName is the name of the web API info endpoint plugin.
const PluginName = "WebAPI info Endpoint"

type dependencies struct {
	dig.In

	Server *echo.Echo
	Local  *peer.Local
	Tangle *tangle.Tangle
}

var (
	// Plugin is the plugin instance of the web API info endpoint plugin.
	Plugin *node.Plugin
	deps   dependencies
)

func init() {
	Plugin = node.NewPlugin(PluginName, node.Enabled, configure)
}

func configure(plugin *node.Plugin) {
	if err := dependencyinjection.Container.Invoke(func(dep dependencies) {
		deps = dep
	}); err != nil {
		plugin.LogError(err)
	}
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
// 		"Autopeering",
// 		"Analysis",
// 		"WebAPI data Endpoint",
// 		"WebAPI dRNG Endpoint",
// 		"MessageLayer",
// 		"CLI",
// 		"Database",
// 		"DRNG",
// 		"WebAPI autopeering Endpoint",
// 		"Metrics",
// 		"PortCheck",
// 		"Dashboard",
// 		"WebAPI",
// 		"WebAPI info Endpoint",
// 		"WebAPI message Endpoint",
// 		"Banner",
// 		"Gossip",
// 		"Graceful Shutdown",
// 		"Logger"
// 	],
// 	"disabledplugins":[
// 		"RemoteLog",
// 		"Spammer",
// 		"WebAPI Auth"
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
	lcm := deps.Tangle.TimeManager.LastConfirmedMessage()
	tangleTime := jsonmodels.TangleTime{
		Synced:    deps.Tangle.TimeManager.Synced(),
		Time:      lcm.Time.UnixNano(),
		MessageID: lcm.MessageID.Base58(),
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

	var delegationAddressString string
	delegationAddress, err := manarefresher.DelegationAddress()
	if err == nil {
		delegationAddressString = delegationAddress.Base58()
	}

	nodeQueueSizes := make(map[string]int)
	for nodeID, size := range deps.Tangle.Scheduler.NodeQueueSizes() {
		nodeQueueSizes[nodeID.String()] = size
	}

	return c.JSON(http.StatusOK, jsonmodels.InfoResponse{
		Version:                 banner.AppVersion,
		NetworkVersion:          discovery.NetworkVersion(),
		TangleTime:              tangleTime,
		IdentityID:              base58.Encode(deps.Local.Identity.ID().Bytes()),
		IdentityIDShort:         deps.Local.Identity.ID().String(),
		PublicKey:               deps.Local.PublicKey().String(),
		MessageRequestQueueSize: int(metrics.MessageRequestQueueSize()),
		SolidMessageCount:       int(metrics.MessageSolidCountDB()),
		TotalMessageCount:       int(metrics.MessageTotalCountDB()),
		EnabledPlugins:          enabledPlugins,
		DisabledPlugins:         disabledPlugins,
		Mana:                    nodeMana,
		ManaDelegationAddress:   delegationAddressString,
		ManaDecay:               mana.Decay,
		Scheduler: jsonmodels.Scheduler{
			Running:        deps.Tangle.Scheduler.Running(),
			Rate:           deps.Tangle.Scheduler.Rate().String(),
			NodeQueueSizes: nodeQueueSizes,
		},
	})
}
