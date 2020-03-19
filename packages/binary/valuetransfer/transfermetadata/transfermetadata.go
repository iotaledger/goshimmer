package transfermetadata

import (
	"github.com/iotaledger/hive.go/objectstorage"
)

type TransferMetadata struct {
	objectstorage.StorableObjectFlags
}

func (metadata *TransferMetadata) IsSolid() bool {
	return true
}
