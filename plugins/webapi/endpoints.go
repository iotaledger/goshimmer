package webapi

import (
	"net/http"

	"github.com/labstack/echo"
)

const (
	// AutopeeringRoot is the root of the autopeering route
	AutopeeringRoot = "autopeering"
	// DataRoot is the root of the data route
	DataRoot = "data"
	// FaucetRoot is the root of the faucet route
	FaucetRoot = "faucet"
	// HealthzRoot is the root of the healthz route
	HealthzRoot = "healthz"
	// InfoRoot is the root of the info route
	InfoRoot = "info"
	// DrngRoot is the root of the drng route
	DrngRoot = "drng"
	// MessageRoot is the root of the message route
	MessageRoot = "message"
	// ToolsRoot is the root of the tools route
	ToolsRoot = "tools"
	// ValueRoot is the root of the value route
	ValueRoot = "value"
)

// IndexRequest returns INDEX
func IndexRequest(c echo.Context) error {
	return c.String(http.StatusOK, "INDEX")
}
