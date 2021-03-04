package syncbeaconfollower

import (
	"sync"

	"github.com/iotaledger/hive.go/node"
)

const (
	// PluginName is the plugin name of the sync beacon plugin.
	PluginName = "SyncBeaconFollower"
)

var (
	// plugin is the plugin instance of the sync beacon plugin.
	plugin *node.Plugin
	once   sync.Once
)

// Plugin gets the plugin instance.
func Plugin() *node.Plugin {
	once.Do(func() {
		plugin = node.NewPlugin(PluginName, node.Enabled, func(plugin *node.Plugin) {})
	})
	return plugin
}
