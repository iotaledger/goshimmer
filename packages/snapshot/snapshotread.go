package snapshot

import (
	"bufio"
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

	chunkReader := bufio.NewReader(reader)
	ecRecord, err := ReadECRecord(chunkReader)
	if err != nil {
		return err
	}
	notarizationConsumer(epoch.Index(fullEpochIndex), epoch.Index(diffEpochIndex), ecRecord)

	epochDiffs := make(map[epoch.Index]*ledger.EpochDiff)
	epochDiffs, err = ReadEpochDiffs(chunkReader)
	if err != nil {
		return errors.Errorf("failed to parse epochDiffs from bytes: %w", err)
	}

	// read outputWithMetadata
	for i := 0; uint64(i) < outputWithMetadataCounter; {
		outputs, err := ReadOutputWithMetadata(chunkReader)
		if err != nil {
			return err
		}
		i += len(outputs)

		outputConsumer(outputs)
	}

	epochDiffsConsumer(epoch.Index(fullEpochIndex), epoch.Index(diffEpochIndex), epochDiffs)

	return nil
}

func ReadOutputWithMetadata(reader *bufio.Reader) (outputMetadatas []*ledger.OutputWithMetadata, err error) {
	chunk, err := reader.ReadBytes('\n')
	if err != nil {
		return nil, err
	}

	if len(chunk) > 0 {
		typeSet := new(serix.TypeSettings)
		outputMetadatas = make([]*ledger.OutputWithMetadata, 0)
		_, err = serix.DefaultAPI.Decode(context.Background(), chunk, &outputMetadatas, serix.WithTypeSettings(typeSet.WithLengthPrefixType(serix.LengthPrefixTypeAsUint32)))
		if err != nil {
			return nil, err
		}
	}

	return
}

func ReadEpochDiffs(reader *bufio.Reader) (epochDiffs map[epoch.Index]*ledger.EpochDiff, err error) {
	epochDiffs = make(map[epoch.Index]*ledger.EpochDiff)

	data, err := reader.ReadBytes('\n')
	if err != nil {
		return nil, err
	}

	if len(data) > 0 {
		typeSet := new(serix.TypeSettings)
		_, err = serix.DefaultAPI.Decode(context.Background(), data, &epochDiffs, serix.WithTypeSettings(typeSet.WithLengthPrefixType(serix.LengthPrefixTypeAsUint32)))
		if err != nil {
			return nil, errors.Errorf("failed to parse epochDiffs from bytes: %w", err)
		}
	}

	return
}

func ReadECRecord(reader *bufio.Reader) (ecRecord *epoch.ECRecord, err error) {
	data, err := reader.ReadBytes('\n')
	if err != nil {
		return nil, err
	}
	ecRecord = &epoch.ECRecord{}
	_, err = serix.DefaultAPI.Decode(context.Background(), data, ecRecord)
	if err != nil {
		return nil, errors.Errorf("failed to parse epochDiffs from bytes: %w", err)
	}

	return
}
