package notarization

import (
	"github.com/iotaledger/goshimmer/packages/epoch"
	"github.com/iotaledger/hive.go/generics/model"
)

type ECRecord struct {
	model.Storable[epoch.EI, ECRecord, *ECRecord, ecRecord] `serix:"0"`
}

type ecRecord struct {
	ECR    *epoch.ECR `serix:"0"`
	PrevEC *epoch.EC  `serix:"1"`
}

func NewECRecord(ei epoch.EI) (new *ECRecord) {
	new = model.NewStorable[epoch.EI, ECRecord](&ecRecord{})
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
	e.SetModified()
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
	e.SetModified()
}
