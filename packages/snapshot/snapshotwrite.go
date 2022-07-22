package snapshot

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"

	"github.com/iotaledger/goshimmer/packages/epoch"
	"github.com/iotaledger/goshimmer/packages/ledger"
	"github.com/iotaledger/hive.go/serix"
)

var delimiter = []byte{';', ';'}

// StreamSnapshotDataTo writes snapshot to a given writer.
func StreamSnapshotDataTo(
	writeSeeker io.WriteSeeker,
	outputProd OutputProducerFunc,
	fullEpochIndex, diffEpochIndex epoch.Index,
	latestECRecord *epoch.ECRecord,
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

	var outputWithMetadataCounter uint64 = 0
	if err := writeFunc(fmt.Sprintf("outputWithMetadata counter %d", outputWithMetadataCounter), outputWithMetadataCounter); err != nil {
		return err
	}

	if err := writeFunc(fmt.Sprintf("fullEpochIndex %d", fullEpochIndex), fullEpochIndex); err != nil {
		return err
	}

	if err := writeFunc(fmt.Sprintf("diffEpochIndex %d", diffEpochIndex), diffEpochIndex); err != nil {
		return err
	}

	data, err := serix.DefaultAPI.Encode(context.Background(), latestECRecord, serix.WithValidation())
	if err != nil {
		return err
	}

	if err := writeFunc("latestECRecord", append(data, delimiter...)); err != nil {
		return err
	}

	// write outputWithMetadata
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

	return nil
}

func NewUTXOOutputProducer(l *ledger.Ledger) OutputProducerFunc {
	prodChan := make(chan interface{})

	go func() {
		l.ForEachOutputWithMetadata(func(o *ledger.OutputWithMetadata) {
			prodChan <- o
		})

		close(prodChan)
	}()

	binder := producerFromChannels(prodChan)
	return func() *ledger.OutputWithMetadata {
		obj := binder()
		if obj == nil {
			return nil
		}
		return obj.(*ledger.OutputWithMetadata)
	}
}

// returns a function which tries to read from the given producer and error channels up on each invocation.
func producerFromChannels(prodChan <-chan interface{}) func() interface{} {
	return func() interface{} {
		select {
		case obj, ok := <-prodChan:
			if !ok {
				return nil
			}
			return obj
		}
	}
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
