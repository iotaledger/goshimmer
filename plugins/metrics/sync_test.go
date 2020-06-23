package metrics

import (
	"sync"
	"testing"

	"github.com/iotaledger/goshimmer/packages/metrics"
	"github.com/iotaledger/hive.go/events"
	"github.com/stretchr/testify/assert"
)

func TestSynced(t *testing.T) {
	var wg sync.WaitGroup
	metrics.Events().Synced.Attach(events.NewClosure(func(synced bool) {
		// sync plugin and node not run, so we expect synced to be false
		assert.Equal(t, false, synced)
		wg.Done()
	}))

	wg.Add(1)
	measureSynced()
	wg.Wait()
}
