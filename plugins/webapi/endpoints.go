package webapi

import (
	"net/http"

	"github.com/labstack/echo"
)

const (
	DrngRoot    = "drng"
	MessageRoot = "message"
	ToolsRoot   = "tools"
	ValueRoot   = "value"
)

// IndexRequest returns INDEX
func IndexRequest(c echo.Context) error {
	return c.String(http.StatusOK, "INDEX")
}
