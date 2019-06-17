package webapi

import (
	"net/http"

	"github.com/labstack/echo"
)

func IndexRequest(c echo.Context) error {
	return c.String(http.StatusOK, "INDEX")
}
