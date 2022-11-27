package snapshot

import (
	"io"
	"os"

	"github.com/iotaledger/hive.go/core/generics/constraints"

	"github.com/iotaledger/goshimmer/packages/protocol/engine"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledgerstate"
)

func ReadSnapshot(fileHandle *os.File, engineInstance *engine.Engine) {
	if err := engineInstance.Storage.Settings.Import(fileHandle); err != nil {
		panic(err)
	} else if err := engineInstance.Storage.Commitments.Import(fileHandle); err != nil {
		panic(err)
	} else if err := engineInstance.Storage.Settings.SetChainID(engineInstance.Storage.Settings.LatestCommitment().ID()); err != nil {
		panic(err)
	} else if err := engineInstance.EvictionState.Import(fileHandle); err != nil {
		panic(err)
	} else if err := engineInstance.Storage.Attestors.Import(fileHandle); err != nil {
		panic(err)
	} else if err := engineInstance.LedgerState.Import(fileHandle); err != nil {
		panic(err)
	}

	// We need to set the genesis time before we add the activity log as otherwise the calculation is based on the empty time value.
	engineInstance.Clock.SetAcceptedTime(engineInstance.Storage.Settings.LatestCommitment().Index().EndTime())
	engineInstance.Clock.SetConfirmedTime(engineInstance.Storage.Settings.LatestCommitment().Index().EndTime())

	// Ledgerstate
	{
		ProcessChunks(NewChunkedReader[ledgerstate.OutputWithMetadata](fileHandle),
			engineInstance.ManaTracker.ImportOutputs,
			// This will import into all the consumers too: sybilprotection and ledgerState.unspentOutputIDs
		)
	}
}

func ProcessChunks[A any, B constraints.MarshalablePtr[A]](chunkedReader *ChunkedReader[A, B], chunkConsumers ...func([]B)) {
	for !chunkedReader.IsFinished() {
		chunk, err := chunkedReader.ReadChunk()
		if err != nil {
			if err == io.EOF {
				break
			}
			panic(err)
		}

		for _, chunkConsumer := range chunkConsumers {
			chunkConsumer(chunk)
		}
	}
}
