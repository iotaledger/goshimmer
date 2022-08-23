package retainer

import (
	"sync"
	"time"

	"github.com/iotaledger/goshimmer/packages/core/booker"
	"github.com/iotaledger/goshimmer/packages/core/otv"
	"github.com/iotaledger/goshimmer/packages/core/tangle"
)

type Metadata struct {
	BlockDAG blockWithTime[tangle.Block]
	Booker   blockWithTime[booker.Block]
	OTV      blockWithTime[otv.Block]

	m sync.RWMutex
}

type blockWithTime[BlockType any] struct {
	Block BlockType
	Time  time.Time
}
