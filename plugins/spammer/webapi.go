package spammer

import (
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/iotaledger/goshimmer/packages/app/jsonmodels"
)

func handleRequest(c echo.Context) error {
	var request jsonmodels.SpammerRequest
	if err := c.Bind(&request); err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels.SpammerResponse{Error: err.Error()})
	}

	switch request.Cmd {
	case "start":
		if request.Rate == 0 {
			log.Infof("Requesting invalid spamming at rate 0 BPS. Setting it to 1 BPS")
			request.Rate = 1
		}

		// IMIF: Inter Block Issuing Function
		switch request.IMIF {
		case "poisson":
			break
		default:
			request.IMIF = "uniform"
		}

		var timeUnit time.Duration
		switch request.Unit {
		case "BPM":
			timeUnit = time.Minute
		default:
			request.Unit = "BPS"
			timeUnit = time.Second
		}

		// Default payload size set to 5 bytes.
		if request.PayloadSize == 0 {
			request.PayloadSize = 5
		}

		blockSpammer.Shutdown()
		blockSpammer.Start(request.Rate, request.PayloadSize, timeUnit, request.IMIF)
		log.Infof("Started spamming blocks with %d %s and %s inter-block issuing function", request.Rate, strings.ReplaceAll(request.Unit, "\n", ""), strings.ReplaceAll(request.IMIF, "\n", ""))
		return c.JSON(http.StatusOK, jsonmodels.SpammerResponse{Block: "started spamming blocks"})
	case "stop":
		blockSpammer.Shutdown()
		log.Info("Stopped spamming blocks")
		return c.JSON(http.StatusOK, jsonmodels.SpammerResponse{Block: "stopped spamming blocks"})
	default:
		return c.JSON(http.StatusBadRequest, jsonmodels.SpammerResponse{Error: "invalid cmd in request"})
	}
}
