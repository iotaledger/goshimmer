package spammer

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo"
)

func handleRequest(c echo.Context) error {
	var request Request
	if err := c.Bind(&request); err != nil {
		return c.JSON(http.StatusBadRequest, Response{Error: err.Error()})
	}

	switch request.Cmd {
	case "start":
		if request.Tps == 0 {
			request.Tps = 1
		}

		transactionSpammer.Shutdown()
		transactionSpammer.Start(request.Tps)
		return c.JSON(http.StatusOK, Response{Message: "started spamming transactions"})
	case "burst":
		if request.Tps == 0 {
			return c.JSON(http.StatusBadRequest, Response{Error: "burst requires the tps to be set"})
		}

		transactionSpammer.Shutdown()
		transactionSpammer.Burst(request.Tps)

		return c.JSON(http.StatusOK, Response{Message: "sent a burst of " + strconv.Itoa(request.Tps) + " transactions"})
	case "stop":
		transactionSpammer.Shutdown()
		return c.JSON(http.StatusOK, Response{Message: "stopped spamming transactions"})
	default:
		return c.JSON(http.StatusBadRequest, Response{Error: "invalid cmd in request"})
	}
}

type Response struct {
	Message string `json:"message"`
	Error   string `json:"error"`
}

type Request struct {
	Cmd string `json:"cmd"`
	Tps int    `json:"tps"`
}
