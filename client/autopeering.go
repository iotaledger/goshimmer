package client

import (
	"fmt"
	"net/http"

	"github.com/iotaledger/goshimmer/plugins/webapi/jsonmodels"
)

const (
	routeGetNeighbors = "autopeering/neighbors"
)

// GetNeighbors gets the chosen/accepted neighbors.
// If knownPeers is set, also all known peers to the node are returned additionally.
func (api *GoShimmerAPI) GetNeighbors(knownPeers bool) (*jsonmodels.GetNeighborsResponse, error) {
	res := &jsonmodels.GetNeighborsResponse{}
	if err := api.do(http.MethodGet, func() string {
		if !knownPeers {
			return routeGetNeighbors
		}
		return fmt.Sprintf("%s?known=1", routeGetNeighbors)
	}(), nil, res); err != nil {
		return nil, err
	}
	return res, nil
}
