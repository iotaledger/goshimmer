package permanent

import (
	"context"
	"encoding/binary"
	"io"
	"sync"

	"github.com/pkg/errors"

	"github.com/iotaledger/goshimmer/packages/core/commitment"
	"github.com/iotaledger/goshimmer/packages/core/module"
	"github.com/iotaledger/goshimmer/packages/core/storable"
	"github.com/iotaledger/hive.go/core/slot"
	"github.com/iotaledger/hive.go/ds/types"
	"github.com/iotaledger/hive.go/serializer/v2/serix"
)

// region Settings /////////////////////////////////////////////////////////////////////////////////////////////////////

type Settings struct {
	*settingsModel
	mutex sync.RWMutex

	module.Module
}

func NewSettings(path string) (settings *Settings) {
	return &Settings{
		settingsModel: storable.InitStruct(&settingsModel{
			SnapshotImported:        false,
			LatestCommitment:        commitment.New(0, commitment.ID{}, types.Identifier{}, 0),
			LatestStateMutationSlot: 0,
			LatestConfirmedSlot:     0,
			ChainID:                 commitment.ID{},
		}, path),
	}
}

func (c *Settings) SnapshotImported() (initialized bool) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	return c.settingsModel.SnapshotImported
}

func (c *Settings) SetSnapshotImported(initialized bool) (err error) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.settingsModel.SnapshotImported = initialized

	if err = c.ToFile(); err != nil {
		return errors.Wrap(err, "failed to persist initialized flag")
	}

	return nil
}

func (c *Settings) LatestCommitment() (latestCommitment *commitment.Commitment) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	return c.settingsModel.LatestCommitment
}

func (c *Settings) SetLatestCommitment(latestCommitment *commitment.Commitment) (err error) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.settingsModel.LatestCommitment = latestCommitment

	if err = c.ToFile(); err != nil {
		return errors.Wrap(err, "failed to persist latest commitment")
	}

	return nil
}

func (c *Settings) LatestStateMutationSlot() slot.Index {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	return c.settingsModel.LatestStateMutationSlot
}

func (c *Settings) SetLatestStateMutationSlot(latestStateMutationSlot slot.Index) (err error) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.settingsModel.LatestStateMutationSlot = latestStateMutationSlot

	if err = c.ToFile(); err != nil {
		return errors.Wrap(err, "failed to persist latest state mutation slot")
	}

	return nil
}

func (c *Settings) LatestConfirmedSlot() slot.Index {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	return c.settingsModel.LatestConfirmedSlot
}

func (c *Settings) SetLatestConfirmedSlot(latestConfirmedSlot slot.Index) (err error) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.settingsModel.LatestConfirmedSlot = latestConfirmedSlot

	if err = c.ToFile(); err != nil {
		return errors.Wrap(err, "failed to persist latest confirmed slot")
	}

	return nil
}

func (c *Settings) ChainID() commitment.ID {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	return c.settingsModel.ChainID
}

func (c *Settings) SetChainID(id commitment.ID) (err error) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.settingsModel.ChainID = id

	if err = c.ToFile(); err != nil {
		return errors.Wrap(err, "failed to persist chain ID")
	}

	return nil
}

func (c *Settings) Export(writer io.WriteSeeker) (err error) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	settingsBytes, err := c.Bytes()
	if err != nil {
		return errors.Wrap(err, "failed to convert settings to bytes")
	}

	if err = binary.Write(writer, binary.LittleEndian, uint32(len(settingsBytes))); err != nil {
		return errors.Wrap(err, "failed to write settings length")
	}

	if err = binary.Write(writer, binary.LittleEndian, settingsBytes); err != nil {
		return errors.Wrap(err, "failed to write settings")
	}

	return nil
}

func (c *Settings) Import(reader io.ReadSeeker) (err error) {
	if err = c.tryImport(reader); err != nil {
		return errors.Wrap(err, "failed to import settings")
	}

	c.TriggerInitialized()

	return
}

func (c *Settings) tryImport(reader io.ReadSeeker) (err error) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	var settingsSize uint32
	if err = binary.Read(reader, binary.LittleEndian, &settingsSize); err != nil {
		return errors.Wrap(err, "failed to read settings length")
	}

	settingsBytes := make([]byte, settingsSize)
	if err = binary.Read(reader, binary.LittleEndian, settingsBytes); err != nil {
		return errors.Wrap(err, "failed to read settings bytes")
	}

	if consumedBytes, fromBytesErr := c.FromBytes(settingsBytes); fromBytesErr != nil {
		return errors.Wrapf(fromBytesErr, "failed to read settings")
	} else if consumedBytes != len(settingsBytes) {
		return errors.Errorf("failed to read settings: consumed bytes (%d) != expected bytes (%d)", consumedBytes, len(settingsBytes))
	}

	c.settingsModel.SnapshotImported = true

	if err = c.settingsModel.ToFile(); err != nil {
		return errors.Wrap(err, "failed to persist chain ID")
	}

	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region settingsModel ////////////////////////////////////////////////////////////////////////////////////////////////

type settingsModel struct {
	SnapshotImported        bool                   `serix:"0"`
	LatestCommitment        *commitment.Commitment `serix:"1"`
	LatestStateMutationSlot slot.Index             `serix:"2"`
	LatestConfirmedSlot     slot.Index             `serix:"3"`
	ChainID                 commitment.ID          `serix:"4"`

	storable.Struct[settingsModel, *settingsModel]
}

func (s *settingsModel) FromBytes(bytes []byte) (int, error) {
	return serix.DefaultAPI.Decode(context.Background(), bytes, s)
}

func (s settingsModel) Bytes() ([]byte, error) {
	return serix.DefaultAPI.Encode(context.Background(), s)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
