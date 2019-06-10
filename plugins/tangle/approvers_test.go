package tangle

import (
	"fmt"
	"testing"
	"time"

	"github.com/iotaledger/goshimmer/packages/events"
	"github.com/iotaledger/goshimmer/packages/ternary"
	"github.com/iotaledger/goshimmer/packages/transaction"
	"github.com/iotaledger/goshimmer/plugins/gossip"
)

func TestGetApprovers(t *testing.T) {
	configureDatabase(nil)

	approvers, err := GetApprovers(ternary.Trinary("AA"), NewApprovers)

	approvers.Add(ternary.Trinary("FF"))

	fmt.Println(approvers)
	fmt.Println(err)

	approvers1, err1 := GetApprovers(ternary.Trinary("AA"))

	fmt.Println(approvers1)
	fmt.Println(err1)
}

func TestSolidifier(t *testing.T) {
	configureDatabase(nil)
	configureSolidifier(nil)

	txData := make([]byte, 1572)
	txData[1571] = 1

	tx := transaction.FromBytes(txData)

	txData[1571] = 2
	tx1 := transaction.FromBytes(txData)
	tx1.BranchTransactionHash = tx.Hash

	txData[1571] = 3
	tx2 := transaction.FromBytes(txData)
	tx2.BranchTransactionHash = tx1.Hash

	txData[1571] = 4
	tx3 := transaction.FromBytes(txData)
	tx3.BranchTransactionHash = tx2.Hash

	fmt.Println(tx.Hash.ToString())
	fmt.Println(tx1.Hash.ToString())
	fmt.Println(tx2.Hash.ToString())
	fmt.Println(tx3.Hash.ToString())

	fmt.Println("============")

	Events.TransactionSolid.Attach(events.NewClosure(func(transaction *Transaction) {
		fmt.Println("SOLID: " + transaction.GetHash())
	}))

	gossip.Events.ReceiveTransaction.Trigger(tx)
	gossip.Events.ReceiveTransaction.Trigger(tx1)
	gossip.Events.ReceiveTransaction.Trigger(tx3)

	fmt.Println("...")

	time.Sleep(1 * time.Second)

	gossip.Events.ReceiveTransaction.Trigger(tx2)

	time.Sleep(1 * time.Second)
}
