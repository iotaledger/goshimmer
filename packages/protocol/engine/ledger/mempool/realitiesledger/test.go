package realitiesledger

import (
	"testing"

	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/vm/mockedvm"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/blockdag"
	"github.com/iotaledger/hive.go/runtime/options"
	"github.com/iotaledger/hive.go/runtime/workerpool"
)

func NewTestLedger(t *testing.T, workers *workerpool.Group, optsLedger ...options.Option[RealitiesLedger]) mempool.MemPool {
	storage := blockdag.NewTestStorage(t, workers)
	l := New(append([]options.Option[RealitiesLedger]{
		WithVM(new(mockedvm.MockedVM)),
	}, optsLedger...)...)
	l.Initialize(workers.CreatePool("RealitiesLedger", 2), storage)

	t.Cleanup(func() {
		workers.WaitChildren()
		l.Shutdown()
		storage.Shutdown()
	})

	return l
}

func NewDefaultTestFramework(t *testing.T, workers *workerpool.Group, optsLedger ...options.Option[RealitiesLedger]) *mempool.TestFramework {
	return mempool.NewTestFramework(t, NewTestLedger(t, workers.CreateGroup("RealitiesLedger"), optsLedger...))
}
