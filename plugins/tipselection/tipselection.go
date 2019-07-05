package tipselection

import (
	"github.com/iotaledger/goshimmer/packages/datastructure"
	"github.com/iotaledger/goshimmer/packages/model/meta_transaction"
	"github.com/iotaledger/iota.go/trinary"
)

var tips = datastructure.NewRandomMap()

func GetRandomTip() (result trinary.Trytes) {
	if randomTipHash := tips.RandomEntry(); randomTipHash != nil {
		result = randomTipHash.(trinary.Trytes)
	} else {
		result = meta_transaction.BRANCH_NULL_HASH
	}

	return
}

func GetTipsCount() int {
	return tips.Size()
}
