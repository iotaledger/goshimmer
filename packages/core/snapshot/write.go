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
	epochDiffsProd EpochDiffProducerFunc,
	activityLogProd ActivityLogProducerFunc) (*ledger.SnapshotHeader, error) {

	writeFunc := func(name string, value any) error {
		return writeFunc(writeSeeker, name, value)
	}

	header, err := headerProd()
	if err != nil {
		return nil, err
	}

	err = writeSnapshotHeader(writeSeeker, header)
	if err != nil {
		return nil, errors.Wrap(err, "failed to write snapshot header to snapshot")
	}

	// write solid entry points
	for i := header.FullEpochIndex; i <= header.DiffEpochIndex; i++ {
		seps := sepsProd()
		if err := writeSolidEntryPoints(writeSeeker, seps); err != nil {
			return nil, errors.Wrap(err, "failed to write solid entry points to snapshot")
		}
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
				return nil, errors.Wrapf(err, "failed to write outputs metadata to snapshot")
			}
			break
		}
		outputWithMetadataCounter++
		outputChunkCounter++
		chunksOutputWithMetadata = append(chunksOutputWithMetadata, output)

		// put a delimiter every chunkSize outputs
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
		if epochErr := writeEpochDiffs(writeSeeker, epochDiffs); epochErr != nil {
			return nil, errors.Wrap(epochErr, "failed to write epochDiffs to snapshot")
		}
	}

	// write active nodes
	activeNodes := activityLogProd()
	if actErr := writeActivityLog(writeSeeker, activeNodes); actErr != nil {
		return nil, errors.Wrap(actErr, "failed to write activity log to snapshot")
	}

	// seek back to the file position of the outputWithMetadata counter
	if _, err := writeSeeker.Seek(0, io.SeekStart); err != nil {
		return nil, errors.Errorf("unable to seek to LS counter placeholders: %w", err)
	}
	if err = writeFunc(fmt.Sprintf("outputWithMetadata counter %d", outputWithMetadataCounter), outputWithMetadataCounter); err != nil {
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
	writeFuncWrap := func(name string, value any) error {
		return writeFunc(writeSeeker, name, value)
	}

	spentLen := len(diffs.Spent())
	if err := writeFuncWrap("epochDiffs spent Len", int64(spentLen)); err != nil {
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
		if err := writeOutputsWithMetadatas(writeSeeker, s[i:end]); err != nil {
			return errors.Wrap(err, "unable to write output with metadata to snapshot")
		}
		i = end
	}

	createdLen := len(diffs.Created())
	if err := writeFuncWrap("epochDiffs created Len", int64(createdLen)); err != nil {
		return err
	}
	c := diffs.Created()
	for i := 0; i < createdLen; {
		if i+chunkSize > createdLen {
			end = createdLen
		} else {
			end = i + chunkSize
		}
		if err := writeOutputsWithMetadatas(writeSeeker, c[i:end]); err != nil {
			return errors.Wrap(err, "unable to write output with metadata to snapshot")
		}
		i = end
	}

	return nil
}

func writeActivityLog(writeSeeker io.WriteSeeker, activityLog epoch.SnapshotEpochActivity) error {
	writeFuncWrap := func(name string, value any) error {
		return writeFunc(writeSeeker, name, value)
	}
	// write activityLog length
	activityLen := len(activityLog)
	if err := writeFuncWrap("activity log len", int64(activityLen)); err != nil {
		return err
	}

	for ei, al := range activityLog {
		// write epoch index
		if err := writeFuncWrap("epoch index", ei); err != nil {
			return err
		}
		// write activity log
		alBytes, err := al.Bytes()
		if err != nil {
			return err
		}

		if err := writeFuncWrap("activity log bytes len", int64(len(alBytes))); err != nil {
			return err
		}

		if err := writeFuncWrap("activity log", alBytes); err != nil {
			return err
		}
	}
	return nil
}

func writeSolidEntryPoints(writeSeeker io.WriteSeeker, seps *SolidEntryPoints) error {
	writeFuncWrap := func(name string, value any) error {
		return writeFunc(writeSeeker, name, value)
	}

	// write EI
	if err := writeFuncWrap("solid entry points epoch", seps.EI); err != nil {
		return err
	}

	// write number of solid entry points
	sepsLen := len(seps.Seps)
	if err := writeFuncWrap("solid entry points Len", int64(sepsLen)); err != nil {
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

		if err := writeFuncWrap("sepsBytesLen", int64(len(data))); err != nil {
			return err
		}
		if err := writeFuncWrap("seps", data); err != nil {
			return err
		}

		i = end
	}
	return nil
}

// NewActivityLogProducer returns an ActivityLogProducerFunc that provides activity log from weightProvider and notarization manager.
func NewActivityLogProducer(notarizationMgr *notarization.Manager, epochDiffIndex epoch.Index) ActivityLogProducerFunc {
	activityLog, err := notarizationMgr.SnapshotEpochActivity(epochDiffIndex)
	if err != nil {
		panic(err)
	}
	return func() (activityLogs epoch.SnapshotEpochActivity) {
		return activityLog
	}
}

func writeOutputsWithMetadatas(writeSeeker io.WriteSeeker, outputsChunks []*ledger.OutputWithMetadata) error {
	if len(outputsChunks) == 0 {
		return nil
	}

	writeFuncWrap := func(name string, value any) error {
		return writeFunc(writeSeeker, name, value)
	}

	data, err := serix.DefaultAPI.Encode(context.Background(), outputsChunks, serix.WithValidation())
	if err != nil {
		return err
	}
	if err := writeFuncWrap("outputsBytesLen", int64(len(data))); err != nil {
		return err
	}
	if err := writeFuncWrap("outputs", data); err != nil {
		return err
	}
	return nil
}

func writeSnapshotHeader(writeSeeker io.WriteSeeker, header *ledger.SnapshotHeader) error {
	writeFuncWrap := func(name string, value any) error {
		return writeFunc(writeSeeker, name, value)
	}

	if err := writeFuncWrap(fmt.Sprintf("outputWithMetadata counter %d", header.OutputWithMetadataCount), header.OutputWithMetadataCount); err != nil {
		return err
	}

	if err := writeFuncWrap(fmt.Sprintf("fullEpochIndex %d", header.FullEpochIndex), header.FullEpochIndex); err != nil {
		return err
	}

	if err := writeFuncWrap(fmt.Sprintf("diffEpochIndex %d", header.DiffEpochIndex), header.DiffEpochIndex); err != nil {
		return err
	}

	data, err := header.LatestECRecord.Bytes()
	if err != nil {
		return err
	}

	if latestLenErr := writeFuncWrap("latestECRecordBytesLen", int64(len(data))); latestLenErr != nil {
		return latestLenErr
	}

	if latestErr := writeFuncWrap("latestECRecord", data); err != nil {
		return latestErr
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
