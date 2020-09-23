package ledgerstate

import "github.com/iotaledger/hive.go/objectstorage"

type UTXODag struct {
	outputStorage *objectstorage.ObjectStorage
}

func (u *UTXODag) Output(outputID OutputID) *CachedOutput {
	return &CachedOutput{CachedObject: u.outputStorage.Load(outputID.Bytes())}
}
