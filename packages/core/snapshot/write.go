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

var delimiter = []byte{';', ';', ';'}

// streamSnapshotDataTo writes snapshot to a given writer.
func streamSnapshotDataTo(
	writeSeeker io.WriteSeeker,
	headerProd HeaderProducerFunc,
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
		if err := writeFunc("outputs", append(data, delimiter...)); err != nil {
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

	bytes, err := serix.DefaultAPI.Encode(context.Background(), epochDiffs, serix.WithValidation())
	if err != nil {
		return nil, err
	}
	if err := writeFunc(fmt.Sprintf("diffEpoch"), append(bytes, delimiter...)); err != nil {
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

// NewLedgerUTXOStatesProducer returns a OutputWithMetadataProducerFunc that provide OutputWithMetadatas from the ledger.
func NewLedgerUTXOStatesProducer(lastConfirmedEpoch epoch.Index, nmgr *notarization.Manager) UTXOStatesProducerFunc {
	prodChan := make(chan *ledger.OutputWithMetadata)
	nmgr.SnapshotLedgerState(lastConfirmedEpoch, prodChan)

	return func() *ledger.OutputWithMetadata {
		obj, ok := <-prodChan
		if !ok {
			return nil
		}
		return obj
	}
}

// NewEpochDiffsProducer returns a OutputWithMetadataProducerFunc that provide OutputWithMetadatas from the ledger.
func NewEpochDiffsProducer(lastConfirmedEpoch, latestCommitableEpoch epoch.Index, nmgr *notarization.Manager) EpochDiffProducerFunc {
	epochDiffs, err := nmgr.SnapshotEpochDiffs(lastConfirmedEpoch, latestCommitableEpoch)

	return func() (map[epoch.Index]*ledger.EpochDiff, error) {
		return epochDiffs, err
	}
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

	if err := writeFunc("latestECRecord", append(data, delimiter...)); err != nil {
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