package webapi

import (
	"net/http"

	"github.com/labstack/echo"
)

// IndexRequest returns INDEX
func IndexRequest(c echo.Context) error {
	return c.String(http.StatusOK, "INDEX")
}
