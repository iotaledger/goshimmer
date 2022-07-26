package snapshot

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/ledger"
	"github.com/iotaledger/hive.go/serix"
)

var delimiter = []byte{';', ';'}

// StreamSnapshotDataTo writes snapshot to a given writer.
func StreamSnapshotDataTo(
	writeSeeker io.WriteSeeker,
	header *ledger.SnapshotHeader,
	outputProd OutputWithMetadataProducerFunc,
	epochDiffsProd EpochDiffProducerFunc) error {

	writeFunc := func(name string, value any, offsetsToIncrease ...*int64) error {
		return writeFunc(writeSeeker, name, value, offsetsToIncrease...)
	}

	writeOutputWithMetadatasFunc := func(chunks []*ledger.OutputWithMetadata) error {
		if len(chunks) == 0 {
			return nil
		}

		typeSet := new(serix.TypeSettings)
		data, err := serix.DefaultAPI.Encode(context.Background(), chunks, serix.WithTypeSettings(typeSet.WithLengthPrefixType(serix.LengthPrefixTypeAsUint32)), serix.WithValidation())
		if err != nil {
			return err
		}
		if err := writeFunc("outputs", append(data, delimiter...)); err != nil {
			return err
		}
		return nil
	}

	err := writeSnapshotHeader(writeSeeker, header)
	if err != nil {
		return err
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
				return err
			}
			break
		}

		outputWithMetadataCounter++
		chunksOutputWithMetadata = append(chunksOutputWithMetadata, output)

		// put a delimeter every 100 outputs
		if outputChunkCounter == 100 {
			err = writeOutputWithMetadatasFunc(chunksOutputWithMetadata)
			if err != nil {
				return err
			}
			chunksOutputWithMetadata = make([]*ledger.OutputWithMetadata, 0)
			outputChunkCounter = 0
		}
	}

	// write epochDiffs
	epochDiffs, err := epochDiffsProd()
	if err != nil {
		return err
	}

	typeSet := new(serix.TypeSettings)
	bytes, err := serix.DefaultAPI.Encode(context.Background(), epochDiffs, serix.WithTypeSettings(typeSet.WithLengthPrefixType(serix.LengthPrefixTypeAsUint32)), serix.WithValidation())
	if err != nil {
		return err
	}
	if err := writeFunc(fmt.Sprintf("diffEpoch"), append(bytes, delimiter...)); err != nil {
		return err
	}

	// seek back to the file position of the outputWithMetadata counter
	if _, err := writeSeeker.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("unable to seek to LS counter placeholders: %w", err)
	}
	if err := writeFunc(fmt.Sprintf("outputWithMetadata counter %d", outputWithMetadataCounter), outputWithMetadataCounter); err != nil {
		return err
	}
	header.OutputWithMetadataCount = outputWithMetadataCounter

	return nil
}

// NewLedgerOutputWithMetadataProducer returns a OutputWithMetadataProducerFunc that provide OutputWithMetadatas from the ledger.
func NewLedgerOutputWithMetadataProducer(lastConfirmedEpoch epoch.Index, l *ledger.Ledger) OutputWithMetadataProducerFunc {
	prodChan := make(chan *ledger.OutputWithMetadata)

	go func() {
		l.ForEachAcceptedUnspentOutputWithMetadata(func(o *ledger.OutputWithMetadata) {
			index := epoch.IndexFromTime(o.CreationTime())
			if index <= lastConfirmedEpoch {
				prodChan <- o
			}
		})

		close(prodChan)
	}()

	return func() *ledger.OutputWithMetadata {
		obj, ok := <-prodChan
		if !ok {
			return nil
		}
		return obj
	}
}

func writeSnapshotHeader(writeSeeker io.WriteSeeker, header *ledger.SnapshotHeader) error {
	writeFunc := func(name string, value any, offsetsToIncrease ...*int64) error {
		return writeFunc(writeSeeker, name, value, offsetsToIncrease...)
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

func increaseOffsets(amount int64, offsets ...*int64) {
	for _, offset := range offsets {
		*offset += amount
	}
}

func writeFunc(writeSeeker io.WriteSeeker, variableName string, value any, offsetsToIncrease ...*int64) error {
	length := binary.Size(value)
	if length == -1 {
		return fmt.Errorf("unable to determine length of %s", variableName)
	}

	if err := binary.Write(writeSeeker, binary.LittleEndian, value); err != nil {
		return fmt.Errorf("unable to write LS %s: %w", variableName, err)
	}

	increaseOffsets(int64(length), offsetsToIncrease...)

	return nil
}
