package testutil

import (
	"testing"

	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/tangle"
)

var valueTangleInstance *tangle.Tangle

func ValueTangle(t *testing.T) *tangle.Tangle {
	if valueTangleInstance == nil {
		valueTangleInstance = tangle.New(DB(t))
		if err := valueTangleInstance.Prune(); err != nil {
			t.Error(err)
		}
	}

	return valueTangleInstance
}
