package dashboard

import (
	"net/http"
	"sync"

	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/address"
	"github.com/iotaledger/goshimmer/plugins/config"
	"github.com/iotaledger/goshimmer/plugins/faucet"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"

	"github.com/labstack/echo"
	"github.com/pkg/errors"
)

// ReqMsg defines the struct of the faucet request message ID.
type ReqMsg struct {
	ID string `json:"MsgId"`
}

func setupFaucetRoutes(routeGroup *echo.Group) {
	routeGroup.GET("/faucet/:hash", func(c echo.Context) (err error) {
		addr, err := address.FromBase58(c.Param("hash"))
		if err != nil {
			return errors.Wrapf(ErrInvalidParameter, "faucet request address invalid: %s", addr)
		}

		t, err := sendFaucetReq(addr)
		if err != nil {
			return
		}

		return c.JSON(http.StatusOK, t)
	})
}

var fundingReqMu = sync.Mutex{}

func sendFaucetReq(addr address.Address) (res *ReqMsg, err error) {
	fundingReqMu.Lock()
	defer fundingReqMu.Unlock()
	faucetPayload, err := faucet.NewRequest(addr, config.Node().Int(faucet.CfgFaucetPoWDifficulty))
	if err != nil {
		return nil, err
	}
	msg, err := messagelayer.Tangle().MessageFactory.IssuePayload(faucetPayload, messagelayer.Tangle())
	if err != nil {
		return nil, errors.Wrapf(ErrInternalError, "Failed to send faucet request: %s", err.Error())
	}

	r := &ReqMsg{
		ID: msg.ID().String(),
	}
	return r, nil
}
