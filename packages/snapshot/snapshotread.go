package snapshot

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/goshimmer/packages/epoch"
	"github.com/iotaledger/goshimmer/packages/ledger"
	"github.com/iotaledger/hive.go/serix"
)

// StreamSnapshotDataFrom consumes a full snapshot from the given reader.
func StreamSnapshotDataFrom(
	reader io.ReadSeeker,
	outputConsumer OutputConsumerFunc,
	epochDiffsConsumer EpochDiffsConsumerFunc,
	notarizationConsumer NotarizationConsumerFunc) error {

	var outputWithMetadataCounter uint64
	if err := binary.Read(reader, binary.LittleEndian, &outputWithMetadataCounter); err != nil {
		return fmt.Errorf("unable to read outputWithMetadata length: %w", err)
	}

	var fullEpochIndex int64
	if err := binary.Read(reader, binary.LittleEndian, &fullEpochIndex); err != nil {
		return fmt.Errorf("unable to read fullEpochIndex: %w", err)
	}

	var diffEpochIndex int64
	if err := binary.Read(reader, binary.LittleEndian, &diffEpochIndex); err != nil {
		return fmt.Errorf("unable to read diffEpochIndex: %w", err)
	}

	scanner := bufio.NewScanner(reader)
	scanner.Split(scanDelimiter)
	ecRecord, err := ReadECRecord(scanner)
	if err != nil {
		return err
	}
	notarizationConsumer(epoch.Index(fullEpochIndex), epoch.Index(diffEpochIndex), ecRecord)

	epochDiffs := make(map[epoch.Index]*ledger.EpochDiff)
	epochDiffs, err = ReadEpochDiffs(scanner)
	if err != nil {
		return errors.Errorf("failed to parse epochDiffs from bytes: %w", err)
	}

	// read outputWithMetadata
	for i := 0; uint64(i) < outputWithMetadataCounter; {
		outputs, err := ReadOutputWithMetadata(scanner)
		if err != nil {
			return err
		}
		i += len(outputs)

		outputConsumer(outputs)
	}

	epochDiffsConsumer(epoch.Index(fullEpochIndex), epoch.Index(diffEpochIndex), epochDiffs)

	return nil
}

func ReadOutputWithMetadata(scanner *bufio.Scanner) (outputMetadatas []*ledger.OutputWithMetadata, err error) {
	scanner.Scan()
	data := scanner.Bytes()

	if len(data) > 0 {
		typeSet := new(serix.TypeSettings)
		outputMetadatas = make([]*ledger.OutputWithMetadata, 0)
		_, err = serix.DefaultAPI.Decode(context.Background(), data, &outputMetadatas, serix.WithTypeSettings(typeSet.WithLengthPrefixType(serix.LengthPrefixTypeAsUint32)))
		if err != nil {
			return nil, err
		}
	}

	return
}

func ReadEpochDiffs(scanner *bufio.Scanner) (epochDiffs map[epoch.Index]*ledger.EpochDiff, err error) {
	epochDiffs = make(map[epoch.Index]*ledger.EpochDiff)

	scanner.Scan()
	data := scanner.Bytes()
	if len(data) > 0 {
		typeSet := new(serix.TypeSettings)
		_, err = serix.DefaultAPI.Decode(context.Background(), data, &epochDiffs, serix.WithTypeSettings(typeSet.WithLengthPrefixType(serix.LengthPrefixTypeAsUint32)))
		if err != nil {
			return nil, errors.Errorf("failed to parse epochDiffs from bytes: %w", err)
		}
	}

	return
}

func ReadECRecord(scanner *bufio.Scanner) (ecRecord *epoch.ECRecord, err error) {
	scanner.Scan()

	ecRecord = &epoch.ECRecord{}
	_, err = serix.DefaultAPI.Decode(context.Background(), scanner.Bytes(), ecRecord)
	if err != nil {
		return nil, errors.Errorf("failed to parse epochDiffs from bytes: %w", err)
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
