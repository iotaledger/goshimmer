package snapshot

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"

	"github.com/iotaledger/hive.go/core/serix"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/ledger"
	"github.com/iotaledger/goshimmer/packages/core/notarization"
)

const utxoStatesChunkSize = 100

// streamSnapshotDataTo writes snapshot to a given writer.
func streamSnapshotDataTo(
	writeSeeker io.WriteSeeker,
	headerProd HeaderProducerFunc,
	sepsProd SolidEntryPointsProducerFunc,
	outputProd UTXOStatesProducerFunc,
	epochDiffsProd EpochDiffProducerFunc) (*ledger.SnapshotHeader, error) {

	writeFunc := func(name string, value any) error {
		return writeFunc(writeSeeker, name, value)
	}

	writeOutputWithMetadatasFunc := func(chunks []*ledger.OutputWithMetadata) error {
		if len(chunks) == 0 {
			return nil
		}

		data, err := serix.DefaultAPI.Encode(context.Background(), chunks, serix.WithValidation())
		if err != nil {
			return err
		}
		if err := writeFunc("outputsBytesLen", int64(len(data))); err != nil {
			return err
		}
		if err := writeFunc("outputs", data); err != nil {
			return err
		}
		return nil
	}

	header, err := headerProd()
	if err != nil {
		return nil, err
	}

	err = writeSnapshotHeader(writeSeeker, header)
	if err != nil {
		return nil, err
	}

	// write solid entry points
	for {
		seps := sepsProd()
		if seps == nil {
			break
		}

		writeSolidEntryPoints(writeSeeker, seps)
	}

	// write outputWithMetadata
	var outputWithMetadataCounter uint64
	var outputChunkCounter int
	chunksOutputWithMetadata := make([]*ledger.OutputWithMetadata, 0)
	for {
		output := outputProd()
		if output == nil {
			// write rests of outputWithMetadatas
			err = writeOutputWithMetadatasFunc(chunksOutputWithMetadata)
			if err != nil {
				return nil, err
			}
			break
		}
		outputWithMetadataCounter++
		outputChunkCounter++
		chunksOutputWithMetadata = append(chunksOutputWithMetadata, output)

		// put a delimeter every utxoStatesChunkSize outputs
		if outputChunkCounter == utxoStatesChunkSize {
			err = writeOutputWithMetadatasFunc(chunksOutputWithMetadata)
			if err != nil {
				return nil, err
			}
			chunksOutputWithMetadata = make([]*ledger.OutputWithMetadata, 0)
			outputChunkCounter = 0
		}
	}

	// write epochDiffs
	epochDiffs, err := epochDiffsProd()
	if err != nil {
		return nil, err
	}

	epochDiffsBytes, err := serix.DefaultAPI.Encode(context.Background(), epochDiffs, serix.WithValidation())
	if err != nil {
		return nil, err
	}
	if err := writeFunc(fmt.Sprintf("epochDiffBytesLen"), int64(len(epochDiffsBytes))); err != nil {
		return nil, err
	}
	if err := writeFunc(fmt.Sprintf("epochDiffs"), epochDiffsBytes); err != nil {
		return nil, err
	}

	// seek back to the file position of the outputWithMetadata counter
	if _, err := writeSeeker.Seek(0, io.SeekStart); err != nil {
		return nil, fmt.Errorf("unable to seek to LS counter placeholders: %w", err)
	}
	if err := writeFunc(fmt.Sprintf("outputWithMetadata counter %d", outputWithMetadataCounter), outputWithMetadataCounter); err != nil {
		return nil, err
	}
	header.OutputWithMetadataCount = outputWithMetadataCounter

	return header, nil
}

// NewSolidEntryPointsProducer returns a SolidEntryPointsProducerFunc that provide solid entry points from the snapshot manager.
func NewSolidEntryPointsProducer(fullEpochIndex, latestCommitableEpoch epoch.Index, smgr *Manager) SolidEntryPointsProducerFunc {
	prodChan := make(chan *SolidEntryPoints)
	stopChan := make(chan struct{})
	smgr.snapshotSolidEntryPoints(fullEpochIndex, latestCommitableEpoch, prodChan, stopChan)

	return func() *SolidEntryPoints {
		select {
		case obj := <-prodChan:
			return obj
		case <-stopChan:
			close(prodChan)
			return nil
		}
	}
}

// NewLedgerUTXOStatesProducer returns a OutputWithMetadataProducerFunc that provide OutputWithMetadatas from the ledger.
func NewLedgerUTXOStatesProducer(nmgr *notarization.Manager) UTXOStatesProducerFunc {
	prodChan := make(chan *ledger.OutputWithMetadata)
	stopChan := make(chan struct{})
	nmgr.SnapshotLedgerState(prodChan, stopChan)

	return func() *ledger.OutputWithMetadata {
		select {
		case obj := <-prodChan:
			return obj
		case <-stopChan:
			close(prodChan)
			return nil
		}
	}
}

// NewEpochDiffsProducer returns a OutputWithMetadataProducerFunc that provide OutputWithMetadatas from the ledger.
func NewEpochDiffsProducer(fullEpochIndex, latestCommitableEpoch epoch.Index, nmgr *notarization.Manager) EpochDiffProducerFunc {
	epochDiffs, err := nmgr.SnapshotEpochDiffs(fullEpochIndex, latestCommitableEpoch)

	return func() (map[epoch.Index]*ledger.EpochDiff, error) {
		return epochDiffs, err
	}
}

func writeSolidEntryPoints(writeSeeker io.WriteSeeker, seps *SolidEntryPoints) error {
	writeFunc := func(name string, value any) error {
		return writeFunc(writeSeeker, name, value)
	}

	data, err := serix.DefaultAPI.Encode(context.Background(), seps, serix.WithValidation())
	if err != nil {
		return err
	}

	if err := writeFunc("sepsBytesLen", int64(len(data))); err != nil {
		return err
	}
	if err := writeFunc("seps", data); err != nil {
		return err
	}

	return nil
}

func writeSnapshotHeader(writeSeeker io.WriteSeeker, header *ledger.SnapshotHeader) error {
	writeFunc := func(name string, value any) error {
		return writeFunc(writeSeeker, name, value)
	}

	if err := writeFunc(fmt.Sprintf("outputWithMetadata counter %d", header.OutputWithMetadataCount), header.OutputWithMetadataCount); err != nil {
		return err
	}

	if err := writeFunc(fmt.Sprintf("fullEpochIndex %d", header.FullEpochIndex), header.FullEpochIndex); err != nil {
		return err
	}

	if err := writeFunc(fmt.Sprintf("diffEpochIndex %d", header.DiffEpochIndex), header.DiffEpochIndex); err != nil {
		return err
	}

	data, err := header.LatestECRecord.Bytes()
	if err != nil {
		return err
	}

	if err := writeFunc("latestECRecordBytesLen", int64(len(data))); err != nil {
		return err
	}

	if err := writeFunc("latestECRecord", data); err != nil {
		return err
	}

	return nil
}

func writeFunc(writeSeeker io.WriteSeeker, variableName string, value any) error {
	length := binary.Size(value)
	if length == -1 {
		return fmt.Errorf("unable to determine length of %s", variableName)
	}

	if err := binary.Write(writeSeeker, binary.LittleEndian, value); err != nil {
		return fmt.Errorf("unable to write LS %s: %w", variableName, err)
	}

	return nil
}
