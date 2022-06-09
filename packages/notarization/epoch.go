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
	model.Storable[epoch.EI, epochDiff] `serix:"0"`
}

type epochDiff struct {
	EI              epoch.EI                `serix:"0"`
	Created         utxo.Outputs            `serix:"1"`
	CreatedMetadata *ledger.OutputsMetadata `serix:"2"`
	Spent           utxo.Outputs            `serix:"3"`
	SpentMetadata   *ledger.OutputsMetadata `serix:"4"`
}
