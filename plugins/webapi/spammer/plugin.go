package spammer

import (
	"net/http"

	"github.com/iotaledger/goshimmer/packages/transactionspammer"
	"github.com/iotaledger/goshimmer/plugins/webapi"
	"github.com/iotaledger/hive.go/node"
	"github.com/labstack/echo"
)

var PLUGIN = node.NewPlugin("Spammer", node.Disabled, configure)

func configure(plugin *node.Plugin) {
	webapi.Server.GET("spammer", WebApiHandler)
}

func WebApiHandler(c echo.Context) error {

	var request Request
	if err := c.Bind(&request); err != nil {
		return requestFailed(c, err.Error())
	}

	switch request.Cmd {
	case "start":
		if request.Tps == 0 {
			request.Tps = 1
		}

		transactionspammer.Stop()
		transactionspammer.Start(request.Tps)

		return requestSuccessful(c, "started spamming transactions")

	case "stop":
		transactionspammer.Stop()

		return requestSuccessful(c, "stopped spamming transactions")

	default:
		return requestFailed(c, "invalid cmd in request")
	}
}

func requestSuccessful(c echo.Context, message string) error {
	return c.JSON(http.StatusOK, Response{
		Message: message,
	})
}

func requestFailed(c echo.Context, message string) error {
	return c.JSON(http.StatusNotFound, Response{
		Message: message,
	})
}

type Response struct {
	Message string `json:"message"`
}

type Request struct {
	Cmd string `json:"cmd"`
	Tps uint   `json:"tps"`
}
