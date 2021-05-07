package dashboard

import (
	"github.com/labstack/echo"

	epochsAPI "github.com/iotaledger/goshimmer/plugins/webapi/epochs"
)

func setupEpochRoutes(routeGroup *echo.Group) {
	routeGroup.GET("/epochs/:epochID", epochsAPI.GetEpoch)
	routeGroup.GET("/epochs/oracle/current", epochsAPI.GetOracleEpochID)
}
