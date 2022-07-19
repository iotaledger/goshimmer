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
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/serix"
	"golang.org/x/crypto/blake2b"
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

	ecRecord, err := ReadECRecord(reader)
	if err != nil {
		return err
	}
	notarizationConsumer(epoch.Index(fullEpochIndex), epoch.Index(diffEpochIndex), ecRecord)

	chunkReader := bufio.NewReader(reader)
	epochDiffs := make(map[epoch.Index]*ledger.EpochDiff)
	bytes, err := chunkReader.ReadBytes(byte(';'))
	if err != nil {
		return err
	}

	_, err = serix.DefaultAPI.Decode(context.Background(), bytes, epochDiffs)
	if err != nil {
		return errors.Errorf("failed to parse epochDiffs from bytes: %w", err)
	}

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
	chunk, err := reader.ReadBytes(byte(';'))
	if err != nil {
		return nil, err
	}

	_, err = serix.DefaultAPI.Decode(context.Background(), chunk, outputMetadatas)
	if err != nil {
		return nil, err
	}

	return
}

func ReadECRecord(reader io.ReadSeeker) (ecRecord *epoch.ECRecord, err error) {
	var size = blake2b.Size256*2 + marshalutil.Int64Size

	ecRecordBytes := make([]byte, size)
	if _, err := io.ReadFull(reader, ecRecordBytes); err != nil {
		return nil, fmt.Errorf("unable to read LS ECRecord: %w", err)
	}

	ecRecord = &epoch.ECRecord{}
	err = ecRecord.FromBytes(ecRecordBytes)

	return
}
