package snapshot

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/ledger"
	"github.com/iotaledger/hive.go/serix"
)

// streamSnapshotDataFrom consumes a snapshot from the given reader.
func streamSnapshotDataFrom(reader io.ReadSeeker, headerConsumer HeaderConsumerFunc, outputConsumer UTXOStatesConsumerFunc, epochDiffsConsumer EpochDiffsConsumerFunc, activityLogConsumer ActivityLogConsumerFunc) error {

	header, err := readSnapshotHeader(reader)
	if err != nil {
		return err
	}

	scanner := bufio.NewScanner(reader)
	scanner.Split(scanDelimiter)

	// read latest ECRecord
	ecRecord, err := readECRecord(scanner)
	if err != nil {
		return err
	}
	header.LatestECRecord = ecRecord
	headerConsumer(header)

	// read outputWithMetadata
	for i := 0; uint64(i) < header.OutputWithMetadataCount; {
		outputs, err := readOutputWithMetadata(scanner)
		if err != nil {
			return err
		}
		i += len(outputs)

		outputConsumer(outputs)
	}

	epochDiffs, err := readEpochDiffs(scanner)
	if err != nil {
		return errors.Errorf("failed to parse epochDiffs from bytes: %w", err)
	}
	epochDiffsConsumer(header, epochDiffs)
	//
	//activityLog, err := readActivityLog(scanner)
	//activityLogConsumer(activityLog)

	return nil
}

func readSnapshotHeader(reader io.ReadSeeker) (*ledger.SnapshotHeader, error) {
	header := &ledger.SnapshotHeader{}

	if err := binary.Read(reader, binary.LittleEndian, &header.OutputWithMetadataCount); err != nil {
		return nil, fmt.Errorf("unable to read outputWithMetadata length: %w", err)
	}

	var index int64
	if err := binary.Read(reader, binary.LittleEndian, &index); err != nil {
		return nil, fmt.Errorf("unable to read fullEpochIndex: %w", err)
	}
	header.FullEpochIndex = epoch.Index(index)

	if err := binary.Read(reader, binary.LittleEndian, &index); err != nil {
		return nil, fmt.Errorf("unable to read diffEpochIndex: %w", err)
	}
	header.DiffEpochIndex = epoch.Index(index)

	return header, nil
}

// readOutputWithMetadata consumes a slice of OutputWithMetadata from the given reader.
func readOutputWithMetadata(scanner *bufio.Scanner) (outputMetadatas []*ledger.OutputWithMetadata, err error) {
	scanner.Scan()
	data := scanner.Bytes()

	if len(data) > 0 {
		outputMetadatas = make([]*ledger.OutputWithMetadata, 0)
		_, err = serix.DefaultAPI.Decode(context.Background(), data, &outputMetadatas, serix.WithValidation())
		if err != nil {
			return nil, err
		}

		for _, o := range outputMetadatas {
			o.SetID(o.M.OutputID)
			o.Output().SetID(o.M.OutputID)
		}
	}

	return
}

// readEpochDiffs consumes a map of EpochDiff from the given reader.
func readEpochDiffs(scanner *bufio.Scanner) (epochDiffs map[epoch.Index]*ledger.EpochDiff, err error) {
	epochDiffs = make(map[epoch.Index]*ledger.EpochDiff)

	scanner.Scan()
	data := scanner.Bytes()
	if len(data) > 0 {
		_, err = serix.DefaultAPI.Decode(context.Background(), data, &epochDiffs, serix.WithValidation())
		if err != nil {
			return nil, errors.Errorf("failed to parse epochDiffs from bytes: %w", err)
		}

		for _, epochdiff := range epochDiffs {
			for _, spentOutput := range epochdiff.Spent() {
				spentOutput.SetID(spentOutput.M.OutputID)
				spentOutput.Output().SetID(spentOutput.M.OutputID)
			}
			for _, createdOutput := range epochdiff.Created() {
				createdOutput.SetID(createdOutput.M.OutputID)
				createdOutput.Output().SetID(createdOutput.M.OutputID)
			}
		}
	}

	return
}

// readECRecord consumes the latest ECRecord from the given reader.
func readECRecord(scanner *bufio.Scanner) (ecRecord *epoch.ECRecord, err error) {
	scanner.Scan()

	ecRecord = &epoch.ECRecord{}
	err = ecRecord.FromBytes(scanner.Bytes())
	if err != nil {
		return nil, errors.Errorf("failed to parse epochDiffs from bytes: %w", err)
	}

	return
}

// readActivityLog consumes the ActivityLog from the given reader.
func readActivityLog(scanner *bufio.Scanner) (activityLogs epoch.NodesActivityLog, err error) {
	activityLogs = make(epoch.NodesActivityLog)

	scanner.Scan()
	data := scanner.Bytes()

	if len(data) > 0 {
		_, err = serix.DefaultAPI.Decode(context.Background(), data, &activityLogs, serix.WithValidation())
		if err != nil {
			return nil, errors.Errorf("failed to parse activityLog from bytes: %w", err)
		}
	}
	return
}

func scanDelimiter(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	if i := bytes.Index(data, delimiter); i >= 0 {
		return i + 2, data[0:i], nil
	}
	// at EOF, return rest of data.
	if atEOF {
		return len(data), data, nil
	}

	return 0, nil, nil
}
