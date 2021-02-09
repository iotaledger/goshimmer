package issuer

import (
	goSync "sync"

	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/goshimmer/packages/tangle/payload"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
	"github.com/iotaledger/goshimmer/plugins/syncbeaconfollower"
	"github.com/iotaledger/hive.go/node"
	"golang.org/x/xerrors"
)

// PluginName is the name of the issuer plugin.
const PluginName = "Issuer"

var (
	// plugin is the plugin instance of the issuer plugin.
	plugin *node.Plugin
	once   goSync.Once
)

// Plugin gets the plugin instance.
func Plugin() *node.Plugin {
	once.Do(func() {
		plugin = node.NewPlugin(PluginName, node.Enabled, configure)
	})
	return plugin
}

func configure(_ *node.Plugin) {}

// IssuePayload issues a payload to the message layer.
// If the node is not synchronized an error is returned.
func IssuePayload(payload payload.Payload, t ...*tangle.Tangle) (*tangle.Message, error) {
	if !syncbeaconfollower.Synced() {
		return nil, xerrors.Errorf("can't issue payload: %w", syncbeaconfollower.ErrNodeNotSynchronized)
	}

	msg, err := messagelayer.Tangle().MessageFactory.IssuePayload(payload, messagelayer.Tangle())
	if err != nil {
		return nil, err
	}

	return msg, nil
}
