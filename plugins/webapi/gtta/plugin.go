package gtta

import (
	"net/http"

	"github.com/iotaledger/goshimmer/packages/binary/tangle/model/message"
	"github.com/iotaledger/goshimmer/plugins/tangle"
	"github.com/iotaledger/goshimmer/plugins/webapi"
	"github.com/iotaledger/hive.go/node"
	"github.com/labstack/echo"
)

var PLUGIN = node.NewPlugin("WebAPI GTTA Endpoint", node.Disabled, func(plugin *node.Plugin) {
	webapi.Server.GET("getTransactionsToApprove", Handler)
})

func Handler(c echo.Context) error {
	trunkTransactionId, branchTransactionId := tangle.TipSelector.GetTips()

	return c.JSON(http.StatusOK, Response{
		TrunkTransaction:  trunkTransactionId,
		BranchTransaction: branchTransactionId,
	})
}

type Response struct {
	BranchTransaction message.Id `json:"branchTransaction"`
	TrunkTransaction  message.Id `json:"trunkTransaction"`
}
