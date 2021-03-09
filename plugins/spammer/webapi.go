package spammer

import (
	"net/http"
	"time"

	"github.com/labstack/echo"
)

func handleRequest(c echo.Context) error {
	var request Request
	if err := c.Bind(&request); err != nil {
		return c.JSON(http.StatusBadRequest, Response{Error: err.Error()})
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
		return c.JSON(http.StatusOK, Response{Message: "started spamming messages"})
	case "stop":
		messageSpammer.Shutdown()
		log.Info("Stopped spamming messages")
		return c.JSON(http.StatusOK, Response{Message: "stopped spamming messages"})
	default:
		return c.JSON(http.StatusBadRequest, Response{Error: "invalid cmd in request"})
	}
}

// Response is the HTTP response of a spammer request.
type Response struct {
	Message string `json:"message"`
	Error   string `json:"error"`
}

// Request contains the parameters of a spammer request.
type Request struct {
	Cmd  string `json:"cmd"`
	IMIF string `json:"imif"`
	MPM  int    `json:"mpm"`
}
