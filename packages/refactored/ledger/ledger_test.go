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

	ledger := New(mapdb.NewMapDB(), vm)
	ledger.outputStorage.Store(genesisOutput).Release()
	ledger.outputMetadataStorage.Store(genesisOutputMetadata).Release()

	ledger.ErrorEvent.Attach(event.NewClosure[error](func(err error) {
		fmt.Println(err)
	}))

	tx := NewMockedTransaction([]*MockedInput{
		NewMockedInput(genesisOutput.ID()),
	}, 2)

	fmt.Println(ledger.StoreAndProcessTransaction(tx))

	time.Sleep(2000 * time.Millisecond)
}
