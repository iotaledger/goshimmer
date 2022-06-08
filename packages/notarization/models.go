package notarization

import (
	"github.com/iotaledger/goshimmer/packages/epoch"
	"github.com/iotaledger/goshimmer/packages/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/hive.go/generics/model"
)

type ECRecord struct {
	model.Storable[epoch.EI, ecRecord] `serix:"0"`
}

type ecRecord struct {
	ECR    *epoch.ECR `serix:"0"`
	PrevEC *epoch.EC  `serix:"1"`
}

func NewECRecord(ei epoch.EI) (new *ECRecord) {
	new = &ECRecord{
		model.NewStorable[epoch.EI](ecRecord{}),
	}
	new.SetID(ei)
	return
}

func (e *ECRecord) ECR() *epoch.ECR {
	e.RLock()
	defer e.RUnlock()

	return e.M.ECR
}

func (e *ECRecord) SetECR(ecr *epoch.ECR) {
	e.Lock()
	defer e.Unlock()

	e.M.ECR = ecr
}

func (e *ECRecord) PrevEC() *epoch.EC {
	e.RLock()
	defer e.RUnlock()

	return e.M.PrevEC
}

func (e *ECRecord) SetPrevEC(prevEC *epoch.EC) {
	e.Lock()
	defer e.Unlock()

	e.M.PrevEC = prevEC
}

// region TangleLeaf ///////////////////////////////////////////////////////////////////////////////////////////////

type TangleLeaf struct {
	model.Storable[epoch.EI, tangle.MessageID] `serix:"0"`
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region TangleLeaf ///////////////////////////////////////////////////////////////////////////////////////////////

type MutationLeaf struct {
	model.Storable[epoch.EI, utxo.TransactionID] `serix:"0"`
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region OutputID /////////////////////////////////////////////////////////////////////////////////////////////////////

type OutputID struct {
	model.Storable[utxo.OutputID, utxo.OutputID] `serix:"0"`
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////