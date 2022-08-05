package causalorder

import (
	"github.com/iotaledger/hive.go/core/generics/options"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
)

func WithReferenceValidator[ID epoch.IndexedID, Entity OrderedEntity[ID]](referenceValidator func(entity Entity, parent Entity) bool) options.Option[CausalOrder[ID, Entity]] {
	return func(causalOrder *CausalOrder[ID, Entity]) {
		causalOrder.isReferenceValid = referenceValidator
	}
}
