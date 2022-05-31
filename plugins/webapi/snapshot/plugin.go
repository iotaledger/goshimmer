package snapshot

import (
	"os"

	"github.com/cockroachdb/errors"
	"go.uber.org/dig"

	"github.com/iotaledger/goshimmer/packages/mana"
	"github.com/iotaledger/goshimmer/packages/snapshot"
	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"

	"github.com/iotaledger/hive.go/node"
	"github.com/labstack/echo"
)

// region Plugin ///////////////////////////////////////////////////////////////////////////////////////////////////////

const (
	snapshotFileName = "snapshot.bin"
)

type dependencies struct {
	dig.In

	Server *echo.Echo
	Tangle *tangle.Tangle
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
	accessManaMap, accessManaTime, err := messagelayer.GetManaMap(mana.AccessMana)
	if err != nil {
		return errors.Errorf("could not get access mana map: %w", err)
	}

	nodeSnapshot := new(snapshot.Snapshot)
	nodeSnapshot.FromNode(deps.Tangle.Ledger, accessManaMap, accessManaTime)

	snapshotBytes := nodeSnapshot.Bytes()
	if err = os.WriteFile(snapshotFileName, snapshotBytes, 0o666); err != nil {
		Plugin.LogErrorf("unable to create snapshot file %s", err)
	}

	Plugin.LogInfo("Snapshot information: ")
	Plugin.LogInfo("     Number of outputs: ", nodeSnapshot.LedgerSnapshot.Outputs.Size())
	Plugin.LogInfo("     Number of accessManaEntries: ", len(nodeSnapshot.ManaSnapshot.ByNodeID))

	Plugin.LogInfof("Bytes written %d", len(snapshotBytes))

	return c.Attachment(snapshotFileName, snapshotFileName)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
