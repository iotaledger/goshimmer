package info

import (
	"net/http"
	"sort"
	goSync "sync"
	"time"

	"github.com/iotaledger/hive.go/node"
	"github.com/labstack/echo"
	"github.com/mr-tron/base58/base58"

	"github.com/iotaledger/goshimmer/packages/jsonmodels"
	"github.com/iotaledger/goshimmer/packages/mana"
	"github.com/iotaledger/goshimmer/plugins/autopeering/discovery"
	"github.com/iotaledger/goshimmer/plugins/autopeering/local"
	"github.com/iotaledger/goshimmer/plugins/banner"
	"github.com/iotaledger/goshimmer/plugins/manarefresher"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
	"github.com/iotaledger/goshimmer/plugins/metrics"
	"github.com/iotaledger/goshimmer/plugins/webapi"
)

// PluginName is the name of the web API info endpoint plugin.
const PluginName = "WebAPI info Endpoint"

var (
	// plugin is the plugin instance of the web API info endpoint plugin.
	plugin *node.Plugin
	once   goSync.Once
)

// Plugin gets the plugin instance.
func Plugin() *node.Plugin {
	once.Do(func() {
		plugin = node.NewPlugin(PluginName, node.Enabled, configure)
	})
	return plugin
}

func configure(_ *node.Plugin) {
	webapi.Server().GET("info", getInfo)
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
	lcm := messagelayer.Tangle().TimeManager.LastConfirmedMessage()
	tangleTime := jsonmodels.TangleTime{
		Synced:    messagelayer.Tangle().TimeManager.Synced(),
		Time:      lcm.Time.UnixNano(),
		MessageID: lcm.MessageID.Base58(),
	}

	t := time.Now()
	accessMana, tAccess, _ := messagelayer.GetAccessMana(local.GetInstance().ID(), t)
	consensusMana, tConsensus, _ := messagelayer.GetConsensusMana(local.GetInstance().ID(), t)
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
	for nodeID, size := range messagelayer.Tangle().Scheduler.NodeQueueSizes() {
		nodeQueueSizes[nodeID.String()] = size
	}

	return c.JSON(http.StatusOK, jsonmodels.InfoResponse{
		Version:                 banner.AppVersion,
		NetworkVersion:          discovery.NetworkVersion(),
		TangleTime:              tangleTime,
		IdentityID:              base58.Encode(local.GetInstance().Identity.ID().Bytes()),
		IdentityIDShort:         local.GetInstance().Identity.ID().String(),
		PublicKey:               local.GetInstance().PublicKey().String(),
		MessageRequestQueueSize: int(metrics.MessageRequestQueueSize()),
		SolidMessageCount:       int(metrics.MessageSolidCountDB()),
		TotalMessageCount:       int(metrics.MessageTotalCountDB()),
		EnabledPlugins:          enabledPlugins,
		DisabledPlugins:         disabledPlugins,
		Mana:                    nodeMana,
		ManaDelegationAddress:   delegationAddressString,
		ManaDecay:               mana.Decay,
		Scheduler: jsonmodels.Scheduler{
			Running:        messagelayer.Tangle().Scheduler.Running(),
			Rate:           messagelayer.Tangle().Scheduler.Rate().String(),
			NodeQueueSizes: nodeQueueSizes,
		},
		RateSetter: jsonmodels.RateSetter{
			Rate:     messagelayer.Tangle().RateSetter.Rate(),
			Size:     messagelayer.Tangle().RateSetter.Size(),
			Estimate: messagelayer.Tangle().RateSetter.Estimate().String(),
		},
	})
}
