package booker

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/iotaledger/goshimmer/packages/core/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/core/tangle"
	"github.com/iotaledger/goshimmer/packages/core/tangle/models"
)

func TestAttachments(t *testing.T) {
	modelTestFramework := models.NewTestFramework()

	block1 := NewBlock(tangle.NewBlock(modelTestFramework.CreateBlock("block1", models.WithStrongParents("Genesis"))))
	block2 := NewBlock(tangle.NewBlock(modelTestFramework.CreateBlock("block2", models.WithStrongParents("Genesis"))))

	var txID utxo.TransactionID
	assert.NoError(t, txID.FromRandomness())

	testAttachments := newAttachments()
	testAttachments.Store(txID, block1)
	testAttachments.Store(txID, block2)

	fmt.Println(testAttachments.Get(txID))
}
