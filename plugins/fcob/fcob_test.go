package fcob

import (
	"testing"

	"github.com/iotaledger/goshimmer/packages/node"
)

func TestConfigure(t *testing.T) {
	// var events node.pluginEvents

	plugin := node.Plugin{
		nil, "", nil, nil,
	}

	Configure(&plugin)
}
