package enginemanager

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/google/uuid"
	"github.com/pkg/errors"

	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/ioutils"

	"github.com/iotaledger/goshimmer/packages/core/database"
	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/protocol/engine"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/sybilprotection"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/throughputquota"
	"github.com/iotaledger/goshimmer/packages/storage"
	"github.com/iotaledger/goshimmer/packages/storage/utils"
)

const engineInfoFile = "info"

type engineInfo struct {
	Name string `json:"name"`
}

type EngineManager struct {
	directory      *utils.Directory
	dbVersion      database.Version
	storageOptions []options.Option[database.Manager]

	engineOptions           []options.Option[engine.Engine]
	sybilProtectionProvider engine.ModuleProvider[sybilprotection.SybilProtection]
	throughputQuotaProvider engine.ModuleProvider[throughputquota.ThroughputQuota]

	activeInstance *EngineInstance
}

type EngineInstance struct {
	Engine  *engine.Engine
	Storage *storage.Storage
}

func New(dir string,
	dbVersion database.Version,
	storageOptions []options.Option[database.Manager],
	engineOptions []options.Option[engine.Engine],
	sybilProtectionProvider engine.ModuleProvider[sybilprotection.SybilProtection],
	throughputQuotaProvider engine.ModuleProvider[throughputquota.ThroughputQuota]) *EngineManager {
	return &EngineManager{
		directory:               utils.NewDirectory(dir),
		dbVersion:               dbVersion,
		storageOptions:          storageOptions,
		engineOptions:           engineOptions,
		sybilProtectionProvider: sybilProtectionProvider,
		throughputQuotaProvider: throughputQuotaProvider,
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
	candidateEngine := engine.New(candidateStorage, m.sybilProtectionProvider, m.throughputQuotaProvider, m.engineOptions...)

	return &EngineInstance{
		Engine:  candidateEngine,
		Storage: candidateStorage,
	}
}

func (m *EngineManager) newEngineInstance() *EngineInstance {
	dirName := lo.PanicOnErr(uuid.NewUUID()).String()
	return m.loadEngineInstance(dirName)
}

func (m *EngineManager) ForkEngineAtEpoch(index epoch.Index) (*EngineInstance, error) {
	// Dump a snapshot at the target index
	snapshotPath := filepath.Join(os.TempDir(), fmt.Sprintf("snapshot_%d.bin", index))
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

func (e *EngineInstance) InitializeWithSnapshot(snapshotPath string) error {
	return e.Engine.Initialize(snapshotPath)
}

func (e *EngineInstance) Shutdown() {
	e.Engine.Shutdown()
	e.Storage.Shutdown()
}

func (e *EngineInstance) RemoveFromFilesystem() error {
	return os.RemoveAll(e.Storage.Directory)
}
