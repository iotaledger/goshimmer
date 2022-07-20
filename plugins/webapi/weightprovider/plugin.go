package weightprovider

import (
	"net/http"
	"strconv"

	"github.com/iotaledger/hive.go/node"
	"github.com/labstack/echo"
	"go.uber.org/dig"

	"github.com/iotaledger/goshimmer/packages/app/jsonmodels"
	"github.com/iotaledger/goshimmer/packages/core/markers"
	"github.com/iotaledger/goshimmer/packages/core/tangle"
)

var (
	// Plugin is the plugin instance of the web API mana endpoint plugin.
	Plugin *node.Plugin
	deps   = new(dependencies)
)

type dependencies struct {
	dig.In

	Server *echo.Echo
	Tangle *tangle.Tangle
}

func init() {
	Plugin = node.NewPlugin("WebAPIWeightProviderEndpoint", deps, node.Enabled, configure)
}

func configure(_ *node.Plugin) {
	deps.Server.GET("weightprovider/activenodes", getNodesHandler)
	deps.Server.GET("weightprovider/weights", getWeightsHandler)
	deps.Server.GET("weightprovider/markers/:sequenceID/:markerIndex/voters", getMarkerVoters)
	deps.Server.GET("weightprovider/markers/:sequenceID/:markerIndex/weight", getMarkerWeight)
}

func getNodesHandler(c echo.Context) (err error) {
	activeNodes := deps.Tangle.WeightProvider.(*tangle.CManaWeightProvider).ActiveNodes()

	activeNodesString := make(map[string][]int64)
	for nodeID, al := range activeNodes {
		activeNodesString[nodeID.String()] = al.Times()
	}

	return c.JSON(http.StatusOK, activeNodesString)
}

func getWeightsHandler(c echo.Context) (err error) {
	weights, totalWeight := deps.Tangle.WeightProvider.WeightsOfRelevantVoters()

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

// getMarkerVoters is the handler for the /weightprovider/marker/:sequenceID/:markerIndex/voters endpoint.
func getMarkerVoters(c echo.Context) (err error) {
	marker, err := markerFromContext(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels.NewErrorResponse(err))
	}

	voters := tangle.NewVoters()
	voters.AddAll(deps.Tangle.ApprovalWeightManager.VotersOfMarker(marker))

	return c.JSON(http.StatusOK, jsonmodels.NewMarkerVotersResponse(marker, voters))
}

// getMarkerWeight is the handler for the /weightprovider/marker/:sequenceID/:markerIndex/weight endpoint.
func getMarkerWeight(c echo.Context) (err error) {
	marker, err := markerFromContext(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels.NewErrorResponse(err))
	}

	return c.JSON(http.StatusOK, jsonmodels.MarkerWeightResponse{
		MarkerIndex:    marker.Index().String(),
		MarkerSequence: marker.SequenceID().String(),
		Weight:         deps.Tangle.ApprovalWeightManager.WeightOfMarker(marker),
	})
}

func markerFromContext(c echo.Context) (marker markers.Marker, err error) {
	sequenceIDString := c.Param("sequenceID")
	markerIndexString := c.Param("markerIndex")
	sequenceID, err := strconv.Atoi(sequenceIDString)
	if err != nil {
		return
	}
	markerIndex, err := strconv.Atoi(markerIndexString)
	if err != nil {
		return
	}

	return markers.NewMarker(markers.SequenceID(uint64(sequenceID)), markers.Index(uint64(markerIndex))), nil
}

// Weights defines the weights associated to the nodes.
type Weights struct {
	Weights     map[string]float64 `json:"weights"`
	TotalWeight float64            `json:"totalWeight"`
}
