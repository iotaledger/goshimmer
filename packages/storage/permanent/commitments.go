package permanent

import (
	"encoding/binary"
	"io"

	"github.com/pkg/errors"

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
		panic(errors.Wrap(err, "failed to create commitments file"))
	}

	return &Commitments{
		slice:         commitmentsSlice,
		filePath:      path,
		Initializable: traits.NewInitializable(),
	}
}

func (c *Commitments) Store(commitment *commitment.Commitment) (err error) {
	if err = c.slice.Set(int(commitment.Index()), commitment); err != nil {
		return errors.Wrapf(err, "failed to store commitment for epoch %d", commitment.Index())
	}

	return nil
}

func (c *Commitments) Load(index epoch.Index) (commitment *commitment.Commitment, err error) {
	if commitment, err = c.slice.Get(int(index)); err != nil {
		return nil, errors.Wrapf(err, "failed to get commitment for epoch %d", index)
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
		return errors.Wrap(err, "failed to write epoch boundary")
	}

	for epochIndex := epoch.Index(0); epochIndex <= targetEpoch; epochIndex++ {
		if err = binary.Write(writer, binary.LittleEndian, lo.PanicOnErr(lo.PanicOnErr(c.Load(epochIndex)).Bytes())); err != nil {
			return errors.Wrapf(err, "failed to write commitment for epoch %d", epochIndex)
		}
	}

	return nil
}

func (c *Commitments) Import(reader io.ReadSeeker) (err error) {
	var epochBoundary int64
	if err = binary.Read(reader, binary.LittleEndian, &epochBoundary); err != nil {
		return errors.Wrap(err, "failed to read epoch boundary")
	}

	commitmentSize := len(lo.PanicOnErr(new(commitment.Commitment).Bytes()))

	for epochIndex := int64(0); epochIndex <= epochBoundary; epochIndex++ {
		commitmentBytes := make([]byte, commitmentSize)
		if err = binary.Read(reader, binary.LittleEndian, commitmentBytes); err != nil {
			return errors.Wrapf(err, "failed to read commitment bytes for epoch %d", epochIndex)
		}

		newCommitment := new(commitment.Commitment)
		if consumedBytes, fromBytesErr := newCommitment.FromBytes(commitmentBytes); fromBytesErr != nil {
			return errors.Wrapf(fromBytesErr, "failed to parse commitment of epoch %d", epochIndex)
		} else if consumedBytes != commitmentSize {
			return errors.Errorf("failed to read commitment of epoch %d: consumed bytes (%d) != expected bytes (%d)", epochIndex, consumedBytes, commitmentSize)
		}

		if err = c.Store(newCommitment); err != nil {
			return errors.Wrapf(err, "failed to store commitment of epoch %d", epochIndex)
		}
	}

	c.TriggerInitialized()

	return nil
}
