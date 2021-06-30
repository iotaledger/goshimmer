package client

import (
	"net/http"
)

const (
	routeHealth = "healthz"
)

// HealthCheck checks whether the node is running and healthy.
func (api *GoShimmerAPI) HealthCheck() error {
	return api.do(http.MethodGet, routeHealth, nil, nil)
}
