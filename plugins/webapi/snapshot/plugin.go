package snapshot

import (
	"net/http"
	"sync"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
	"github.com/iotaledger/goshimmer/plugins/webapi"
	"github.com/iotaledger/hive.go/node"
	"github.com/labstack/echo"
)

// region Plugin ///////////////////////////////////////////////////////////////////////////////////////////////////////

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

func DumpCurrentLedger(c echo.Context) (err error) {
	// transactions :=
	messagelayer.Tangle().LedgerState.Transactions()
	// for _, transaction := range transactions {
	// 	for _, output := range transaction.Essence().Outputs(){
	// 		messagelayer.Tangle().LedgerState.
	// 	}
	// }
	return c.JSON(http.StatusOK, nil)
}

func New_DumpCurrentLedger(c echo.Context) (snapshot *ledgerstate.Snapshot, err error) {
	transactions := messagelayer.Tangle().LedgerState.Transactions()
	snapshot = &ledgerstate.Snapshot{
		Transactions: map[ledgerstate.TransactionID]*ledgerstate.TransactionEssence{},
	}

	for _, transaction := range transactions {
		snapshot.Transactions[transaction.ID()] = transaction.Essence()
	}

	return snapshot, c.JSON(http.StatusOK, nil)

	// // testTangle.LedgerState.LoadSnapshot(snapshot)

}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
