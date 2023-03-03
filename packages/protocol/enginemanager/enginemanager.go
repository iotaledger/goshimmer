package enginemanager

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/google/uuid"
	"github.com/pkg/errors"

	"github.com/iotaledger/goshimmer/packages/core/database"
	"github.com/iotaledger/goshimmer/packages/core/module"
	"github.com/iotaledger/goshimmer/packages/protocol/engine"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/sybilprotection"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/throughputquota"
	"github.com/iotaledger/goshimmer/packages/storage"
	"github.com/iotaledger/goshimmer/packages/storage/utils"
	"github.com/iotaledger/hive.go/core/slot"
	"github.com/iotaledger/hive.go/lo"
	"github.com/iotaledger/hive.go/runtime/ioutils"
	"github.com/iotaledger/hive.go/runtime/options"
	"github.com/iotaledger/hive.go/runtime/workerpool"
)

const engineInfoFile = "info"

type engineInfo struct {
	Name string `json:"name"`
}

// region EngineManager ////////////////////////////////////////////////////////////////////////////////////////////////

type EngineManager struct {
	directory      *utils.Directory
	dbVersion      database.Version
	storageOptions []options.Option[database.Manager]
	workers        *workerpool.Group

	engineOptions           []options.Option[engine.Engine]
	sybilProtectionProvider module.Provider[*engine.Engine, sybilprotection.SybilProtection]
	throughputQuotaProvider module.Provider[*engine.Engine, throughputquota.ThroughputQuota]
	slotTimeProvider        *slot.TimeProvider

	activeInstance *EngineInstance
}

func New(
	workers *workerpool.Group,
	dir string,
	dbVersion database.Version,
	storageOptions []options.Option[database.Manager],
	engineOptions []options.Option[engine.Engine],
	sybilProtectionProvider module.Provider[*engine.Engine, sybilprotection.SybilProtection],
	throughputQuotaProvider module.Provider[*engine.Engine, throughputquota.ThroughputQuota],
	slotTimeProvider *slot.TimeProvider) *EngineManager {
	return &EngineManager{
		workers:                 workers,
		directory:               utils.NewDirectory(dir),
		dbVersion:               dbVersion,
		storageOptions:          storageOptions,
		engineOptions:           engineOptions,
		sybilProtectionProvider: sybilProtectionProvider,
		throughputQuotaProvider: throughputQuotaProvider,
		slotTimeProvider:        slotTimeProvider,
	}
}

func (m *EngineManager) LoadActiveEngine() (*EngineInstance, error) {
	info := &engineInfo{}
	if err := ioutils.ReadJSONFromFile(m.infoFilePath(), info); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("unable to read engine info file: %w", err)
		}
	}

	if len(info.Name) > 0 {
		if exists, isDirectory, err := ioutils.PathExists(m.directory.Path(info.Name)); err == nil && exists && isDirectory {
			// Load previous engine as active
			m.activeInstance = m.loadEngineInstance(info.Name)
		}
	}

	if m.activeInstance == nil {
		// Start with a new instance and set to active
		instance := m.newEngineInstance()
		if err := m.SetActiveInstance(instance); err != nil {
			return nil, err
		}
	}

	// Cleanup non-active instances
	if err := m.CleanupNonActive(); err != nil {
		return nil, err
	}

	return m.activeInstance, nil
}

func (m *EngineManager) CleanupNonActive() error {
	activeDir := filepath.Base(m.activeInstance.Storage.Directory)

	dirs, err := m.directory.SubDirs()
	if err != nil {
		return err
	}
	for _, dir := range dirs {
		if dir == activeDir {
			continue
		}
		if err := m.directory.RemoveSubdir(dir); err != nil {
			return err
		}
	}
	return nil
}

func (m *EngineManager) infoFilePath() string {
	return m.directory.Path(engineInfoFile)
}

func (m *EngineManager) SetActiveInstance(instance *EngineInstance) error {
	m.activeInstance = instance

	info := &engineInfo{
		Name: filepath.Base(instance.Storage.Directory),
	}

	return ioutils.WriteJSONToFile(m.infoFilePath(), info, 0o644)
}

func (m *EngineManager) loadEngineInstance(dirName string) *EngineInstance {
	candidateStorage := storage.New(m.directory.Path(dirName), m.dbVersion, m.storageOptions...)
	candidateEngine := engine.New(m.workers.CreateGroup(dirName), candidateStorage, m.sybilProtectionProvider, m.throughputQuotaProvider, m.slotTimeProvider, m.engineOptions...)

	return &EngineInstance{
		Engine:  candidateEngine,
		Storage: candidateStorage,
	}
}

func (m *EngineManager) newEngineInstance() *EngineInstance {
	dirName := lo.PanicOnErr(uuid.NewUUID()).String()
	return m.loadEngineInstance(dirName)
}

func (m *EngineManager) ForkEngineAtSlot(index slot.Index) (*EngineInstance, error) {
	// Dump a snapshot at the target index
	snapshotPath := filepath.Join(os.TempDir(), fmt.Sprintf("snapshot_%d_%s.bin", index, lo.PanicOnErr(uuid.NewUUID()).String()))
	if err := m.activeInstance.Engine.WriteSnapshot(snapshotPath, index); err != nil {
		return nil, errors.Wrapf(err, "error exporting snapshot for index %s", index)
	}

	instance := m.newEngineInstance()
	if err := instance.InitializeWithSnapshot(snapshotPath); err != nil {
		instance.Shutdown()
		_ = instance.RemoveFromFilesystem()
		_ = os.Remove(snapshotPath)
		return nil, err
	}

	return instance, nil
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region EngineInstance ////////////////////////////////////////////////////////////////////////////////////////////////

type EngineInstance struct {
	Engine  *engine.Engine
	Storage *storage.Storage
}

func (e *EngineInstance) InitializeWithSnapshot(snapshotPath string) error {
	return e.Engine.Initialize(snapshotPath)
}

func (e *EngineInstance) Name() string {
	return filepath.Base(e.Storage.Directory)
}

func (e *EngineInstance) Shutdown() {
	e.Engine.Shutdown()
	e.Storage.Shutdown()
}

func (e *EngineInstance) RemoveFromFilesystem() error {
	return os.RemoveAll(e.Storage.Directory)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
