package snapshot

import (
	"net/http"

	"go.uber.org/dig"

	"github.com/iotaledger/hive.go/node"
	"github.com/labstack/echo"

	"github.com/iotaledger/goshimmer/packages/app/jsonmodels"
	"github.com/iotaledger/goshimmer/packages/core/notarization"
	"github.com/iotaledger/goshimmer/packages/core/tangleold"

	"github.com/iotaledger/goshimmer/packages/core/snapshot"
)

// region Plugin ///////////////////////////////////////////////////////////////////////////////////////////////////////

const (
	snapshotFileName = "snapshot.bin"
)

type dependencies struct {
	dig.In

	Server          *echo.Echo
	Tangle          *tangleold.Tangle
	NotarizationMgr *notarization.Manager
}

var (
	// Plugin holds the singleton instance of the Plugin.
	Plugin *node.Plugin

	deps = new(dependencies)
)

func init() {
	Plugin = node.NewPlugin("Snapshot", deps, node.Disabled, configure)
}

func configure(_ *node.Plugin) {
	deps.Server.GET("snapshot", DumpCurrentLedger)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region DumpCurrentLedger ///////////////////////////////////////////////////////////////////////////////////////////////////

// DumpCurrentLedger dumps a snapshot (all unspent UTXO and all of the access mana) from now.
func DumpCurrentLedger(c echo.Context) (err error) {
	nodeSnapshot := new(snapshot.Snapshot)
	err = nodeSnapshot.CreateStreamableSnapshot(snapshotFileName, deps.Tangle, deps.NotarizationMgr)
	if err != nil {
		Plugin.LogErrorf("unable to get snapshot bytes %s", err)
		return c.JSON(http.StatusInternalServerError, jsonmodels.NewErrorResponse(err))
	}

	return c.Attachment(snapshotFileName, snapshotFileName)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
