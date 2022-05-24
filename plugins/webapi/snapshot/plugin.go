package snapshot

import (
	"os"

	"go.uber.org/dig"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/mana"
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
	snapshot := deps.Tangle.LedgerState.SnapshotUTXO()

	aMana, err := snapshotAccessMana()
	if err != nil {
		return err
	}
	snapshot.AccessManaByNode = aMana

	f, err := os.OpenFile(snapshotFileName, os.O_RDWR|os.O_CREATE, 0o666)
	if err != nil {
		Plugin.LogErrorf("unable to create snapshot file %s", err)
	}

	n, err := snapshot.WriteTo(f)
	if err != nil {
		Plugin.LogErrorf("unable to write snapshot content to file %s", err)
	}

	Plugin.LogInfo("Snapshot information: ")
	Plugin.LogInfo("     Number of snapshotted transactions: ", len(snapshot.Transactions))
	Plugin.LogInfo("     Number of snapshotted accessManaEntries: ", len(snapshot.AccessManaByNode))

	Plugin.LogInfof("Bytes written %d", n)
	f.Close()

	return c.Attachment(snapshotFileName, snapshotFileName)
}

// snapshotAccessMana returns snapshot of the current access mana.
func snapshotAccessMana() (aManaSnapshot map[identity.ID]ledgerstate.AccessMana, err error) {
	aManaSnapshot = make(map[identity.ID]ledgerstate.AccessMana)

	m, t, err := messagelayer.GetManaMap(mana.AccessMana)
	if err != nil {
		return nil, err
	}
	for nodeID, aMana := range m {
		aManaSnapshot[nodeID] = ledgerstate.AccessMana{
			Value:     aMana,
			Timestamp: t,
		}
	}
	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
