package permanent

import (
	"context"
	"encoding/binary"
	"io"
	"sync"

	"github.com/pkg/errors"

	"github.com/iotaledger/goshimmer/packages/core/commitment"
	"github.com/iotaledger/goshimmer/packages/core/storable"
	"github.com/iotaledger/hive.go/core/slot"
	"github.com/iotaledger/hive.go/ds/types"
	"github.com/iotaledger/hive.go/runtime/module"
	"github.com/iotaledger/hive.go/serializer/v2/serix"
	"github.com/iotaledger/hive.go/stringify"
)

// region Settings /////////////////////////////////////////////////////////////////////////////////////////////////////

type Settings struct {
	*settingsModel
	mutex sync.RWMutex

	slotTimeProvider *slot.TimeProvider

	module.Module
}

func NewSettings(path string) (settings *Settings) {
	s := &Settings{
		settingsModel: storable.InitStruct(&settingsModel{
			SnapshotImported:        false,
			GenesisUnixTime:         0,
			SlotDuration:            0,
			LatestCommitment:        commitment.New(0, commitment.ID{}, types.Identifier{}, 0),
			LatestStateMutationSlot: 0,
			LatestConfirmedSlot:     0,
			ChainID:                 commitment.ID{},
		}, path),
	}

	s.UpdateSlotTimeProvider()

	return s
}

func (s *Settings) SlotTimeProvider() *slot.TimeProvider {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	if s.settingsModel.SlotDuration == 0 || s.settingsModel.GenesisUnixTime == 0 {
		panic("SlotTimeProvider not initialized yet")
	}

	return s.slotTimeProvider
}

func (s *Settings) SnapshotImported() (initialized bool) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	return s.settingsModel.SnapshotImported
}

func (s *Settings) SetSnapshotImported(initialized bool) (err error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.settingsModel.SnapshotImported = initialized

	if err = s.ToFile(); err != nil {
		return errors.Wrap(err, "failed to persist initialized flag")
	}

	return nil
}

func (s *Settings) GenesisUnixTime() int64 {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	return s.settingsModel.GenesisUnixTime
}

func (s *Settings) SetGenesisUnixTime(unixTime int64) (err error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.settingsModel.GenesisUnixTime = unixTime
	s.UpdateSlotTimeProvider()

	if err = s.ToFile(); err != nil {
		return errors.Wrap(err, "failed to persist initialized flag")
	}

	return nil
}

func (s *Settings) SlotDuration() int64 {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	return s.settingsModel.SlotDuration
}

func (s *Settings) SetSlotDuration(duration int64) (err error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.settingsModel.SlotDuration = duration
	s.UpdateSlotTimeProvider()

	if err = s.ToFile(); err != nil {
		return errors.Wrap(err, "failed to persist initialized flag")
	}

	return nil
}

func (s *Settings) LatestCommitment() (latestCommitment *commitment.Commitment) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	return s.settingsModel.LatestCommitment
}

func (s *Settings) SetLatestCommitment(latestCommitment *commitment.Commitment) (err error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.settingsModel.LatestCommitment = latestCommitment

	if err = s.ToFile(); err != nil {
		return errors.Wrap(err, "failed to persist latest commitment")
	}

	return nil
}

func (s *Settings) LatestStateMutationSlot() slot.Index {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	return s.settingsModel.LatestStateMutationSlot
}

func (s *Settings) SetLatestStateMutationSlot(latestStateMutationSlot slot.Index) (err error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.settingsModel.LatestStateMutationSlot = latestStateMutationSlot

	if err = s.ToFile(); err != nil {
		return errors.Wrap(err, "failed to persist latest state mutation slot")
	}

	return nil
}

func (s *Settings) LatestConfirmedSlot() slot.Index {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	return s.settingsModel.LatestConfirmedSlot
}

func (s *Settings) SetLatestConfirmedSlot(latestConfirmedSlot slot.Index) (err error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.settingsModel.LatestConfirmedSlot = latestConfirmedSlot

	if err = s.ToFile(); err != nil {
		return errors.Wrap(err, "failed to persist latest confirmed slot")
	}

	return nil
}

func (s *Settings) ChainID() commitment.ID {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	return s.settingsModel.ChainID
}

func (s *Settings) SetChainID(id commitment.ID) (err error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.settingsModel.ChainID = id

	if err = s.ToFile(); err != nil {
		return errors.Wrap(err, "failed to persist chain ID")
	}

	return nil
}

func (s *Settings) Export(writer io.WriteSeeker) (err error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	settingsBytes, err := s.Bytes()
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

func (s *Settings) Import(reader io.ReadSeeker) (err error) {
	if err = s.tryImport(reader); err != nil {
		return errors.Wrap(err, "failed to import settings")
	}

	s.TriggerInitialized()

	return
}

func (s *Settings) String() string {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	builder := stringify.NewStructBuilder("Settings", stringify.NewStructField("path", s.FilePath()))
	builder.AddField(stringify.NewStructField("SnapshotImported", s.settingsModel.SnapshotImported))
	builder.AddField(stringify.NewStructField("GenesisUnixTime", s.settingsModel.GenesisUnixTime))
	builder.AddField(stringify.NewStructField("SlotDuration", s.settingsModel.SlotDuration))
	builder.AddField(stringify.NewStructField("LatestCommitment", s.settingsModel.LatestCommitment))
	builder.AddField(stringify.NewStructField("LatestStateMutationSlot", s.settingsModel.LatestStateMutationSlot))
	builder.AddField(stringify.NewStructField("LatestConfirmedSlot", s.settingsModel.LatestConfirmedSlot))
	builder.AddField(stringify.NewStructField("ChainID", s.settingsModel.ChainID))

	return builder.String()
}

func (s *Settings) tryImport(reader io.ReadSeeker) (err error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	var settingsSize uint32
	if err = binary.Read(reader, binary.LittleEndian, &settingsSize); err != nil {
		return errors.Wrap(err, "failed to read settings length")
	}

	settingsBytes := make([]byte, settingsSize)
	if err = binary.Read(reader, binary.LittleEndian, settingsBytes); err != nil {
		return errors.Wrap(err, "failed to read settings bytes")
	}

	if consumedBytes, fromBytesErr := s.FromBytes(settingsBytes); fromBytesErr != nil {
		return errors.Wrapf(fromBytesErr, "failed to read settings")
	} else if consumedBytes != len(settingsBytes) {
		return errors.Errorf("failed to read settings: consumed bytes (%d) != expected bytes (%d)", consumedBytes, len(settingsBytes))
	}

	s.settingsModel.SnapshotImported = true

	s.UpdateSlotTimeProvider()

	if err = s.settingsModel.ToFile(); err != nil {
		return errors.Wrap(err, "failed to persist chain ID")
	}

	return
}

func (s *Settings) UpdateSlotTimeProvider() {
	s.slotTimeProvider = slot.NewTimeProvider(s.settingsModel.GenesisUnixTime, s.settingsModel.SlotDuration)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region settingsModel ////////////////////////////////////////////////////////////////////////////////////////////////

type settingsModel struct {
	SnapshotImported        bool                   `serix:"0"`
	GenesisUnixTime         int64                  `serix:"1"`
	SlotDuration            int64                  `serix:"2"`
	LatestCommitment        *commitment.Commitment `serix:"3"`
	LatestStateMutationSlot slot.Index             `serix:"4"`
	LatestConfirmedSlot     slot.Index             `serix:"5"`
	ChainID                 commitment.ID          `serix:"6"`

	storable.Struct[settingsModel, *settingsModel]
}

func (s *settingsModel) FromBytes(bytes []byte) (int, error) {
	return serix.DefaultAPI.Decode(context.Background(), bytes, s)
}

func (s settingsModel) Bytes() ([]byte, error) {
	return serix.DefaultAPI.Encode(context.Background(), s)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
