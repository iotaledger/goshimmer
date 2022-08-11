package snapshot

import (
	"bufio"
	"context"
	"encoding/binary"
	"fmt"
	"io"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/core/serix"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/ledger"
)

// streamSnapshotDataFrom consumes a snapshot from the given reader.
func streamSnapshotDataFrom(
	reader io.ReadSeeker,
	headerConsumer HeaderConsumerFunc,
	sepsConsumer SolidEntryPointsConsumerFunc,
	outputConsumer UTXOStatesConsumerFunc,
	epochDiffsConsumer EpochDiffsConsumerFunc) error {

	header, err := readSnapshotHeader(reader)
	if err != nil {
		return err
	}
	headerConsumer(header)

	// read solid entry points
	for i := header.FullEpochIndex; i <= header.DiffEpochIndex; i++ {
		seps, err := readSolidEntryPoints(reader)
		if err != nil {
			return err
		}
		sepsConsumer(seps)
	}

	// read outputWithMetadata
	for i := 0; uint64(i) < header.OutputWithMetadataCount; {
		outputs, err := readOutputWithMetadata(reader)
		if err != nil {
			return err
		}
		i += len(outputs)
		outputConsumer(outputs)
	}

	// read epochDiffs
	for i := header.FullEpochIndex + 1; i <= header.DiffEpochIndex; i++ {
		epochDiffs, err := readEpochDiffs(reader)
		if err != nil {
			return errors.Errorf("failed to parse epochDiffs from bytes: %w", err)
		}
		epochDiffsConsumer(i, epochDiffs)
	}

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

	var latestECRecordLen int64
	if err := binary.Read(reader, binary.LittleEndian, &latestECRecordLen); err != nil {
		return nil, fmt.Errorf("unable to read latest ECRecord bytes len: %w", err)
	}

	ecRecordBytes := make([]byte, latestECRecordLen)
	if err := binary.Read(reader, binary.LittleEndian, ecRecordBytes); err != nil {
		return nil, fmt.Errorf("unable to read latest ECRecord: %w", err)
	}
	header.LatestECRecord = &epoch.ECRecord{}
	if err := header.LatestECRecord.FromBytes(ecRecordBytes); err != nil {
		return nil, err
	}

	return header, nil
}

func readSolidEntryPoints(reader io.ReadSeeker) (seps *SolidEntryPoints, err error) {
	var sepsLen int64
	if err := binary.Read(reader, binary.LittleEndian, &sepsLen); err != nil {
		return nil, fmt.Errorf("unable to read seps bytes len: %w", err)
	}

	sepsBytes := make([]byte, sepsLen)
	if err := binary.Read(reader, binary.LittleEndian, sepsBytes); err != nil {
		return nil, fmt.Errorf("unable to read solid entry points: %w", err)
	}

	seps = &SolidEntryPoints{}
	if _, err = serix.DefaultAPI.Decode(context.Background(), sepsBytes, &seps, serix.WithValidation()); err != nil {
		return nil, err
	}

	return seps, nil
}

// readOutputWithMetadata consumes a slice of OutputWithMetadata from the given reader.
func readOutputWithMetadata(reader io.ReadSeeker) (outputMetadatas []*ledger.OutputWithMetadata, err error) {
	var outputsLen int64
	if err := binary.Read(reader, binary.LittleEndian, &outputsLen); err != nil {
		return nil, fmt.Errorf("unable to read outputsWithMetadata bytes len: %w", err)
	}

	outputsBytes := make([]byte, outputsLen)
	if err := binary.Read(reader, binary.LittleEndian, outputsBytes); err != nil {
		return nil, fmt.Errorf("unable to read outputsWithMetadata: %w", err)
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

// readEpochDiffs consumes a map of EpochDiff from the given reader.
func readEpochDiffs(reader io.ReadSeeker) (epochDiffs *ledger.EpochDiff, err error) {
	var epochDiffsLen int64
	if err := binary.Read(reader, binary.LittleEndian, &epochDiffsLen); err != nil {
		return nil, fmt.Errorf("unable to read epochDiffs bytes len: %w", err)
	}
	fmt.Println(">>> epoch diff len:", epochDiffsLen)
	epochDiffsBytes := make([]byte, epochDiffsLen)
	if err := binary.Read(reader, binary.LittleEndian, epochDiffsBytes); err != nil {
		return nil, fmt.Errorf("unable to read epochDiffs: %w", err)
	}

	_, err = serix.DefaultAPI.Decode(context.Background(), epochDiffsBytes, &epochDiffs, serix.WithValidation())
	if err != nil {
		return nil, errors.Errorf("failed to parse epochDiffs from bytes: %w", err)
	}

	for _, spentOutput := range epochDiffs.Spent() {
		spentOutput.SetID(spentOutput.M.OutputID)
		spentOutput.Output().SetID(spentOutput.M.OutputID)
	}
	for _, createdOutput := range epochDiffs.Created() {
		createdOutput.SetID(createdOutput.M.OutputID)
		createdOutput.Output().SetID(createdOutput.M.OutputID)
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
