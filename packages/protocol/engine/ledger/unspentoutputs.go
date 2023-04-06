package ledger

import (
	"github.com/iotaledger/goshimmer/packages/core/traits"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/utxo"
	"github.com/iotaledger/hive.go/ads"
	"github.com/iotaledger/hive.go/runtime/module"
)

// UnspentOutputs is a submodule that provides access to the unspent outputs of the ledger state.
type UnspentOutputs interface {
	// IDs returns the IDs of the unspent outputs.
	IDs() *ads.Set[utxo.OutputID, *utxo.OutputID]

	// Subscribe subscribes to changes in the unspent outputs.
	Subscribe(UnspentOutputsSubscriber)

	// Unsubscribe unsubscribes from changes in the unspent outputs.
	Unsubscribe(UnspentOutputsSubscriber)

	// ApplyCreatedOutput applies the given output to the unspent outputs.
	ApplyCreatedOutput(*mempool.OutputWithMetadata) error

	// BatchCommittable embeds the required methods of the BatchCommittable trait.
	traits.BatchCommittable

	// Interface embeds the required methods of the module.Interface.
	module.Interface
}
