package dashboard

import (
	"net/http"
	"sync"

	"github.com/iotaledger/hive.go/identity"
	"github.com/labstack/echo"
	"golang.org/x/xerrors"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/mana"
	"github.com/iotaledger/goshimmer/plugins/config"
	"github.com/iotaledger/goshimmer/plugins/faucet"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
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

		emptyID := identity.ID{}
		accessMana := emptyID
		consensusMana := emptyID

		accessManaStr := c.QueryParam("accessMana")
		if accessManaStr != "" {
			accessMana, err = mana.IDFromStr(accessManaStr)
			if err != nil {
				return xerrors.Errorf("faucet request access mana node ID invalid: %s: %w", accessManaStr, err)
			}
		}

		consensusManaStr := c.QueryParam("consensusMana")
		if consensusManaStr != "" {
			consensusMana, err = mana.IDFromStr(consensusManaStr)
			if err != nil {
				return xerrors.Errorf("faucet request consensus mana node ID invalid: %s: %w", consensusManaStr, err)
			}
		}

		t, err := sendFaucetReq(addr, accessMana, consensusMana)
		if err != nil {
			return
		}

		return c.JSON(http.StatusOK, t)
	})
}

var fundingReqMu = sync.Mutex{}

func sendFaucetReq(addr ledgerstate.Address, accessManaNodeID, consensusManaNodeID identity.ID) (res *ReqMsg, err error) {
	fundingReqMu.Lock()
	defer fundingReqMu.Unlock()
	faucetPayload, err := faucet.NewRequest(addr, config.Node().Int(faucet.CfgFaucetPoWDifficulty), accessManaNodeID, consensusManaNodeID)
	if err != nil {
		return nil, err
	}
	msg, err := messagelayer.Tangle().MessageFactory.IssuePayload(faucetPayload, messagelayer.Tangle())
	if err != nil {
		return nil, xerrors.Errorf("Failed to send faucet request: %s: %w", err.Error(), ErrInternalError)
	}

	r := &ReqMsg{
		ID: msg.ID().Base58(),
	}
	return r, nil
}
