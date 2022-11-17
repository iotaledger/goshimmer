package weightprovider

import (
	"net/http"

	"github.com/iotaledger/hive.go/core/identity"
	"github.com/iotaledger/hive.go/core/node"
	"github.com/labstack/echo"
	"go.uber.org/dig"

	"github.com/iotaledger/goshimmer/packages/core/validator"
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

	deps.Protocol.Engine().Tangle.ActiveNodes.ForEach(func(id identity.ID, _ *validator.Validator) bool {
		activeValidatorsString = append(activeValidatorsString, id.String())

		return true
	})

	return c.JSON(http.StatusOK, activeValidatorsString)
}

func getWeightsHandler(c echo.Context) (err error) {
	weightsString := make(map[string]int64)
	deps.Protocol.Engine().Tangle.ActiveNodes.ForEach(func(id identity.ID, validator *validator.Validator) bool {
		weightsString[id.String()] = validator.Weight()

		return true
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
