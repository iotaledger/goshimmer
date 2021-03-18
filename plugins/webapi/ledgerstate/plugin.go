package ledgerstate

import (
	"sync"

	"github.com/iotaledger/goshimmer/plugins/webapi"
	"github.com/iotaledger/hive.go/node"
)

var (
	plugin *node.Plugin
	once   sync.Once
)

// Plugin returns the Plugin.
func Plugin() *node.Plugin {
	once.Do(func() {
		plugin = node.NewPlugin("WebAPI ledgerstate Endpoint", node.Enabled, configure)
	})

	return plugin
}

func configure(plugin *node.Plugin) {
	webapi.Server().GET("ledgerstate/branch/:branchID", getBranch)
	//webapi.Server().GET("ledgerstate/branch/:branchID/conflicts", findBranchByIDHandler)
	//webapi.Server().GET("ledgerstate/branch/:branchID/children", findBranchByIDHandler)
}
