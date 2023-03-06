package webapi

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

// IndexRequest returns INDEX
func IndexRequest(c echo.Context) error {
	return c.String(http.StatusOK, "INDEX")
}
