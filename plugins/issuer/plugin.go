package issuer

import (
	"fmt"

	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/message"
	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/payload"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
	"github.com/iotaledger/goshimmer/plugins/sync"
	"github.com/iotaledger/hive.go/node"
)

// PluginName is the name of the issuer plugin.
const PluginName = "Issuer"

var (
	// Plugin is the plugin instance of the issuer plugin.
	Plugin = node.NewPlugin(PluginName, node.Enabled, configure)
)

func configure(_ *node.Plugin) {}

// IssuePayload issues a payload to the message layer.
// If the node is not synchronized an error is returned.
func IssuePayload(payload payload.Payload) (*message.Message, error) {
	if !sync.Synced() {
		return nil, fmt.Errorf("can't issue payload: %w", sync.ErrNodeNotSynchronized)
	}
	return messagelayer.MessageFactory.IssuePayload(payload), nil
}
