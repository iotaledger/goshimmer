package snapshot

import (
	"encoding/binary"
	"os"

	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/identity"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/protocol/engine"
)

func WriteSnapshot(filePath string, engineInstance *engine.Engine, depth epoch.Index) {
	fileHandle, err := os.Create(filePath)
	defer fileHandle.Close()

	if err != nil {
		panic(err)
	}

	currentEpoch := engineInstance.Storage.Settings.LatestCommitment().Index()
	targetEpoch := currentEpoch - depth

	if err := engineInstance.Storage.Settings.Export(fileHandle); err != nil {
		panic(err)
	} else if err := engineInstance.Storage.Commitments.Export(fileHandle, targetEpoch); err != nil {
		panic(err)
	} else if err := engineInstance.EvictionState.Export(fileHandle, targetEpoch); err != nil {
		panic(err)
	} else if err := engineInstance.LedgerState.Export(fileHandle, targetEpoch); err != nil {
		panic(err)
	}

	// Activity Log
	{
		// Activity Log count
		binary.Write(fileHandle, binary.LittleEndian, uint32(engineInstance.Storage.Attestors.LoadAll(currentEpoch).Size()))
		// Activity Log size
		binary.Write(fileHandle, binary.LittleEndian, uint32(len(lo.PanicOnErr((&identity.ID{}).Bytes()))))
		engineInstance.Storage.Attestors.Stream(currentEpoch, func(id identity.ID) {
			binary.Write(fileHandle, binary.LittleEndian, id)
		})
	}
}
