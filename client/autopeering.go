package client

import (
	"fmt"
	"net/http"

	"github.com/iotaledger/goshimmer/plugins/webapi"
)

const (
	routeGetNeighbors = "autopeering/neighbors"
)

// GetNeighbors gets the chosen/accepted neighbors.
// If knownPeers is set, also all known peers to the node are returned additionally.
func (api *GoShimmerAPI) GetNeighbors(knownPeers bool) (*webapi.AutopeeringResponse, error) {
	res := &webapi.AutopeeringResponse{}
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
