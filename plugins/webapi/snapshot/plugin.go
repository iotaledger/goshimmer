package snapshot

import (
	"net/http"

	"go.uber.org/dig"

	"github.com/iotaledger/hive.go/node"
	"github.com/labstack/echo"

	"github.com/iotaledger/goshimmer/packages/jsonmodels"
	"github.com/iotaledger/goshimmer/packages/notarization"
	"github.com/iotaledger/goshimmer/packages/snapshot"
	"github.com/iotaledger/goshimmer/packages/tangle"
)

// region Plugin ///////////////////////////////////////////////////////////////////////////////////////////////////////

const (
	snapshotFileName = "snapshot.bin"
)

type dependencies struct {
	dig.In

	Server          *echo.Echo
	Tangle          *tangle.Tangle
	NotarizationMgr *notarization.Manager
}

var (
	// Plugin holds the singleton instance of the Plugin.
	Plugin *node.Plugin

	deps = new(dependencies)
)

func init() {
	Plugin = node.NewPlugin("Snapshot", deps, node.Enabled, configure)
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
	//nodeSnapshot.FromNode(deps.Tangle.Ledger)

	// snapshotBytes, err := nodeSnapshot.Bytes()
	// if err != nil {
	// 	Plugin.LogErrorf("unable to get snapshot bytes %s", err)
	// 	return c.JSON(http.StatusInternalServerError, jsonmodels.NewErrorResponse(err))
	// }
	// if err = os.WriteFile(snapshotFileName, snapshotBytes, 0o666); err != nil {
	// 	Plugin.LogErrorf("unable to create snapshot file %s", err)
	// }

	// Plugin.LogInfo("Snapshot information: ")
	// Plugin.LogInfo("     Number of outputs: ", len(nodeSnapshot.LedgerSnapshot.OutputsWithMetadata))
	// Plugin.LogInfo("     Number of epochdiffs: ", len(nodeSnapshot.LedgerSnapshot.EpochDiffs))

	// Plugin.LogInfof("Bytes written %d", len(snapshotBytes))

	return c.Attachment(snapshotFileName, snapshotFileName)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
