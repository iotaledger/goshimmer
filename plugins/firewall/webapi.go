package firewall

import (
	"net/http"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/identity"
	"github.com/labstack/echo"

	"github.com/iotaledger/goshimmer/packages/jsonmodels"
)

// RouteIsFaulty defines the HTTP path for firewall/is-peer-faulty endpoint.
const RouteIsFaulty = "firewall/is-peer-faulty/:peerId"

func configureWebAPI() {
	deps.Server.GET(RouteIsFaulty, isFaultyHandler)
}

func isFaultyHandler(c echo.Context) error {
	peerID, err := identity.DecodeIDBase58(c.Param("peerId"))
	if err != nil {
		Plugin.Logger().Errorw("Failed to decode peer id from the URL",
			"err", err)
		return c.JSON(http.StatusBadRequest, jsonmodels.NewErrorResponse(errors.Wrap(err, "invalid peer id in the URL")))
	}
	faulty := deps.Firewall.IsFaulty(peerID)
	return c.JSON(http.StatusOK, faulty)
}
