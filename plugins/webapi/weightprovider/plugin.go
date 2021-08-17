package weightprovider

import (
	"net/http"
	"time"

	"github.com/iotaledger/hive.go/node"
	"github.com/labstack/echo"
	"go.uber.org/dig"

	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/goshimmer/plugins/dependencyinjection"
)

var (
	// Plugin is the plugin instance of the web API mana endpoint plugin.
	Plugin *node.Plugin
	deps   dependencies
)

type dependencies struct {
	dig.In

	Server *echo.Echo
	Tangle *tangle.Tangle
}

func init() {
	Plugin = node.NewPlugin("WebAPI WeightProvider Endpoint", node.Enabled, configure)
}

func configure(_ *node.Plugin) {
	dependencyinjection.Container.Invoke(func(dep dependencies) {
		deps = dep
	})

	deps.Server.GET("weightprovider/activenodes", getNodesHandler)
	deps.Server.GET("weightprovider/weights", getWeightsHandler)
}

func getNodesHandler(c echo.Context) (err error) {
	activeNodes := deps.Tangle.WeightProvider.(*tangle.CManaWeightProvider).ActiveNodes()

	activeNodesString := make(map[string]time.Time)
	for nodeID, t := range activeNodes {
		activeNodesString[nodeID.String()] = t
	}

	return c.JSON(http.StatusOK, activeNodesString)
}

func getWeightsHandler(c echo.Context) (err error) {
	weights, totalWeight := deps.Tangle.WeightProvider.WeightsOfRelevantSupporters()

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
