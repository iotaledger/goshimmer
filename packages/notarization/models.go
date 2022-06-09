package notarization

import (
	"github.com/iotaledger/goshimmer/packages/epoch"
	"github.com/iotaledger/hive.go/generics/model"
)

// ECRecord is a storable object represents the ecRecord of an epoch.
type ECRecord struct {
	model.Storable[epoch.EI, ECRecord, *ECRecord, ecRecord] `serix:"0"`
}

type ecRecord struct {
	ECR    *epoch.ECR `serix:"0"`
	PrevEC *epoch.EC  `serix:"1"`
}

// NewECRecord creates and returns a ECRecord of the given EI.
func NewECRecord(ei epoch.EI) (new *ECRecord) {
	new = model.NewStorable[epoch.EI, ECRecord](&ecRecord{})
	new.SetID(ei)
	return
}

// ECR returns the ECR of an ECRecord.
func (e *ECRecord) ECR() *epoch.ECR {
	e.RLock()
	defer e.RUnlock()

	return e.M.ECR
}

// SetECR sets the ECR of an ECRecord.
func (e *ECRecord) SetECR(ecr *epoch.ECR) {
	e.Lock()
	defer e.Unlock()

	e.M.ECR = ecr
	e.SetModified()
}

// PrevEC returns the EC of an ECRecord.
func (e *ECRecord) PrevEC() *epoch.EC {
	e.RLock()
	defer e.RUnlock()

	return e.M.PrevEC
}

// SetPrevEC sets the PrevEC of an ECRecord.
func (e *ECRecord) SetPrevEC(prevEC *epoch.EC) {
	e.Lock()
	defer e.Unlock()

	e.M.PrevEC = prevEC
	e.SetModified()
}
