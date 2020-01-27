package tipselection

import (
	"math/rand"
	"sync"

	"github.com/iotaledger/goshimmer/packages/model/meta_transaction"
	"github.com/iotaledger/iota.go/trinary"
)

var (
	tipSet map[trinary.Hash]struct{}
	mutex  sync.RWMutex
)

func GetRandomTip(excluding ...trinary.Hash) trinary.Trytes {
	mutex.RLock()
	defer mutex.RUnlock()

	numTips := len(tipSet)
	if numTips == 0 {
		return meta_transaction.BRANCH_NULL_HASH
	}

	var ignore trinary.Hash
	if len(excluding) > 0 {
		ignore = excluding[0]
	}
	if _, contains := tipSet[ignore]; contains {
		if numTips == 1 {
			return ignore
		}
		numTips -= 1
	}

	i := rand.Intn(numTips)
	for k := range tipSet {
		if k == ignore {
			continue
		}
		if i == 0 {
			return k
		}
		i--
	}
	panic("unreachable")
}

func GetTipsCount() int {
	mutex.RLock()
	defer mutex.RUnlock()

	return len(tipSet)
}
