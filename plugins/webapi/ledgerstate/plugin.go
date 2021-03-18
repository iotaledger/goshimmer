package ledgerstate

import (
	"sync"

	"github.com/iotaledger/goshimmer/plugins/webapi"
	"github.com/iotaledger/hive.go/node"
)

// region Plugin ///////////////////////////////////////////////////////////////////////////////////////////////////////

var (
	// plugin holds the singleton instance of the plugin.
	plugin *node.Plugin

	// pluginOnce is used to ensure that the plugin is a singleton.
	pluginOnce sync.Once
)

// Plugin returns the plugin as a singleton.
func Plugin() *node.Plugin {
	pluginOnce.Do(func() {
		plugin = node.NewPlugin("WebAPI ledgerstate Endpoint", node.Enabled, configure)
	})

	return plugin
}

// configure bind the API endpoints to their corresponding route.
func configure(*node.Plugin) {
	webapi.Server().GET("ledgerstate/branches/:branchID", GetBranchEndPoint)
	webapi.Server().GET("ledgerstate/branches/:branchID/children", GetBranchChildrenEndPoint)
	webapi.Server().GET("ledgerstate/branches/:branchID/conflicts", GetBranchConflictsEndPoint)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
