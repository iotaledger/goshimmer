package dashboard

import (
	"net/http"
	"sync"

	"golang.org/x/xerrors"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"

	"github.com/iotaledger/goshimmer/plugins/config"
	"github.com/iotaledger/goshimmer/plugins/faucet"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"

	"github.com/labstack/echo"
)

// ReqMsg defines the struct of the faucet request message ID.
type ReqMsg struct {
	ID string `json:"MsgId"`
}

func setupFaucetRoutes(routeGroup *echo.Group) {
	routeGroup.GET("/faucet/:hash", func(c echo.Context) (err error) {
		addr, err := ledgerstate.AddressFromBase58EncodedString(c.Param("hash"))
		if err != nil {
			return xerrors.Errorf("faucet request address invalid: %s: %w", addr, ErrInvalidParameter)
		}

		t, err := sendFaucetReq(addr)
		if err != nil {
			return
		}

		return c.JSON(http.StatusOK, t)
	})
}

var fundingReqMu = sync.Mutex{}

func sendFaucetReq(addr ledgerstate.Address) (res *ReqMsg, err error) {
	fundingReqMu.Lock()
	defer fundingReqMu.Unlock()
	faucetPayload, err := faucet.NewRequest(addr, config.Node().Int(faucet.CfgFaucetPoWDifficulty))
	if err != nil {
		return nil, err
	}
	msg, err := messagelayer.Tangle().MessageFactory.IssuePayload(faucetPayload, messagelayer.Tangle())
	if err != nil {
		return nil, xerrors.Errorf("Failed to send faucet request: %s: %w", err.Error(), ErrInternalError)
	}

	r := &ReqMsg{
		ID: msg.ID().String(),
	}
	return r, nil
}
