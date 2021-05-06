package weightprovider

import (
	"net/http"
	"sync"
	"time"

	"github.com/iotaledger/hive.go/node"
	"github.com/labstack/echo"

	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
	"github.com/iotaledger/goshimmer/plugins/webapi"
)

var (
	// plugin is the plugin instance of the web API mana endpoint plugin.
	plugin *node.Plugin
	once   sync.Once
)

// Plugin gets the plugin instance.
func Plugin() *node.Plugin {
	once.Do(func() {
		plugin = node.NewPlugin("WebAPI WeightProvider Endpoint", node.Enabled, configure)
	})
	return plugin
}

func configure(_ *node.Plugin) {
	webapi.Server().GET("weightprovider/activenodes", getNodesHandler)
	webapi.Server().GET("weightprovider/weights", getWeightsHandler)
}

func getNodesHandler(c echo.Context) (err error) {
	activeNodes := messagelayer.Tangle().WeightProvider.(*tangle.CManaWeightProvider).ActiveNodes()

	activeNodesString := make(map[string]time.Time)
	for nodeID, t := range activeNodes {
		activeNodesString[nodeID.String()] = t
	}

	return c.JSON(http.StatusOK, activeNodesString)
}

func getWeightsHandler(c echo.Context) (err error) {
	weights, totalWeight := messagelayer.Tangle().WeightProvider.WeightsOfRelevantSupporters()

	weightsString := make(map[string]float64)
	for nodeID, mana := range weights {
		weightsString[nodeID.String()] = mana
	}
	resp := Weights{
		Weights:     weightsString,
		TotalWeight: totalWeight,
	}

	return c.JSON(http.StatusOK, resp)
}

// Weights defines the weights associated to the nodes.
type Weights struct {
	Weights     map[string]float64 `json:"weights"`
	TotalWeight float64            `json:"totalWeight"`
}
