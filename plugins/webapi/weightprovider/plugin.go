package weightprovider

import (
	"net/http"

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
	activeValidators := deps.Protocol.Instance().ValidatorSet

	activeValidatorsString := make([]string, 0)

	for _, validator := range activeValidators.Slice() {
		activeValidatorsString = append(activeValidatorsString, validator.ID().String())
	}

	return c.JSON(http.StatusOK, activeValidatorsString)
}

func getWeightsHandler(c echo.Context) (err error) {
	validatorSet := deps.Protocol.Instance().Engine.Tangle.ValidatorSet

	weightsString := make(map[string]int64)
	for _, validator := range validatorSet.Slice() {
		weightsString[validator.ID().String()] = validator.Weight()
	}
	resp := Weights{
		Weights:     weightsString,
		TotalWeight: validatorSet.TotalWeight(),
	}

	return c.JSON(http.StatusOK, resp)
}

// Weights defines the weights associated to the nodes.
type Weights struct {
	Weights     map[string]int64 `json:"weights"`
	TotalWeight int64            `json:"totalWeight"`
}
