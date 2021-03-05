package info

import (
	"net/http"
	"sort"
	goSync "sync"
	"time"

	"github.com/iotaledger/goshimmer/packages/mana"
	"github.com/iotaledger/goshimmer/plugins/autopeering"
	"github.com/iotaledger/goshimmer/plugins/autopeering/local"
	"github.com/iotaledger/goshimmer/plugins/banner"
	manaPlugin "github.com/iotaledger/goshimmer/plugins/mana"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
	"github.com/iotaledger/goshimmer/plugins/metrics"
	"github.com/iotaledger/goshimmer/plugins/webapi"
	"github.com/iotaledger/hive.go/node"
	"github.com/labstack/echo"
	"github.com/mr-tron/base58/base58"
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
//  "synchronized": true,
//	"beacons":[{
// 		"public_key":"EYsaGXnUVA9aTYL9FwYEvoQ8d1HCJveQVL7vogu6pqCP",
// 		"msg_id":"24Uq4UFQ7p5oLyjuXX32jHhNreo5hY9eo8Awh36RhdTHCwFMtct3SE2rhe3ceYz6rjKDjBs3usoHS3ujFEabP5ri",
// 		"sent_time":1595528075204868900,
// 		"synced":true
// }]
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

	synced, beacons := messagelayer.SyncStatus()
	var beaconsStatus []Beacon
	for publicKey, s := range beacons {
		beaconsStatus = append(beaconsStatus, Beacon{
			PublicKey: publicKey.String(),
			MsgID:     s.MsgID.String(),
			SentTime:  s.SentTime,
			Synced:    s.Synced,
		})
	}

	t := time.Now()
	accessMana, tAccess, _ := manaPlugin.GetAccessMana(local.GetInstance().ID(), t)
	consensusMana, tConsensus, _ := manaPlugin.GetConsensusMana(local.GetInstance().ID(), t)
	nodeMana := Mana{
		Access:             accessMana,
		AccessTimestamp:    tAccess,
		Consensus:          consensusMana,
		ConsensusTimestamp: tConsensus,
	}

	return c.JSON(http.StatusOK, Response{
		Version:                 banner.AppVersion,
		NetworkVersion:          autopeering.NetworkVersion(),
		Synced:                  synced,
		Beacons:                 beaconsStatus,
		IdentityID:              base58.Encode(local.GetInstance().Identity.ID().Bytes()),
		IdentityIDShort:         local.GetInstance().Identity.ID().String(),
		PublicKey:               local.GetInstance().PublicKey().String(),
		MessageRequestQueueSize: int(metrics.MessageRequestQueueSize()),
		SolidMessageCount:       int(metrics.MessageSolidCountDB()),
		TotalMessageCount:       int(metrics.MessageTotalCountDB()),
		EnabledPlugins:          enabledPlugins,
		DisabledPlugins:         disabledPlugins,
		Mana:                    nodeMana,
		ManaDecay:               mana.Decay,
	})
}

// Response holds the response of the GET request.
type Response struct {
	// version of GoShimmer
	Version string `json:"version,omitempty"`
	// Network Version of the autopeering
	NetworkVersion uint32 `json:"networkVersion,omitempty"`
	// whether the node is synchronized
	Synced bool `json:"synced"`
	// sync beacons status
	Beacons []Beacon `json:"beacons"`
	// identity ID of the node encoded in base58
	IdentityID string `json:"identityID,omitempty"`
	// identity ID of the node encoded in base58 and truncated to its first 8 bytes
	IdentityIDShort string `json:"identityIDShort,omitempty"`
	// public key of the node encoded in base58
	PublicKey string `json:"publicKey,omitempty"`
	// MessageRequestQueueSize is the number of messages a node is trying to request from neighbors.
	MessageRequestQueueSize int `json:"messageRequestQueueSize,omitempty"`
	// SolidMessageCount is the number of solid messages in the node's database.
	SolidMessageCount int `json:"solidMessageCount,omitempty"`
	// TotalMessageCount is the number of messages in the node's database.
	TotalMessageCount int `json:"totalMessageCount,omitempty"`
	// list of enabled plugins
	EnabledPlugins []string `json:"enabledPlugins,omitempty"`
	// list if disabled plugins
	DisabledPlugins []string `json:"disabledPlugins,omitempty"`
	// Mana values
	Mana Mana `json:"mana,omitempty"`
	// ManaDecay is the decay coefficient of bm2.
	ManaDecay float64 `json:"mana_decay"`
	// error of the response
	Error string `json:"error,omitempty"`
}

// Beacon contains a sync beacons detailed status.
type Beacon struct {
	PublicKey string `json:"public_key"`
	MsgID     string `json:"msg_id"`
	SentTime  int64  `json:"sent_time"`
	Synced    bool   `json:"synced"`
}

// Mana contains the different mana values of the node.
type Mana struct {
	Access             float64   `json:"access"`
	AccessTimestamp    time.Time `json:"accessTimestamp"`
	Consensus          float64   `json:"consensus"`
	ConsensusTimestamp time.Time `json:"consensusTimestamp"`
}
