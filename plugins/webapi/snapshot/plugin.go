package snapshot

import (
	"net/http"

	"go.uber.org/dig"

	"github.com/iotaledger/hive.go/core/node"
	"github.com/labstack/echo"

	"github.com/iotaledger/goshimmer/packages/app/jsonmodels"
	"github.com/iotaledger/goshimmer/packages/core/snapshot"
)

// region Plugin ///////////////////////////////////////////////////////////////////////////////////////////////////////

const (
	snapshotFileName = "/tmp/snapshot.bin"
)

type dependencies struct {
	dig.In

	Server      *echo.Echo
	SnapshotMgr *snapshot.Manager
}

var (
	// Plugin holds the singleton instance of the Plugin.
	Plugin *node.Plugin

	deps = new(dependencies)
)

func init() {
	Plugin = node.NewPlugin("WebAPISnapshot", deps, node.Disabled, configure)
}

func configure(_ *node.Plugin) {
	deps.Server.GET("snapshot", DumpCurrentLedger)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region DumpCurrentLedger ///////////////////////////////////////////////////////////////////////////////////////////////////

// DumpCurrentLedger dumps a snapshot (all unspent UTXO and all of the access mana) from now.
func DumpCurrentLedger(c echo.Context) (err error) {
	header, err := deps.SnapshotMgr.CreateSnapshot(snapshotFileName)
	if err != nil {
		Plugin.LogErrorf("unable to get snapshot bytes %s", err)
		return c.JSON(http.StatusInternalServerError, jsonmodels.NewErrorResponse(err))
	}
	Plugin.LogInfo("Snapshot information: ")
	Plugin.LogInfo("     Number of outputs: ", header.OutputWithMetadataCount)
	Plugin.LogInfo("     FullEpochIndex: ", header.FullEpochIndex)
	Plugin.LogInfo("     DiffEpochIndex: ", header.DiffEpochIndex)
	Plugin.LogInfo("     LatestECRecord: ", header.LatestECRecord)

	return c.Attachment(snapshotFileName, snapshotFileName)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
