package causalorder

import "github.com/iotaledger/hive.go/generics/options"

func WithReferenceValidator[IDType IDWithEpoch, EntityType Entity[IDType]](referenceValidator func(entity EntityType, parent EntityType) bool) options.Option[CausalOrder[IDType, EntityType]] {
	return func(causalOrder *CausalOrder[IDType, EntityType]) {
		causalOrder.optsReferenceValidator = referenceValidator
	}
}
