package gtta

import (
	"net/http"

	"github.com/iotaledger/goshimmer/plugins/tipselection"
	"github.com/iotaledger/goshimmer/plugins/webapi"
	"github.com/iotaledger/hive.go/node"
	"github.com/iotaledger/iota.go/trinary"
	"github.com/labstack/echo"
)

var PLUGIN = node.NewPlugin("WebAPI GTTA Endpoint", node.Disabled, func(plugin *node.Plugin) {
	webapi.Server.GET("getTransactionsToApprove", Handler)
})

func Handler(c echo.Context) error {

	branchTransactionHash := tipselection.GetRandomTip()
	trunkTransactionHash := tipselection.GetRandomTip()

	return c.JSON(http.StatusOK, Response{
		BranchTransaction: branchTransactionHash,
		TrunkTransaction:  trunkTransactionHash,
	})
}

type Response struct {
	BranchTransaction trinary.Trytes `json:"branchTransaction"`
	TrunkTransaction  trinary.Trytes `json:"trunkTransaction"`
}
