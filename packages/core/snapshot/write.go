package snapshot

import (
	"os"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/protocol/engine"
)

func WriteSnapshot(filePath string, engineInstance *engine.Engine, depth epoch.Index) {
	fileHandle, err := os.Create(filePath)
	defer func(fileHandle *os.File) {
		if err = fileHandle.Close(); err != nil {
			panic(err)
		}
	}(fileHandle)

	if err != nil {
		panic(err)
	}

	targetEpoch := engineInstance.Storage.Settings.LatestCommitment().Index() - depth

	if err := engineInstance.Storage.Settings.Export(fileHandle); err != nil {
		panic(err)
	} else if err := engineInstance.Storage.Commitments.Export(fileHandle, targetEpoch); err != nil {
		panic(err)
	} else if err := engineInstance.EvictionState.Export(fileHandle, targetEpoch); err != nil {
		panic(err)
	} else if err := engineInstance.Storage.Attestors.Export(fileHandle, targetEpoch-1); err != nil {
		panic(err)
	} else if err := engineInstance.LedgerState.Export(fileHandle, targetEpoch); err != nil {
		panic(err)
	}
}
