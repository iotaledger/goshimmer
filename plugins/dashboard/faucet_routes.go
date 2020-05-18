package dashboard

import (
	"net/http"

	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/address"
	faucetpayload "github.com/iotaledger/goshimmer/packages/binary/faucet/payload"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"

	"github.com/labstack/echo"
	"github.com/pkg/errors"
)

type ReqMsg struct {
	Id string `json:"MsgId"`
}

func setupFaucetRoutes(routeGroup *echo.Group) {
	routeGroup.GET("/faucet/:hash", func(c echo.Context) (err error) {
		addr := c.Param("hash")
		// TODO: check if the address is valid
		if len(addr) < address.Length {
			return errors.Wrapf(ErrInvalidParameter, "search address invalid: %s", addr)
		}

		t, err := sendFaucetReq(addr)
		if err != nil {
			return
		}

		return c.JSON(http.StatusOK, t)
	})
}

func sendFaucetReq(address string) (res *ReqMsg, err error) {
	msg := messagelayer.MessageFactory.IssuePayload(faucetpayload.New([]byte(address)))
	if msg == nil {
		return nil, errors.Wrapf(ErrInternalError, "Fail to send faucet request")
	}

	r := &ReqMsg{
		Id: msg.Id().String(),
	}
	return r, nil
}
