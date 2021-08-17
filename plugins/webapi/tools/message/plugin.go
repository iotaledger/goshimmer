package message

import (
	"go.uber.org/dig"

	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/configuration"
)

var deps dependencies

type dependencies struct {
	dig.In

	Tangle             *tangle.Tangle
	ConsensusMechanism tangle.ConsensusMechanism
	Local              *peer.Local
	Config             *configuration.Configuration
}

func Invoke(dep dependencies) {
	deps = dep
}
