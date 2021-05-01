package snapshot

import (
	"os"
	"sync"

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
	snapshot := messagelayer.Tangle().LedgerState.Snapshot()

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
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
