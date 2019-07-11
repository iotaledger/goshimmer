package webapi_spammer

import (
	"net/http"
	"time"

	"github.com/iotaledger/goshimmer/packages/node"
	"github.com/iotaledger/goshimmer/packages/transactionspammer"
	"github.com/iotaledger/goshimmer/plugins/webapi"
	"github.com/labstack/echo"
)

var PLUGIN = node.NewPlugin("Spammer", configure)

func configure(plugin *node.Plugin) {
	webapi.AddEndpoint("spammer", WebApiHandler)
}

func WebApiHandler(c echo.Context) error {
	c.Set("requestStartTime", time.Now())

	var request webRequest
	if err := c.Bind(&request); err != nil {
		return requestFailed(c, err.Error())
	}

	switch request.Cmd {
	case "start":
		if request.Tps == 0 {
			request.Tps = 1000
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
	return c.JSON(http.StatusOK, webResponse{
		Duration: time.Since(c.Get("requestStartTime").(time.Time)).Nanoseconds() / 1e6,
		Status:   "success",
		Message:  message,
	})
}

func requestFailed(c echo.Context, message string) error {
	return c.JSON(http.StatusOK, webResponse{
		Duration: time.Since(c.Get("requestStartTime").(time.Time)).Nanoseconds() / 1e6,
		Status:   "failed",
		Message:  message,
	})
}

type webResponse struct {
	Duration int64  `json:"duration"`
	Status   string `json:"status"`
	Message  string `json:"message"`
}

type webRequest struct {
	Cmd string `json:"cmd"`
	Tps uint   `json:"tps"`
}
