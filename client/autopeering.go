package client

import (
	"fmt"
	"net/http"

	"github.com/iotaledger/goshimmer/packages/jsonmodels"
)

const (
	routeGetAutopeeringNeighbors = "autopeering/neighbors"
)

// GetAutopeeringNeighbors gets the chosen/accepted neighbors.
// If knownPeers is set, also all known peers to the node are returned additionally.
func (api *GoShimmerAPI) GetAutopeeringNeighbors(knownPeers bool) (*jsonmodels.GetNeighborsResponse, error) {
	res := &jsonmodels.GetNeighborsResponse{}
	if err := api.do(http.MethodGet, func() string {
		if !knownPeers {
			return routeGetAutopeeringNeighbors
		}
		return fmt.Sprintf("%s?known=1", routeGetAutopeeringNeighbors)
	}(), nil, res); err != nil {
		return nil, err
	}
	return res, nil
}
