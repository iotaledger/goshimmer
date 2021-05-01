package snapshot

import (
	"os"
	"sync"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
	"github.com/iotaledger/goshimmer/plugins/webapi"

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
		plugin = node.NewPlugin("WebAPI snapshot Endpoint", node.Enabled, func(*node.Plugin) {
			webapi.Server().GET("snapshot", DumpCurrentLedger)
		})
	})

	return plugin
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region DumpCurrentLedger ///////////////////////////////////////////////////////////////////////////////////////////////////

// DumpCurrentLedger dumps the current ledger state.
func DumpCurrentLedger(c echo.Context) (err error) {
	snapshot := &ledgerstate.Snapshot{
		Transactions: make(map[ledgerstate.TransactionID]ledgerstate.Record),
	}

	transactions := messagelayer.Tangle().LedgerState.Transactions()
	for _, transaction := range transactions {
		unpsentOutputs := make([]bool, len(transaction.Essence().Outputs()))
		for i, output := range transaction.Essence().Outputs() {
			messagelayer.Tangle().LedgerState.OutputMetadata(output.ID()).Consume(func(outputMetadata *ledgerstate.OutputMetadata) {
				if outputMetadata.ConsumerCount() == 0 {
					unpsentOutputs[i] = true
				}
			})
		}
		if len(unpsentOutputs) > 0 {
			snapshot.Transactions[transaction.ID()] = ledgerstate.Record{
				Essence:        transaction.Essence(),
				UnpsentOutputs: unpsentOutputs,
			}
		}
	}

	// plugin.LogInfo(snapshot)

	f, err := os.OpenFile(snapshotFileName, os.O_RDWR|os.O_CREATE, 0666)
	if err != nil {
		plugin.LogFatal("unable to create snapshot file", err)
	}

	n, err := snapshot.WriteTo(f)
	if err != nil {
		plugin.LogFatal("unable to write snapshot content to file", err)
	}

	plugin.LogInfo("Bytes written %d", n)
	f.Close()

	return c.Attachment(snapshotFileName, snapshotFileName)
	// JSON(http.StatusOK, nil)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
