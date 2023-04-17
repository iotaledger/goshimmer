package permanent

import (
	"encoding/binary"
	"io"

	"github.com/pkg/errors"

	"github.com/iotaledger/goshimmer/packages/core/commitment"
	"github.com/iotaledger/goshimmer/packages/core/storable"
	"github.com/iotaledger/hive.go/core/slot"
	"github.com/iotaledger/hive.go/lo"
	"github.com/iotaledger/hive.go/runtime/module"
)

type Commitments struct {
	slice    *storable.Slice[commitment.Commitment, *commitment.Commitment]
	filePath string

	module.Module
}

func NewCommitments(path string) (newCommitment *Commitments) {
	commitmentsLength, err := determineCommitmentLength()
	if err != nil {
		panic(errors.Wrapf(err, "failed to serialize empty commitment (to determine its length)"))
	}

	commitmentsSlice, err := storable.NewSlice[commitment.Commitment](path, commitmentsLength)
	if err != nil {
		panic(errors.Wrap(err, "failed to create commitments file"))
	}

	return &Commitments{
		slice:    commitmentsSlice,
		filePath: path,
	}
}

func (c *Commitments) Store(commitment *commitment.Commitment) (err error) {
	if err = c.slice.Set(int(commitment.Index()), commitment); err != nil {
		return errors.Wrapf(err, "failed to store commitment for slot %d", commitment.Index())
	}

	return nil
}

func (c *Commitments) Load(index slot.Index) (commitment *commitment.Commitment, err error) {
	if commitment, err = c.slice.Get(int(index)); err != nil {
		return nil, errors.Wrapf(err, "failed to get commitment for slot %d", index)
	}

	return commitment, nil
}

func (c *Commitments) Close() (err error) {
	return c.slice.Close()
}

// FilePath returns the path that this is associated to.
func (c *Commitments) FilePath() (filePath string) {
	return c.filePath
}

func (c *Commitments) Export(writer io.WriteSeeker, targetSlot slot.Index) (err error) {
	if err = binary.Write(writer, binary.LittleEndian, int64(targetSlot)); err != nil {
		return errors.Wrap(err, "failed to write slot boundary")
	}

	for slotIndex := slot.Index(0); slotIndex <= targetSlot; slotIndex++ {
		commitment, err := c.Load(slotIndex)
		if err != nil {
			return errors.Wrapf(err, "failed to load commitment for slot %d", slotIndex)
		}
		if err = binary.Write(writer, binary.LittleEndian, lo.PanicOnErr(commitment.Bytes())); err != nil {
			return errors.Wrapf(err, "failed to write commitment for slot %d", slotIndex)
		}
	}

	return nil
}

func (c *Commitments) Import(reader io.ReadSeeker) (err error) {
	var slotBoundary int64
	if err = binary.Read(reader, binary.LittleEndian, &slotBoundary); err != nil {
		return errors.Wrap(err, "failed to read slot boundary")
	}

	commitmentSize := len(lo.PanicOnErr(commitment.NewEmptyCommitment().Bytes()))

	for slotIndex := int64(0); slotIndex <= slotBoundary; slotIndex++ {
		commitmentBytes := make([]byte, commitmentSize)
		if err = binary.Read(reader, binary.LittleEndian, commitmentBytes); err != nil {
			return errors.Wrapf(err, "failed to read commitment bytes for slot %d", slotIndex)
		}

		newCommitment := new(commitment.Commitment)
		if consumedBytes, fromBytesErr := newCommitment.FromBytes(commitmentBytes); fromBytesErr != nil {
			return errors.Wrapf(fromBytesErr, "failed to parse commitment of slot %d", slotIndex)
		} else if consumedBytes != commitmentSize {
			return errors.Errorf("failed to read commitment of slot %d: consumed bytes (%d) != expected bytes (%d)", slotIndex, consumedBytes, commitmentSize)
		}

		if err = c.Store(newCommitment); err != nil {
			return errors.Wrapf(err, "failed to store commitment of slot %d", slotIndex)
		}
	}

	c.TriggerInitialized()

	return nil
}

func determineCommitmentLength() (length int, err error) {
	serializedCommitment, err := commitment.NewEmptyCommitment().Bytes()
	if err != nil {
		return 0, err
	}

	return len(serializedCommitment), nil
}
