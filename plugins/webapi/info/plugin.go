package info

import (
	"net/http"

	"github.com/iotaledger/goshimmer/plugins/autopeering/local"
	"github.com/iotaledger/goshimmer/plugins/banner"
	"github.com/iotaledger/goshimmer/plugins/sync"
	"github.com/iotaledger/goshimmer/plugins/webapi"
	"github.com/iotaledger/hive.go/node"
	"github.com/labstack/echo"
)

// PluginName is the name of the web API info endpoint plugin.
const PluginName = "WebAPI info Endpoint"

// Plugin is the plugin instance of the web API info endpoint plugin.
var Plugin = node.NewPlugin(PluginName, node.Enabled, configure)

func configure(_ *node.Plugin) {
	webapi.Server.GET("info", getInfo)
}

// getInfo returns the info of the node
// e.g.,
// {
// 	"version":"v0.2.0",
//  "synchronized": true,
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
// 	"disabledlugins":[
// 		"RemoteLog",
// 		"Spammer",
// 		"WebAPI Auth"
// 	]
// }
func getInfo(c echo.Context) error {
	var enabledPlugins []string
	var disabledPlugins []string
	for plugin, status := range node.GetPlugins() {
		switch status {
		case node.Disabled:
			disabledPlugins = append(disabledPlugins, plugin)
		case node.Enabled:
			enabledPlugins = append(enabledPlugins, plugin)
		default:
			continue
		}
	}

	return c.JSON(http.StatusOK, Response{
		Version:         banner.AppVersion,
		Synced:          sync.Synced(),
		IdentityID:      local.GetInstance().Identity.ID().String(),
		PublicKey:       local.GetInstance().PublicKey().String(),
		EnabledPlugins:  enabledPlugins,
		DisabledPlugins: disabledPlugins,
	})
}

// Response holds the response of the GET request.
type Response struct {
	// version of GoShimmer
	Version string `json:"version,omitempty"`
	// whether the node is synchronized
	Synced bool `json:"synced"`
	// identity ID of the node encoded in hex and truncated to its first 8 bytes
	IdentityID string `json:"identityID,omitempty"`
	// public key of the node encoded in base58
	PublicKey string `json:"publickey,omitempty"`
	// list of enabled plugins
	EnabledPlugins []string `json:"enabledplugins,omitempty"`
	// list if disabled plugins
	DisabledPlugins []string `json:"disabledlugins,omitempty"`
	// error of the response
	Error string `json:"error,omitempty"`
}
