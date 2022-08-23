package snapshot

import (
	"bufio"
	"context"
	"encoding/binary"
	"io"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/ledger"
	"github.com/iotaledger/goshimmer/packages/core/tangleold"
	"github.com/iotaledger/hive.go/core/serix"
)

// streamSnapshotDataFrom consumes a snapshot from the given reader.
func streamSnapshotDataFrom(
	reader io.ReadSeeker,
	headerConsumer HeaderConsumerFunc,
	sepsConsumer SolidEntryPointsConsumerFunc,
	outputConsumer UTXOStatesConsumerFunc,
	epochDiffsConsumer EpochDiffsConsumerFunc,
	activityLogConsumer ActivityLogConsumerFunc) error {

	header, err := readSnapshotHeader(reader)
	if err != nil {
		return errors.Wrap(err, "failed to stream snapshot header from snapshot")
	}
	headerConsumer(header)

	// read solid entry points
	for i := header.FullEpochIndex; i <= header.DiffEpochIndex; i++ {
		seps, solidErr := readSolidEntryPoints(reader)
		if solidErr != nil {
			return errors.Wrap(solidErr, "failed to stream solid entry points from snapshot")
		}
		sepsConsumer(seps)
	}

	// read outputWithMetadata
	for i := 0; uint64(i) < header.OutputWithMetadataCount; {
		outputs, outErr := readOutputsWithMetadatas(reader)
		if outErr != nil {
			return errors.Wrap(outErr, "failed to stream output with metadata from snapshot")
		}
		i += len(outputs)
		outputConsumer(outputs)
	}

	// read epochDiffs
	for ei := header.FullEpochIndex + 1; ei <= header.DiffEpochIndex; ei++ {
		epochDiffs, epochErr := readEpochDiffs(reader)
		if epochErr != nil {
			return errors.Wrapf(epochErr, "failed to parse epochDiffs from bytes")
		}
		epochDiffsConsumer(epochDiffs)
	}

	activityLog, err := readActivityLog(reader)
	if err != nil {
		return errors.Wrap(err, "failed to parse activity log from bytes")
	}
	activityLogConsumer(activityLog)

	return nil
}

func readSnapshotHeader(reader io.ReadSeeker) (*ledger.SnapshotHeader, error) {
	header := &ledger.SnapshotHeader{}

	if err := binary.Read(reader, binary.LittleEndian, &header.OutputWithMetadataCount); err != nil {
		return nil, errors.Wrap(err, "unable to read outputWithMetadata length")
	}

	var index int64
	if err := binary.Read(reader, binary.LittleEndian, &index); err != nil {
		return nil, errors.Wrap(err, "unable to read fullEpochIndex")
	}
	header.FullEpochIndex = epoch.Index(index)

	if err := binary.Read(reader, binary.LittleEndian, &index); err != nil {
		return nil, errors.Wrap(err, "unable to read diffEpochIndex")
	}
	header.DiffEpochIndex = epoch.Index(index)

	var latestECRecordLen int64
	if err := binary.Read(reader, binary.LittleEndian, &latestECRecordLen); err != nil {
		return nil, errors.Errorf("unable to read latest ECRecord bytes len: %w", err)
	}

	ecRecordBytes := make([]byte, latestECRecordLen)
	if err := binary.Read(reader, binary.LittleEndian, ecRecordBytes); err != nil {
		return nil, errors.Errorf("unable to read latest ECRecord: %w", err)
	}
	header.LatestECRecord = &epoch.ECRecord{}
	if err := header.LatestECRecord.FromBytes(ecRecordBytes); err != nil {
		return nil, err
	}

	return header, nil
}

func readSolidEntryPoints(reader io.ReadSeeker) (seps *SolidEntryPoints, err error) {
	seps = &SolidEntryPoints{}
	blkIDs := make([]tangleold.BlockID, 0)

	// read seps EI
	var index int64
	if err := binary.Read(reader, binary.LittleEndian, &index); err != nil {
		return nil, errors.Errorf("unable to read epoch index: %w", err)
	}
	seps.EI = epoch.Index(index)

	// read numbers of solid entry point
	var sepsLen int64
	if err := binary.Read(reader, binary.LittleEndian, &sepsLen); err != nil {
		return nil, errors.Errorf("unable to read seps len: %w", err)
	}

	for i := 0; i < int(sepsLen); {
		var sepsBytesLen int64
		if err := binary.Read(reader, binary.LittleEndian, &sepsBytesLen); err != nil {
			return nil, errors.Errorf("unable to read seps bytes len: %w", err)
		}

		sepsBytes := make([]byte, sepsBytesLen)
		if err := binary.Read(reader, binary.LittleEndian, sepsBytes); err != nil {
			return nil, errors.Errorf("unable to read solid entry points: %w", err)
		}

		ids := make([]tangleold.BlockID, 0)
		_, err = serix.DefaultAPI.Decode(context.Background(), sepsBytes, &ids, serix.WithValidation())
		if err != nil {
			return nil, err
		}
		blkIDs = append(blkIDs, ids...)
		i += len(ids)
	}

	seps.Seps = blkIDs

	return seps, nil
}

// readOutputsWithMetadatas consumes less or equal chunkSize of OutputWithMetadatas from the given reader.
func readOutputsWithMetadatas(reader io.ReadSeeker) (outputMetadatas []*ledger.OutputWithMetadata, err error) {
	var outputsLen int64
	if err := binary.Read(reader, binary.LittleEndian, &outputsLen); err != nil {
		return nil, errors.Errorf("unable to read outputsWithMetadata bytes len: %w", err)
	}

	outputsBytes := make([]byte, outputsLen)
	if err := binary.Read(reader, binary.LittleEndian, outputsBytes); err != nil {
		return nil, errors.Errorf("unable to read outputsWithMetadata: %w", err)
	}

	outputMetadatas = make([]*ledger.OutputWithMetadata, 0)
	_, err = serix.DefaultAPI.Decode(context.Background(), outputsBytes, &outputMetadatas, serix.WithValidation())
	if err != nil {
		return nil, err
	}

	for _, o := range outputMetadatas {
		o.SetID(o.M.OutputID)
		o.Output().SetID(o.M.OutputID)
	}

	return
}

// readEpochDiffs consumes an EpochDiff of an epoch from the given reader.
func readEpochDiffs(reader io.ReadSeeker) (epochDiffs *ledger.EpochDiff, err error) {
	spent := make([]*ledger.OutputWithMetadata, 0)
	created := make([]*ledger.OutputWithMetadata, 0)

	// read spent
	var spentLen int64
	if err := binary.Read(reader, binary.LittleEndian, &spentLen); err != nil {
		return nil, errors.Errorf("unable to read epochDiffs spent len: %w", err)
	}

	for i := 0; i < int(spentLen); {
		s, err := readOutputsWithMetadatas(reader)
		if err != nil {
			return nil, errors.Errorf("unable to read epochDiffs spent: %w", err)
		}
		spent = append(spent, s...)
		i += len(s)
	}

	// read created
	var createdLen int64
	if err := binary.Read(reader, binary.LittleEndian, &createdLen); err != nil {
		return nil, errors.Errorf("unable to read epochDiffs created len: %w", err)
	}

	for i := 0; i < int(createdLen); {
		c, err := readOutputsWithMetadatas(reader)
		if err != nil {
			return nil, errors.Errorf("unable to read epochDiffs created: %w", err)
		}
		created = append(created, c...)
		i += len(c)
	}

	epochDiffs = ledger.NewEpochDiff(spent, created)

	return
}

// readECRecord consumes the latest ECRecord from the given reader.
func readECRecord(scanner *bufio.Scanner) (ecRecord *epoch.ECRecord, err error) {
	scanner.Scan()

	ecRecord = &epoch.ECRecord{}
	err = ecRecord.FromBytes(scanner.Bytes())
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse epochDiffs from bytes")
	}

	return
}

// readActivityLog consumes the ActivityLog from the given reader.
func readActivityLog(reader io.ReadSeeker) (activityLogs epoch.SnapshotEpochActivity, err error) {
	var activityLen int64
	if lenErr := binary.Read(reader, binary.LittleEndian, &activityLen); lenErr != nil {
		return nil, errors.Wrap(lenErr, "unable to read activity len")
	}

	activityLogs = epoch.NewSnapshotEpochActivity()

	for i := 0; i < int(activityLen); i++ {
		var epochIndex epoch.Index
		if eiErr := binary.Read(reader, binary.LittleEndian, &epochIndex); eiErr != nil {
			return nil, errors.Errorf("unable to read epoch index: %w", eiErr)
		}
		var activityBytesLen int64
		if activityLenErr := binary.Read(reader, binary.LittleEndian, &activityBytesLen); activityLenErr != nil {
			return nil, errors.Errorf("unable to read activity log length: %w", activityLenErr)
		}
		activityLogBytes := make([]byte, activityBytesLen)
		if alErr := binary.Read(reader, binary.LittleEndian, activityLogBytes); alErr != nil {
			return nil, errors.Errorf("unable to read activity log: %w", alErr)
		}
		activityLog := new(epoch.SnapshotNodeActivity)
		activityLog.FromBytes(activityLogBytes)

		activityLogs[epochIndex] = activityLog
	}

	return
}
