package spammer

import (
	"net/http"
	"time"

	"github.com/labstack/echo"

	"github.com/iotaledger/goshimmer/packages/jsonmodels"
)

func handleRequest(c echo.Context) error {
	var request jsonmodels.SpammerRequest
	if err := c.Bind(&request); err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels.SpammerResponse{Error: err.Error()})
	}

	switch request.Cmd {
	case "start":
		if request.Rate == 0 {
			log.Infof("Requesting invalid spamming at rate 0 mps. Setting it to 1 mps")
			request.Rate = 1
		}

		// IMIF: Inter Message Issuing Function
		switch request.IMIF {
		case "poisson":
			break
		default:
			request.IMIF = "uniform"
		}

		var timeUnit time.Duration
		switch request.Unit {
		case "mpm":
			timeUnit = time.Minute
		default:
			request.Unit = "mps"
			timeUnit = time.Second
		}

		messageSpammer.Shutdown()
		messageSpammer.Start(request.Rate, timeUnit, request.IMIF)
		log.Infof("Started spamming messages with %d %s and %s inter-message issuing function", request.Rate, request.Unit, request.IMIF)
		return c.JSON(http.StatusOK, jsonmodels.SpammerResponse{Message: "started spamming messages"})
	case "stop":
		messageSpammer.Shutdown()
		log.Info("Stopped spamming messages")
		return c.JSON(http.StatusOK, jsonmodels.SpammerResponse{Message: "stopped spamming messages"})
	default:
		return c.JSON(http.StatusBadRequest, jsonmodels.SpammerResponse{Error: "invalid cmd in request"})
	}
}
