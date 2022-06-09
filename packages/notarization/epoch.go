package notarization

import (
	"github.com/iotaledger/hive.go/generics/model"
	"github.com/iotaledger/hive.go/generics/orderedmap"

	"github.com/iotaledger/goshimmer/packages/epoch"
	"github.com/iotaledger/goshimmer/packages/ledger"
	"github.com/iotaledger/goshimmer/packages/ledger/utxo"
)

type EpochDiffs struct {
	orderedmap.OrderedMap[epoch.EI, *EpochDiff] `serix:"0"`
}

type EpochDiff struct {
	model.Storable[epoch.EI, EpochDiff, *EpochDiff, epochDiff] `serix:"0"`
}

type epochDiff struct {
	EI              epoch.EI                `serix:"0"`
	Created         utxo.Outputs            `serix:"1"`
	CreatedMetadata *ledger.OutputsMetadata `serix:"2"`
	Spent           utxo.Outputs            `serix:"3"`
	SpentMetadata   *ledger.OutputsMetadata `serix:"4"`
}

func NewEpochDiff(ei epoch.EI) (new *EpochDiff) {
	new = model.NewStorable[epoch.EI, EpochDiff](&epochDiff{
		EI: ei,
	})
	new.SetID(ei)
	return
}

func (e *EpochDiff) EI() epoch.EI {
	e.RLock()
	defer e.RUnlock()

	return e.M.EI
}

func (e *EpochDiff) SetEI(ei epoch.EI) {
	e.Lock()
	defer e.Unlock()

	e.M.EI = ei
	e.SetModified()
}

func (e *EpochDiff) AddCreated(created utxo.Output) {
	e.Lock()
	defer e.Unlock()

	e.M.Created.Add(created)
	e.SetModified()
}

func (e *EpochDiff) DeleteCreated(id utxo.OutputID) (existed bool) {
	e.Lock()
	defer e.Unlock()

	if existed = e.M.Created.OrderedMap.Delete(id); existed {
		e.SetModified()
	}

	return
}

func (e *EpochDiff) AddSpent(spent utxo.Output) {
	e.Lock()
	defer e.Unlock()

	e.M.Spent.Add(spent)
	e.SetModified()
}

func (e *EpochDiff) DeleteSpent(id utxo.OutputID) (existed bool) {
	e.Lock()
	defer e.Unlock()

	if existed = e.M.Spent.OrderedMap.Delete(id); existed {
		e.SetModified()
	}

	return
}

func (e *EpochDiff) Created() *utxo.Outputs {
	e.RLock()
	defer e.RUnlock()

	return &utxo.Outputs{*e.M.Created.OrderedMap.Clone()}
}

func (e *EpochDiff) SetCreated(created utxo.Outputs) {
	e.Lock()
	defer e.Unlock()

	e.M.Created = created
	e.SetModified()
}

func (e *EpochDiff) Spent() *utxo.Outputs {
	e.RLock()
	defer e.RUnlock()

	return &utxo.Outputs{*e.M.Spent.OrderedMap.Clone()}
}

func (e *EpochDiff) SetSpent(spent utxo.Outputs) {
	e.Lock()
	defer e.Unlock()

	e.M.Spent = spent
	e.SetModified()
}
