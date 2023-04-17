package enginemanager

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/google/uuid"
	"github.com/pkg/errors"

	"github.com/iotaledger/goshimmer/packages/core/database"
	"github.com/iotaledger/goshimmer/packages/protocol/engine"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/clock"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/consensus"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/filter"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/notarization"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/sybilprotection"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/throughputquota"
	"github.com/iotaledger/goshimmer/packages/storage"
	"github.com/iotaledger/goshimmer/packages/storage/utils"
	"github.com/iotaledger/hive.go/core/slot"
	"github.com/iotaledger/hive.go/lo"
	"github.com/iotaledger/hive.go/runtime/ioutils"
	"github.com/iotaledger/hive.go/runtime/module"
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
	clockProvider           module.Provider[*engine.Engine, clock.Clock]
	ledgerProvider          module.Provider[*engine.Engine, ledger.Ledger]
	filterProvider          module.Provider[*engine.Engine, filter.Filter]
	sybilProtectionProvider module.Provider[*engine.Engine, sybilprotection.SybilProtection]
	throughputQuotaProvider module.Provider[*engine.Engine, throughputquota.ThroughputQuota]
	notarizationProvider    module.Provider[*engine.Engine, notarization.Notarization]
	tangleProvider          module.Provider[*engine.Engine, tangle.Tangle]
	consensusProvider       module.Provider[*engine.Engine, consensus.Consensus]

	activeInstance *engine.Engine
}

func New(
	workers *workerpool.Group,
	dir string,
	dbVersion database.Version,
	storageOptions []options.Option[database.Manager],
	engineOptions []options.Option[engine.Engine],
	clockProvider module.Provider[*engine.Engine, clock.Clock],
	ledgerProvider module.Provider[*engine.Engine, ledger.Ledger],
	filterProvider module.Provider[*engine.Engine, filter.Filter],
	sybilProtectionProvider module.Provider[*engine.Engine, sybilprotection.SybilProtection],
	throughputQuotaProvider module.Provider[*engine.Engine, throughputquota.ThroughputQuota],
	notarizationProvider module.Provider[*engine.Engine, notarization.Notarization],
	tangleProvider module.Provider[*engine.Engine, tangle.Tangle],
	consensusProvider module.Provider[*engine.Engine, consensus.Consensus],
) *EngineManager {
	return &EngineManager{
		workers:                 workers,
		directory:               utils.NewDirectory(dir),
		dbVersion:               dbVersion,
		storageOptions:          storageOptions,
		engineOptions:           engineOptions,
		clockProvider:           clockProvider,
		ledgerProvider:          ledgerProvider,
		filterProvider:          filterProvider,
		sybilProtectionProvider: sybilProtectionProvider,
		throughputQuotaProvider: throughputQuotaProvider,
		notarizationProvider:    notarizationProvider,
		tangleProvider:          tangleProvider,
		consensusProvider:       consensusProvider,
	}
}

func (e *EngineManager) LoadActiveEngine() (*engine.Engine, error) {
	info := &engineInfo{}
	if err := ioutils.ReadJSONFromFile(e.infoFilePath(), info); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("unable to read engine info file: %w", err)
		}
	}

	if len(info.Name) > 0 {
		if exists, isDirectory, err := ioutils.PathExists(e.directory.Path(info.Name)); err == nil && exists && isDirectory {
			// Load previous engine as active
			e.activeInstance = e.loadEngineInstance(info.Name)
		}
	}

	if e.activeInstance == nil {
		// Start with a new instance and set to active
		instance := e.newEngineInstance()
		if err := e.SetActiveInstance(instance); err != nil {
			return nil, err
		}
	}

	// Cleanup non-active instances
	if err := e.CleanupNonActive(); err != nil {
		return nil, err
	}

	return e.activeInstance, nil
}

func (e *EngineManager) CleanupNonActive() error {
	activeDir := filepath.Base(e.activeInstance.Storage.Directory)

	dirs, err := e.directory.SubDirs()
	if err != nil {
		return err
	}
	for _, dir := range dirs {
		if dir == activeDir {
			continue
		}
		if err := e.directory.RemoveSubdir(dir); err != nil {
			return err
		}
	}
	return nil
}

func (e *EngineManager) infoFilePath() string {
	return e.directory.Path(engineInfoFile)
}

func (e *EngineManager) SetActiveInstance(instance *engine.Engine) error {
	e.activeInstance = instance

	info := &engineInfo{
		Name: filepath.Base(instance.Storage.Directory),
	}

	return ioutils.WriteJSONToFile(e.infoFilePath(), info, 0o644)
}

func (e *EngineManager) loadEngineInstance(dirName string) *engine.Engine {
	return engine.New(e.workers.CreateGroup(dirName),
		storage.New(e.directory.Path(dirName), e.dbVersion, e.storageOptions...),
		e.clockProvider,
		e.ledgerProvider,
		e.filterProvider,
		e.sybilProtectionProvider,
		e.throughputQuotaProvider,
		e.notarizationProvider,
		e.tangleProvider,
		e.consensusProvider,
		e.engineOptions...,
	)
}

func (e *EngineManager) newEngineInstance() *engine.Engine {
	dirName := lo.PanicOnErr(uuid.NewUUID()).String()
	return e.loadEngineInstance(dirName)
}

func (e *EngineManager) ForkEngineAtSlot(index slot.Index) (*engine.Engine, error) {
	// Dump a snapshot at the target index
	snapshotPath := filepath.Join(os.TempDir(), fmt.Sprintf("snapshot_%d_%s.bin", index, lo.PanicOnErr(uuid.NewUUID()).String()))
	if err := e.activeInstance.WriteSnapshot(snapshotPath, index); err != nil {
		return nil, errors.Wrapf(err, "error exporting snapshot for index %s", index)
	}

	instance := e.newEngineInstance()
	if err := instance.Initialize(snapshotPath); err != nil {
		instance.Shutdown()
		_ = instance.RemoveFromFilesystem()
		_ = os.Remove(snapshotPath)
		return nil, err
	}

	return instance, nil
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
