package snapshot

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/core/serix"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/ledger"
	"github.com/iotaledger/goshimmer/packages/core/notarization"
)

const chunkSize = 100

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

	header, err := headerProd()
	if err != nil {
		return nil, err
	}

	err = writeSnapshotHeader(writeSeeker, header)
	if err != nil {
		return nil, err
	}

	// write solid entry points
	for i := header.FullEpochIndex; i <= header.DiffEpochIndex; i++ {
		seps := sepsProd()
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
			err = writeOutputsWithMetadatas(writeSeeker, chunksOutputWithMetadata)
			if err != nil {
				return nil, err
			}
			break
		}
		outputWithMetadataCounter++
		outputChunkCounter++
		chunksOutputWithMetadata = append(chunksOutputWithMetadata, output)

		// put a delimeter every chunkSize outputs
		if outputChunkCounter == chunkSize {
			err = writeOutputsWithMetadatas(writeSeeker, chunksOutputWithMetadata)
			if err != nil {
				return nil, err
			}
			chunksOutputWithMetadata = make([]*ledger.OutputWithMetadata, 0)
			outputChunkCounter = 0
		}
	}

	// write epochDiffs
	for i := header.FullEpochIndex + 1; i <= header.DiffEpochIndex; i++ {
		epochDiffs := epochDiffsProd()
		writeEpochDiffs(writeSeeker, epochDiffs)
	}

	// seek back to the file position of the outputWithMetadata counter
	if _, err := writeSeeker.Seek(0, io.SeekStart); err != nil {
		return nil, errors.Errorf("unable to seek to LS counter placeholders: %w", err)
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
	prodChan := make(chan *ledger.EpochDiff)
	stopChan := make(chan struct{})
	nmgr.SnapshotEpochDiffs(fullEpochIndex, latestCommitableEpoch, prodChan, stopChan)

	return func() *ledger.EpochDiff {
		select {
		case obj := <-prodChan:
			return obj
		case <-stopChan:
			close(prodChan)
			return nil
		}
	}
}

func writeEpochDiffs(writeSeeker io.WriteSeeker, diffs *ledger.EpochDiff) error {
	writeFunc := func(name string, value any) error {
		return writeFunc(writeSeeker, name, value)
	}

	spentLen := len(diffs.Spent())
	if err := writeFunc("epochDiffs spent Len", int64(spentLen)); err != nil {
		return err
	}

	s := diffs.Spent()
	var end int
	for i := 0; i < spentLen; {
		if i+chunkSize > spentLen {
			end = spentLen
		} else {
			end = i + chunkSize
		}
		writeOutputsWithMetadatas(writeSeeker, s[i:end])
		i = end
	}

	createdLen := len(diffs.Created())
	if err := writeFunc("epochDiffs created Len", int64(createdLen)); err != nil {
		return err
	}
	c := diffs.Created()
	for i := 0; i < createdLen; {
		if i+chunkSize > createdLen {
			end = createdLen
		} else {
			end = i + chunkSize
		}
		writeOutputsWithMetadatas(writeSeeker, c[i:end])
		i = end
	}

	return nil
}

func writeSolidEntryPoints(writeSeeker io.WriteSeeker, seps *SolidEntryPoints) error {
	writeFunc := func(name string, value any) error {
		return writeFunc(writeSeeker, name, value)
	}

	// write EI
	if err := writeFunc("solid entry points epoch", seps.EI); err != nil {
		return err
	}

	// write number of solid entry points
	sepsLen := len(seps.Seps)
	if err := writeFunc("solid entry points Len", int64(sepsLen)); err != nil {
		return err
	}

	// write solid entry points in chunks
	s := seps.Seps
	var end int
	for i := 0; i < sepsLen; {
		if i+chunkSize > sepsLen {
			end = sepsLen
		} else {
			end = i + chunkSize
		}

		data, err := serix.DefaultAPI.Encode(context.Background(), s[i:end], serix.WithValidation())
		if err != nil {
			return err
		}

		if err := writeFunc("sepsBytesLen", int64(len(data))); err != nil {
			return err
		}
		if err := writeFunc("seps", data); err != nil {
			return err
		}

		i = end
	}

	return nil
}

func writeOutputsWithMetadatas(writeSeeker io.WriteSeeker, outputsChunks []*ledger.OutputWithMetadata) error {
	if len(outputsChunks) == 0 {
		return nil
	}

	writeFunc := func(name string, value any) error {
		return writeFunc(writeSeeker, name, value)
	}

	data, err := serix.DefaultAPI.Encode(context.Background(), outputsChunks, serix.WithValidation())
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
		return errors.Errorf("unable to determine length of %s", variableName)
	}

	if err := binary.Write(writeSeeker, binary.LittleEndian, value); err != nil {
		return errors.Errorf("unable to write LS %s: %w", variableName, err)
	}

	return nil
}
