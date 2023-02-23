package dashboard

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

type TipsResponse struct {
	Tips []string `json:"tips"`
}

func setupTipsRoutes(routeGroup *echo.Group) {
	routeGroup.GET("/tips", func(c echo.Context) (err error) {
		return c.JSON(http.StatusOK, tips())
	})
}

func tips() *TipsResponse {
	allTips := deps.Protocol.TipManager.AllTips()
	t := make([]string, len(allTips))

	for i, tip := range allTips {
		t[i] = tip.ID().Base58()
	}

	return &TipsResponse{Tips: t}
}
