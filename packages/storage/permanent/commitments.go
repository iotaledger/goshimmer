package permanent

import (
	"encoding/binary"
	"io"

	"github.com/cockroachdb/errors"

	"github.com/iotaledger/hive.go/core/generics/lo"

	"github.com/iotaledger/goshimmer/packages/core/commitment"
	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/storable"
	"github.com/iotaledger/goshimmer/packages/core/traits"
)

type Commitments struct {
	slice    *storable.Slice[commitment.Commitment, *commitment.Commitment]
	filePath string

	traits.Initializable
}

func NewCommitments(path string) (newCommitment *Commitments) {
	commitmentsSlice, err := storable.NewSlice[commitment.Commitment](path)
	if err != nil {
		panic(errors.Errorf("failed to create commitments file: %w", err))
	}

	return &Commitments{
		slice:         commitmentsSlice,
		filePath:      path,
		Initializable: traits.NewInitializable(),
	}
}

func (c *Commitments) Store(commitment *commitment.Commitment) (err error) {
	if err = c.slice.Set(int(commitment.Index()), commitment); err != nil {
		return errors.Errorf("failed to store commitment for epoch %d: %w", commitment.Index(), err)
	}

	return nil
}

func (c *Commitments) Load(index epoch.Index) (commitment *commitment.Commitment, err error) {
	if commitment, err = c.slice.Get(int(index)); err != nil {
		return nil, errors.Errorf("failed to get commitment for epoch %d: %w", index, err)
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

func (c *Commitments) Export(writer io.WriteSeeker, targetEpoch epoch.Index) (err error) {
	if err = binary.Write(writer, binary.LittleEndian, int64(targetEpoch)); err != nil {
		return errors.Errorf("failed to write epoch boundary: %w", err)
	}

	for epochIndex := epoch.Index(0); epochIndex <= targetEpoch; epochIndex++ {
		if err = binary.Write(writer, binary.LittleEndian, lo.PanicOnErr(lo.PanicOnErr(c.Load(epochIndex)).Bytes())); err != nil {
			return errors.Errorf("failed to write commitment for epoch %d: %w", epochIndex, err)
		}
	}

	return nil
}

func (c *Commitments) Import(reader io.ReadSeeker) (err error) {
	var epochBoundary int64
	if err = binary.Read(reader, binary.LittleEndian, &epochBoundary); err != nil {
		return errors.Errorf("failed to read epoch boundary: %w", err)
	}

	commitmentSize := len(lo.PanicOnErr(new(commitment.Commitment).Bytes()))

	for epochIndex := int64(0); epochIndex <= epochBoundary; epochIndex++ {
		commitmentBytes := make([]byte, commitmentSize)
		if err = binary.Read(reader, binary.LittleEndian, commitmentBytes); err != nil {
			return errors.Errorf("failed to read commitment bytes for epoch %d: %w", epochIndex, err)
		}

		newCommitment := new(commitment.Commitment)
		if consumedBytes, fromBytesErr := newCommitment.FromBytes(commitmentBytes); fromBytesErr != nil {
			return errors.Errorf("failed to parse commitment of epoch %d: %w", epochIndex, fromBytesErr)
		} else if consumedBytes != commitmentSize {
			return errors.Errorf("failed to read commitment of epoch %d: consumed bytes (%d) != expected bytes (%d)", epochIndex, consumedBytes, commitmentSize)
		}

		if err = c.Store(newCommitment); err != nil {
			return errors.Errorf("failed to store commitment of epoch %d: %w", epochIndex, err)
		}
	}

	c.TriggerInitialized()

	return nil
}
