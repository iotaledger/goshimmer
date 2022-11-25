package permanent

import (
	"context"
	"encoding/binary"
	"io"
	"sync"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/core/serix"
	"github.com/iotaledger/hive.go/core/types"

	"github.com/iotaledger/goshimmer/packages/core/commitment"
	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/storable"
)

// region Settings /////////////////////////////////////////////////////////////////////////////////////////////////////

type Settings struct {
	settingsModel *settingsModel

	sync.RWMutex
}

func NewSettings(path string) (setting *Settings) {
	return &Settings{
		settingsModel: storable.InitStruct(&settingsModel{
			LatestCommitment:         commitment.New(0, commitment.ID{}, types.Identifier{}, 0),
			LatestStateMutationEpoch: 0,
			LatestConfirmedEpoch:     0,
			ChainID:                  commitment.ID{},
		}, path),
	}
}

func (c *Settings) LatestCommitment() (latestCommitment *commitment.Commitment) {
	c.RLock()
	defer c.RUnlock()

	return c.settingsModel.LatestCommitment
}

func (c *Settings) SetLatestCommitment(latestCommitment *commitment.Commitment) (err error) {
	c.Lock()
	defer c.Unlock()

	c.settingsModel.LatestCommitment = latestCommitment

	if err = c.settingsModel.ToFile(); err != nil {
		return errors.Errorf("failed to persist latest commitment: %w", err)
	}

	return nil
}

func (c *Settings) LatestStateMutationEpoch() (latestStateMutationEpoch epoch.Index) {
	c.RLock()
	defer c.RUnlock()

	return c.settingsModel.LatestStateMutationEpoch
}

func (c *Settings) SetLatestStateMutationEpoch(latestStateMutationEpoch epoch.Index) (err error) {
	c.Lock()
	defer c.Unlock()

	c.settingsModel.LatestStateMutationEpoch = latestStateMutationEpoch

	if err = c.settingsModel.ToFile(); err != nil {
		return errors.Errorf("failed to persist latest state mutation epoch: %w", err)
	}

	return nil
}

func (c *Settings) LatestConfirmedEpoch() (latestConfirmedEpoch epoch.Index) {
	c.RLock()
	defer c.RUnlock()

	return c.settingsModel.LatestConfirmedEpoch
}

func (c *Settings) SetLatestConfirmedEpoch(latestConfirmedEpoch epoch.Index) (err error) {
	c.Lock()
	defer c.Unlock()

	c.settingsModel.LatestConfirmedEpoch = latestConfirmedEpoch

	if err = c.settingsModel.ToFile(); err != nil {
		return errors.Errorf("failed to persist latest confirmed epoch: %w", err)
	}

	return nil
}

func (c *Settings) ChainID() commitment.ID {
	c.RLock()
	defer c.RUnlock()

	return c.settingsModel.ChainID
}

func (c *Settings) SetChainID(id commitment.ID) (err error) {
	c.Lock()
	defer c.Unlock()

	c.settingsModel.ChainID = id

	if err = c.settingsModel.ToFile(); err != nil {
		return errors.Errorf("failed to persist chain ID: %w", err)
	}

	return nil
}

func (c *Settings) Export(writer io.WriteSeeker) (err error) {
	c.RLock()
	defer c.RUnlock()

	settingsBytes, err := c.settingsModel.Bytes()
	if err != nil {
		return errors.Errorf("failed to convert settings to bytes: %w", err)
	}

	if err = binary.Write(writer, binary.LittleEndian, uint32(len(settingsBytes))); err != nil {
		return errors.Errorf("failed to write settings length: %w", err)
	}

	if err = binary.Write(writer, binary.LittleEndian, settingsBytes); err != nil {
		return errors.Errorf("failed to write settings: %w", err)
	}

	return nil
}

func (c *Settings) ReadFrom(reader io.ReadSeeker) (err error) {
	c.Lock()
	defer c.Unlock()

	var settingsSize uint32
	if err = binary.Read(reader, binary.LittleEndian, &settingsSize); err != nil {
		return errors.Errorf("failed to read settings length: %w", err)
	}

	settingsBytes := make([]byte, settingsSize)
	if err = binary.Read(reader, binary.LittleEndian, settingsBytes); err != nil {
		return errors.Errorf("failed to read settings bytes: %w", err)
	}

	if consumedBytes, err := c.settingsModel.FromBytes(settingsBytes); err != nil {
		return errors.Errorf("failed to read settings: %w", err)
	} else if consumedBytes != len(settingsBytes) {
		return errors.Errorf("failed to read settings: consumed bytes (%d) != expected bytes (%d)", consumedBytes, len(settingsBytes))
	}

	return nil
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region settingsModel ////////////////////////////////////////////////////////////////////////////////////////////////

type settingsModel struct {
	LatestCommitment         *commitment.Commitment `serix:"0"`
	LatestStateMutationEpoch epoch.Index            `serix:"1"`
	LatestConfirmedEpoch     epoch.Index            `serix:"2"`
	ChainID                  commitment.ID          `serix:"3"`

	storable.Struct[settingsModel, *settingsModel]
}

func (s *settingsModel) FromBytes(bytes []byte) (int, error) {
	return serix.DefaultAPI.Decode(context.Background(), bytes, s)
}

func (s settingsModel) Bytes() ([]byte, error) {
	return serix.DefaultAPI.Encode(context.Background(), s)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
