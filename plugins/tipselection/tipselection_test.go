package tipselection

import (
	"fmt"
	"testing"

	"github.com/iotaledger/goshimmer/packages/model/value_transaction"
	"github.com/iotaledger/goshimmer/plugins/tangle"
)

func Test(t *testing.T) {
	configure(nil)

	for i := 0; i < 1000; i++ {
		tx := value_transaction.New()
		tx.SetValue(int64(i + 1))
		tx.SetBranchTransactionHash(GetRandomTip())
		tx.SetTrunkTransactionHash(GetRandomTip())

		tangle.Events.TransactionSolid.Trigger(tx)

		fmt.Println(GetTipsCount())
	}
}
