package dashboard

import (
	"net/http"

	faucetpayload "github.com/iotaledger/goshimmer/dapps/faucet/packages/payload"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/address"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"

	"github.com/labstack/echo"
	"github.com/pkg/errors"
)

type ReqMsg struct {
	Id string `json:"MsgId"`
}

func setupFaucetRoutes(routeGroup *echo.Group) {
	routeGroup.GET("/faucet/:hash", func(c echo.Context) (err error) {
		addr, err := address.FromBase58(c.Param("hash"))
		if err != nil {
			return errors.Wrapf(ErrInvalidParameter, "search address invalid: %s", addr)
		}

		t, err := sendFaucetReq(addr)
		if err != nil {
			return
		}

		return c.JSON(http.StatusOK, t)
	})
}

func sendFaucetReq(addr address.Address) (res *ReqMsg, err error) {
	msg := messagelayer.MessageFactory.IssuePayload(faucetpayload.New(addr))
	if msg == nil {
		return nil, errors.Wrapf(ErrInternalError, "Fail to send faucet request")
	}

	r := &ReqMsg{
		Id: msg.Id().String(),
	}
	return r, nil
}
