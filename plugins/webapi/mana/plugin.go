package mana

import (
	"net/http"
	"sync"

	manaPkg "github.com/iotaledger/goshimmer/packages/mana"
	"github.com/iotaledger/hive.go/identity"

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
const PluginName = "WebAPI Mana Endpoint"

var (
	// plugin is the plugin instance of the web API mana endpoint plugin.
	plugin *node.Plugin
	once   sync.Once
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
	var request Request
	if err := c.Bind(&request); err != nil {
		return c.JSON(http.StatusBadRequest, Response{Error: err.Error()})
	}
	ID, err := manaPkg.IDFromStr(request.Node)
	if err != nil {
		return c.JSON(http.StatusBadRequest, Response{Error: err.Error()})
	}
	emptyID := identity.ID{}
	if ID == emptyID {
		ID = local.GetInstance().ID()
	}
	accessMana, err := mana.GetAccessMana(ID)
	if err != nil {
		return c.JSON(http.StatusBadRequest, Response{Error: err.Error()})
	}
	consensusMana, err := mana.GetConsensusMana(ID)
	if err != nil {
		return c.JSON(http.StatusBadRequest, Response{Error: err.Error()})
	}

	return c.JSON(http.StatusOK, Response{
		Node:      base58.Encode(ID.Bytes()),
		Access:    accessMana,
		Consensus: consensusMana,
	})
}

// Request is the request for get mana.
type Request struct {
	Node string `json:"node"`
}

// Response defines the response for get mana.
type Response struct {
	Error     string  `json:"error,omitempty"`
	Node      string  `json:"node"`
	Access    float64 `json:"access"`
	Consensus float64 `json:"consensus"`
}
