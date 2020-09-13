package mana

import (
	"net/http"
	goSync "sync"

	"github.com/iotaledger/goshimmer/plugins/autopeering/local"
	"github.com/iotaledger/goshimmer/plugins/mana"
	"github.com/iotaledger/goshimmer/plugins/webapi"
	"github.com/iotaledger/goshimmer/plugins/webapi/mana/all"
	"github.com/iotaledger/goshimmer/plugins/webapi/mana/nhighest"
	"github.com/iotaledger/goshimmer/plugins/webapi/mana/percentile"
	"github.com/iotaledger/hive.go/node"
	"github.com/labstack/echo"
)

// PluginName is the name of the web API mana endpoint plugin.
const PluginName = "WebAPI mana Endpoint"

var (
	// plugin is the plugin instance of the web API mana endpoint plugin.
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
	webapi.Server().GET("mana", getMana)
	webapi.Server().GET("mana/all", all.Handler)
	webapi.Server().GET("/mana/access/nhighest", nhighest.AccessHandler)
	webapi.Server().GET("/mana/consensus/nhighest", nhighest.ConsensusHandler)
	webapi.Server().GET("/mana/percentile", percentile.Handler)
}

func getMana(c echo.Context) error {
	accessMana, _ := mana.GetAccessMana(local.GetInstance().ID())
	consensusMana, _ := mana.GetConsensusMana(local.GetInstance().ID())

	return c.JSON(http.StatusOK, Response{
		Access:    accessMana,
		Consensus: consensusMana,
	})
}

// Response defines the response for get mana.
type Response struct {
	Access    float64 `json:"access"`
	Consensus float64 `json:"consensus"`
}
