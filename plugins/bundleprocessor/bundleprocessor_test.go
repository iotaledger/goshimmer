package bundleprocessor

import (
	"fmt"
	"testing"

	"github.com/iotaledger/goshimmer/packages/events"
	"github.com/iotaledger/goshimmer/packages/model/bundle"
	"github.com/iotaledger/goshimmer/packages/model/value_transaction"
	"github.com/iotaledger/goshimmer/plugins/tangle"
	"github.com/iotaledger/iota.go/trinary"
	"github.com/magiconair/properties/assert"
)

func TestProcessSolidBundleHead(t *testing.T) {
	tangle.PLUGIN.InitTest()

	tx := value_transaction.New()
	tx.SetTail(true)
	tx.SetValue(3)

	tx1 := value_transaction.New()
	tx1.SetTrunkTransactionHash(tx.GetHash())
	tx1.SetHead(true)

	tangle.StoreTransaction(tx)
	tangle.StoreTransaction(tx1)

	Events.BundleSolid.Attach(events.NewClosure(func(bundle *bundle.Bundle, transactions []*value_transaction.ValueTransaction) {
		fmt.Println("IT HAPPENED")
		fmt.Println(bundle.GetHash())
		fmt.Println(bundle.GetBundleEssenceHash())
	}))

	result, err := ProcessSolidBundleHead(tx1)
	if err != nil {
		t.Error(err)
	} else {
		assert.Equal(t, result.GetHash(), trinary.Trytes("UFWJYEWKMEQDNSQUCUWBGOFRHVBGHVVYEZCLCGRDTRQSMAFALTIPMJEEYFDPMQCNJWLXUWFMBZGHQRO99"), "invalid bundle hash")
		assert.Equal(t, result.IsValueBundle(), true, "invalid value bundle status")
	}
}
