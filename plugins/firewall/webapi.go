package firewall

import (
	"net/http"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/identity"
	"github.com/labstack/echo"

	"github.com/iotaledger/goshimmer/packages/jsonmodels"
)

// RoutePeerFaultinessCount defines the HTTP path for firewall/is-peer-faulty endpoint.
const RoutePeerFaultinessCount = "firewall/peer-faultiness-count/:peerId"

func configureWebAPI() {
	deps.Server.GET(RoutePeerFaultinessCount, getPeerFaultinessCountHandler)
}

func getPeerFaultinessCountHandler(c echo.Context) error {
	peerID, err := identity.DecodeIDBase58(c.Param("peerId"))
	if err != nil {
		Plugin.Logger().Errorw("Failed to decode peer id from the URL",
			"err", err)
		return c.JSON(http.StatusBadRequest, jsonmodels.NewErrorResponse(errors.Wrap(err, "invalid peer id in the URL")))
	}
	count := deps.Firewall.GetPeerFaultinessCount(peerID)
	return c.JSON(http.StatusOK, count)
}
