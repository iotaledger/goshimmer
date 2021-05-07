package spammer

import (
	"net/http"
	"time"

	"github.com/labstack/echo"

	"github.com/iotaledger/goshimmer/plugins/webapi/jsonmodels"
)

func handleRequest(c echo.Context) error {
	var request jsonmodels.SpammerRequest
	if err := c.Bind(&request); err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels.SpammerResponse{Error: err.Error()})
	}

	switch request.Cmd {
	case "start":
		if request.MPM == 0 {
			request.MPM = 1
		}

		// IMIF: Inter Message Issuing Function
		switch request.IMIF {
		case "exponential":
			break
		default:
			request.IMIF = "uniform"
		}

		messageSpammer.Shutdown()
		messageSpammer.Start(request.MPM, time.Minute, request.IMIF)
		log.Infof("Started spamming messages with %d MPM and %s inter-message issuing function", request.MPM, request.IMIF)
		return c.JSON(http.StatusOK, jsonmodels.SpammerResponse{Message: "started spamming messages"})
	case "stop":
		messageSpammer.Shutdown()
		log.Info("Stopped spamming messages")
		return c.JSON(http.StatusOK, jsonmodels.SpammerResponse{Message: "stopped spamming messages"})
	default:
		return c.JSON(http.StatusBadRequest, jsonmodels.SpammerResponse{Error: "invalid cmd in request"})
	}
}
