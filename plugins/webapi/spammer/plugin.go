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
		return c.JSON(http.StatusBadRequest, Response{Error: err.Error()})
	}

	switch request.Cmd {
	case "start":
		if request.Tps == 0 {
			request.Tps = 1
		}

		transactionspammer.Stop()
		transactionspammer.Start(request.Tps)
		return c.JSON(http.StatusOK, Response{Message: "started spamming transactions"})
	case "stop":
		transactionspammer.Stop()
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
	Tps uint64 `json:"tps"`
}
