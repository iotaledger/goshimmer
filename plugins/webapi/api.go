package webapi

import (
	"github.com/labstack/echo"
)

func AddEndpoint(url string, handler func(c echo.Context) error) {
	Server.GET(url, handler)
}
