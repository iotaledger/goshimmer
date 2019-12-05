package tangle

import (
	"os"
	"sync"
	"testing"

	"github.com/golang/protobuf/proto"
	"github.com/iotaledger/hive.go/parameter"

	"github.com/iotaledger/goshimmer/packages/gossip"
	pb "github.com/iotaledger/goshimmer/packages/gossip/proto"
	"github.com/iotaledger/goshimmer/packages/model/value_transaction"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/node"
	"github.com/iotaledger/iota.go/trinary"
)

func TestMain(m *testing.M) {
	parameter.FetchConfig(false)
	os.Exit(m.Run())
}

func TestSolidifier(t *testing.T) {
	// show all error messages for tests
	// TODO: adjust logger package

	// start a test node
	node.Start(PLUGIN)

	// create transactions and chain them together
	transaction1 := value_transaction.New()
	transaction1.SetNonce(trinary.Trytes("99999999999999999999999999A"))
	transaction2 := value_transaction.New()
	transaction2.SetBranchTransactionHash(transaction1.GetHash())
	transaction3 := value_transaction.New()
	transaction3.SetBranchTransactionHash(transaction2.GetHash())
	transaction4 := value_transaction.New()
	transaction4.SetBranchTransactionHash(transaction3.GetHash())

	// setup event handlers
	var wg sync.WaitGroup
	Events.TransactionSolid.Attach(events.NewClosure(func(transaction *value_transaction.ValueTransaction) {
		wg.Done()
	}))

	// issue transactions
	wg.Add(4)
	tx := &pb.Transaction{Body: transaction1.MetaTransaction.GetBytes()}
	b, _ := proto.Marshal(tx)
	gossip.Event.NewTransaction.Trigger(b)

	tx = &pb.Transaction{Body: transaction2.MetaTransaction.GetBytes()}
	b, _ = proto.Marshal(tx)
	gossip.Event.NewTransaction.Trigger(b)

	tx = &pb.Transaction{Body: transaction3.MetaTransaction.GetBytes()}
	b, _ = proto.Marshal(tx)
	gossip.Event.NewTransaction.Trigger(b)

	tx = &pb.Transaction{Body: transaction4.MetaTransaction.GetBytes()}
	b, _ = proto.Marshal(tx)
	gossip.Event.NewTransaction.Trigger(b)

	// wait until all are solid
	wg.Wait()

	// shutdown test node
	node.Shutdown()
}
