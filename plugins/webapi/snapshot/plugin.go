package snapshot

import (
	"net/http"
	"os"
	"path/filepath"

	"github.com/labstack/echo/v4"
	"go.uber.org/dig"

	"github.com/iotaledger/goshimmer/packages/app/jsonmodels"
	"github.com/iotaledger/goshimmer/packages/node"
	"github.com/iotaledger/goshimmer/packages/protocol"
	"github.com/iotaledger/hive.go/core/slot"
)

// region Plugin ///////////////////////////////////////////////////////////////////////////////////////////////////////

type dependencies struct {
	dig.In

	Server   *echo.Echo
	Protocol *protocol.Protocol
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
	deps.Server.GET("snapshot", GetSnapshot)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region DumpCurrentLedger ///////////////////////////////////////////////////////////////////////////////////////////////////

// GetSnapshot dumps a snapshot (all unspent UTXO and all of the access mana) from now.
func GetSnapshot(c echo.Context) (err error) {
	var request jsonmodels.GetSnapshotRequest
	if err = c.Bind(&request); err != nil {
		Plugin.LogInfo(err.Error())
		return c.JSON(http.StatusBadRequest, jsonmodels.NewErrorResponse(err))
	}
	snapshotFilePath := filepath.Join(os.TempDir(), "snapshot.bin")
	if c.QueryParam("index") == "" {
		err = deps.Protocol.Engine().WriteSnapshot(snapshotFilePath)
	} else {
		err = deps.Protocol.Engine().WriteSnapshot(snapshotFilePath, slot.Index(request.SlotIndex))
	}
	if err != nil {
		Plugin.LogErrorf("unable to get snapshot bytes %s", err)
		return c.JSON(http.StatusInternalServerError, jsonmodels.NewErrorResponse(err))
	}

	return c.Attachment(snapshotFilePath, snapshotFilePath)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
