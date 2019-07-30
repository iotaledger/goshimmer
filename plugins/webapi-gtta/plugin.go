package webapi_gtta

import (
	"net/http"
	"time"

	"github.com/iotaledger/goshimmer/packages/node"
	"github.com/iotaledger/goshimmer/plugins/tipselection"
	"github.com/iotaledger/goshimmer/plugins/webapi"
	"github.com/iotaledger/iota.go/trinary"
	"github.com/labstack/echo"
)

var PLUGIN = node.NewPlugin("WebAPI GTTA Endpoint", func(plugin *node.Plugin) {
	webapi.AddEndpoint("getTransactionsToApprove", Handler)
})

func Handler(c echo.Context) error {
	start := time.Now()

	branchTransactionHash := tipselection.GetRandomTip()
	trunkTransactionHash := tipselection.GetRandomTip()

	return c.JSON(http.StatusOK, webResponse{
		Duration:          time.Since(start).Nanoseconds() / 1e6,
		BranchTransaction: branchTransactionHash,
		TrunkTransaction:  trunkTransactionHash,
	})
}

type webResponse struct {
	Duration          int64          `json:"duration"`
	BranchTransaction trinary.Trytes `json:"branchTransaction"`
	TrunkTransaction  trinary.Trytes `json:"trunkTransaction"`
}
