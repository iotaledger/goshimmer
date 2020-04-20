package spammer

import (
	"net/http"

	"github.com/labstack/echo"
)

func handleRequest(c echo.Context) error {
	var request Request
	if err := c.Bind(&request); err != nil {
		return c.JSON(http.StatusBadRequest, Response{Error: err.Error()})
	}

	switch request.Cmd {
	case "start":
		if request.MPS == 0 {
			request.MPS = 1
		}

		messageSpammer.Shutdown()
		messageSpammer.Start(request.MPS)
		return c.JSON(http.StatusOK, Response{Message: "started spamming messages"})
	case "stop":
		messageSpammer.Shutdown()
		return c.JSON(http.StatusOK, Response{Message: "stopped spamming messages"})
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
	MPS int    `json:"mps"`
}
