package drng

import (
	"go.uber.org/dig"

	"github.com/iotaledger/goshimmer/packages/tangle"
)

var deps dependencies

type dependencies struct {
	dig.In

	Tangle             *tangle.Tangle
	ConsensusMechanism tangle.ConsensusMechanism
}

// Invoke invokes dependencies for tools/drng apis.
func Invoke(dep dependencies) {
	deps = dep
}
