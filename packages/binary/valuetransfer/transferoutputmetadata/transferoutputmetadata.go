package transferoutputmetadata

import (
	"github.com/iotaledger/hive.go/objectstorage"
)

type TransferOutputMetadata struct {
}

func (t TransferOutputMetadata) MarshalBinary() (data []byte, err error) {
	panic("implement me")
}

func (t TransferOutputMetadata) UnmarshalBinary(data []byte) error {
	panic("implement me")
}

func (t TransferOutputMetadata) SetModified(modified ...bool) {
	panic("implement me")
}

func (t TransferOutputMetadata) IsModified() bool {
	panic("implement me")
}

func (t TransferOutputMetadata) Delete(delete ...bool) {
	panic("implement me")
}

func (t TransferOutputMetadata) IsDeleted() bool {
	panic("implement me")
}

func (t TransferOutputMetadata) Persist(enabled ...bool) {
	panic("implement me")
}

func (t TransferOutputMetadata) PersistenceEnabled() bool {
	panic("implement me")
}

func (t TransferOutputMetadata) Update(other objectstorage.StorableObject) {
	panic("implement me")
}

func (t TransferOutputMetadata) GetStorageKey() []byte {
	panic("implement me")
}

var _ objectstorage.StorableObject = &TransferOutputMetadata{}
