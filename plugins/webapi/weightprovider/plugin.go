package weightprovider

import (
	"net/http"

	"github.com/iotaledger/hive.go/core/identity"
	"github.com/iotaledger/hive.go/core/node"
	"github.com/labstack/echo"
	"go.uber.org/dig"

	"github.com/iotaledger/goshimmer/packages/protocol"
)

var (
	// Plugin is the plugin instance of the web API mana endpoint plugin.
	Plugin *node.Plugin
	deps   = new(dependencies)
)

type dependencies struct {
	dig.In

	Server   *echo.Echo
	Protocol *protocol.Protocol
}

// TODO: rename to validatorset or sth?
func init() {
	Plugin = node.NewPlugin("WebAPIWeightProviderEndpoint", deps, node.Enabled, configure)
}

func configure(_ *node.Plugin) {
	deps.Server.GET("weightprovider/activeissuers", getIssuersHandler)
	deps.Server.GET("weightprovider/weights", getWeightsHandler)
}

func getIssuersHandler(c echo.Context) (err error) {
	activeValidatorsString := make([]string, 0)

	_ = deps.Protocol.Engine().Tangle.ActiveNodes.ForEach(func(id identity.ID) error {
		activeValidatorsString = append(activeValidatorsString, id.String())
		return nil
	})

	return c.JSON(http.StatusOK, activeValidatorsString)
}

func getWeightsHandler(c echo.Context) (err error) {
	weightsString := make(map[string]int64)
	_ = deps.Protocol.Engine().Tangle.ActiveNodes.ForEachWeighted(func(id identity.ID, weight int64) error {
		weightsString[id.String()] = weight
		return nil
	})

	resp := Weights{
		Weights:     weightsString,
		TotalWeight: deps.Protocol.Engine().Tangle.ActiveNodes.Weight(),
	}

	return c.JSON(http.StatusOK, resp)
}

// Weights defines the weights associated to the nodes.
type Weights struct {
	Weights     map[string]int64 `json:"weights"`
	TotalWeight int64            `json:"totalWeight"`
}
