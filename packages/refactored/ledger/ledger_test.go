package ledger

import (
	"fmt"
	"testing"
	"time"

	"github.com/iotaledger/hive.go/generics/event"
	"github.com/iotaledger/hive.go/kvstore/mapdb"

	"github.com/iotaledger/goshimmer/packages/consensus/gof"
	"github.com/iotaledger/goshimmer/packages/refactored/utxo"
)

func TestLedger(t *testing.T) {
	vm := NewMockedVM()

	genesisOutput := NewOutput(NewMockedOutput(utxo.EmptyTransactionID, 0))
	genesisOutputMetadata := NewOutputMetadata(genesisOutput.ID())
	genesisOutputMetadata.SetSolid(true)
	genesisOutputMetadata.SetGradeOfFinality(gof.High)

	nonExistingOutput := NewOutput(NewMockedOutput(utxo.EmptyTransactionID, 1))

	ledger := New(mapdb.NewMapDB(), vm)
	ledger.outputStorage.Store(genesisOutput).Release()
	ledger.outputMetadataStorage.Store(genesisOutputMetadata).Release()

	ledger.ErrorEvent.Attach(event.NewClosure[error](func(err error) {
		fmt.Println(err)
	}))

	fmt.Println(genesisOutput.ID())
	fmt.Println(nonExistingOutput.ID())

	tx1 := NewMockedTransaction([]*MockedInput{
		NewMockedInput(nonExistingOutput.ID()),
	}, 2)

	fmt.Println(tx1.ID())

	tx2 := NewMockedTransaction([]*MockedInput{
		NewMockedInput(genesisOutput.ID()),
	}, 3)

	fmt.Println(tx2.ID())

	fmt.Println(ledger.StoreAndProcessTransaction(tx1))
	fmt.Println(ledger.StoreAndProcessTransaction(tx2))

	time.Sleep(2000 * time.Millisecond)

	// testFramework.NewTransaction("TX1", []string{"G"}, 2)
	// testFramework.NewTransaction("TX2", []string{"TX1_1"}, 1)
	// "G->TX1(2)"
	// "TX1_1->TX2(3)"
}
