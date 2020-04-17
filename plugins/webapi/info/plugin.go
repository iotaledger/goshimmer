package info

import (
	"net/http"

	"github.com/iotaledger/goshimmer/plugins/autopeering/local"
	"github.com/iotaledger/goshimmer/plugins/banner"
	"github.com/iotaledger/goshimmer/plugins/webapi"
	"github.com/iotaledger/hive.go/node"
	"github.com/labstack/echo"
	"github.com/mr-tron/base58"
)

var PLUGIN = node.NewPlugin("WebAPI info Endpoint", node.Enabled, configure)

func configure(plugin *node.Plugin) {
	webapi.Server.GET("info", getInfo)
}

// getInfo returns the info of the node
// e.g.,
// {
// 	"version":"v0.2.0",
// 	"identity":"7BxV1v3nFHefn4J88jeZebqnJRvSHt1jC7ME6tmKLhy7",
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
// 		"SPA",
// 		"WebAPI",
// 		"WebAPI info Endpoint",
// 		"WebAPI message Endpoint",
// 		"Banner",
// 		"Gossip",
// 		"Graceful Shutdown",
// 		"Logger"
// 	],
// 	"disabledlugins":[
// 		"Graph",
// 		"RemoteLog",
// 		"Spammer",
// 		"WebAPI Auth"
// 	]
// }
func getInfo(c echo.Context) error {
	enabledPlugins := []string{}
	disabledPlugins := []string{}
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
		Identity:        base58.Encode(local.GetInstance().Identity.ID().Bytes()),
		PublicKey:       base58.Encode(local.GetInstance().PublicKey().Bytes()),
		EnabledPlugins:  enabledPlugins,
		DisabledPlugins: disabledPlugins,
	})
}

// Response holds the response of the GET request.
type Response struct {
	// version of GoShimmer
	Version string `json:"version,omitempty"`
	// identity of the node encoded in base58
	Identity string `json:"identity,omitempty"`
	// public key of the node encoded in base58
	PublicKey string `json:"publickey,omitempty"`
	// list of enabled plugins
	EnabledPlugins []string `json:"enabledplugins,omitempty"`
	// list if disabled plugins
	DisabledPlugins []string `json:"disabledlugins,omitempty"`
	// error of the response
	Error string `json:"error,omitempty"`
}
