package webapi

import (
	"net/http"

	"github.com/iotaledger/goshimmer/plugins/gossip"
	"github.com/iotaledger/goshimmer/plugins/syncbeaconfollower"
	"github.com/labstack/echo"
)

func init() {
	Server().GET("healthz", getHealthz)
}

func getHealthz(c echo.Context) error {
	if _, exists := DisabledAPIs[HealthzRoot]; exists {
		return c.NoContent(http.StatusForbidden)
	}

	if !IsNodeHealthy() {
		return c.NoContent(http.StatusServiceUnavailable)
	}

	return c.NoContent(http.StatusOK)
}

// IsNodeHealthy returns whether the node is synced, has active neighbors.
func IsNodeHealthy() bool {
	// Synced
	if !syncbeaconfollower.Synced() {
		return false
	}

	// Has connected neighbors
	if len(gossip.Manager().AllNeighbors()) == 0 {
		return false
	}

	return true
}
