package snapshot

import (
	"os"
	"sync"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/mana"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
	"github.com/iotaledger/goshimmer/plugins/webapi"

	"github.com/iotaledger/hive.go/identity"
	"github.com/iotaledger/hive.go/node"
	"github.com/labstack/echo"
)

// region Plugin ///////////////////////////////////////////////////////////////////////////////////////////////////////

const (
	snapshotFileName = "snapshot.bin"
)

var (
	// plugin holds the singleton instance of the plugin.
	plugin *node.Plugin

	// pluginOnce is used to ensure that the plugin is a singleton.
	once sync.Once
)

// Plugin returns the plugin as a singleton.
func Plugin() *node.Plugin {
	once.Do(func() {
		plugin = node.NewPlugin("snapshot", node.Disabled, func(*node.Plugin) {
			webapi.Server().GET("snapshot", DumpCurrentLedger)
		})
	})

	return plugin
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region DumpCurrentLedger ///////////////////////////////////////////////////////////////////////////////////////////////////

// DumpCurrentLedger dumps a snapshot (all unspent UTXO and all of the access mana) from now.
func DumpCurrentLedger(c echo.Context) (err error) {
	snapshot := messagelayer.Tangle().LedgerState.SnapshotUTXO()

	aMana, err := snapshotAccessMana()
	if err != nil {
		return err
	}
	snapshot.AccessManaByNode = aMana

	f, err := os.OpenFile(snapshotFileName, os.O_RDWR|os.O_CREATE, 0666)
	if err != nil {
		plugin.LogErrorf("unable to create snapshot file %s", err)
	}

	n, err := snapshot.WriteTo(f)
	if err != nil {
		plugin.LogErrorf("unable to write snapshot content to file %s", err)
	}

	// plugin.LogInfo(snapshot)
	plugin.LogInfo("Snapshot information: ")
	plugin.LogInfo("     Number of snapshotted transactions: ", len(snapshot.Transactions))
	plugin.LogInfo("          inputs, outputs, txID, unspentOutputs")
	for key, tx := range snapshot.Transactions {
		plugin.LogInfo("          ", len(tx.Essence.Inputs()), len(tx.Essence.Outputs()), key)
		plugin.LogInfo("          ", tx.UnspentOutputs)
	}
	plugin.LogInfo("     Number of snapshotted accessManaEntries: ", len(snapshot.AccessManaByNode))
	plugin.LogInfo("          nodeID, aMana, timestamp")
	for nodeID, accessMana := range snapshot.AccessManaByNode {
		plugin.LogInfo("          ", nodeID, accessMana.Value, accessMana.Timestamp)
	}

	plugin.LogInfof("Bytes written %d", n)
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
