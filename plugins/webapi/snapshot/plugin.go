package snapshot

import (
	"os"

	"go.uber.org/dig"

	"github.com/iotaledger/goshimmer/packages/mana"
	"github.com/iotaledger/goshimmer/packages/snapshot"
	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"

	"github.com/iotaledger/hive.go/identity"
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
	nodeSnapshot := new(snapshot.Snapshot)
	nodeSnapshot.FromNode(deps.Tangle.Ledger)

	snapshotBytes := nodeSnapshot.Bytes()
	if err = os.WriteFile(snapshotFileName, snapshotBytes, 0o666); err != nil {
		Plugin.LogErrorf("unable to create snapshot file %s", err)
	}

	Plugin.LogInfo("Snapshot information: ")
	Plugin.LogInfo("     Number of snapshotted outputs: ", len(nodeSnapshot.LedgerSnapshot.Outputs()))
	Plugin.LogInfo("     Number of snapshotted accessManaEntries: ", len(nodeSnapshot.ManaSnapshot.ByNodeID))

	Plugin.LogInfof("Bytes written %d", len(snapshotBytes))

	return c.Attachment(snapshotFileName, snapshotFileName)
}

// snapshotAccessMana returns snapshot of the current access mana.
func snapshotAccessMana() (aManaSnapshot map[identity.ID]mana.AccessManaRecord, err error) {
	aManaSnapshot = make(map[identity.ID]mana.AccessManaRecord)

	m, t, err := messagelayer.GetManaMap(mana.AccessMana)
	if err != nil {
		return nil, err
	}
	for nodeID, aMana := range m {
		aManaSnapshot[nodeID] = mana.AccessManaRecord{
			Value:     aMana,
			Timestamp: t,
		}
	}
	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
