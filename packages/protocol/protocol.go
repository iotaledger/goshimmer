package protocol

import (
	"fmt"
	"os"
	"sync"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/iotaledger/hive.go/core/logger"

	"github.com/iotaledger/goshimmer/packages/core/chainstorage"
	"github.com/iotaledger/goshimmer/packages/core/chainstorage/snapshot"
	"github.com/iotaledger/goshimmer/packages/core/commitment"
	"github.com/iotaledger/goshimmer/packages/core/diskutil"
	"github.com/iotaledger/goshimmer/packages/network"
	"github.com/iotaledger/goshimmer/packages/protocol/chainmanager"
	"github.com/iotaledger/goshimmer/packages/protocol/congestioncontrol"
	"github.com/iotaledger/goshimmer/packages/protocol/congestioncontrol/icca/scheduler"
	"github.com/iotaledger/goshimmer/packages/protocol/engine"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/consensus/acceptance"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/blockdag"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
	"github.com/iotaledger/goshimmer/packages/protocol/tipmanager"
)

const (
	mainBaseDir      = "main"
	candidateBaseDir = "candidate"
)

// region Protocol /////////////////////////////////////////////////////////////////////////////////////////////////////

type Protocol struct {
	Events            *Events
	CongestionControl *congestioncontrol.CongestionControl
	TipManager        *tipmanager.TipManager

	dispatcher           network.Endpoint
	networkProtocol      *network.Protocol
	disk                 *diskutil.DiskUtil
	chainManager         *chainmanager.Manager
	activeEngineMutex    sync.RWMutex
	engine               *engine.Engine
	candidateEngine      *engine.Engine
	storage              *chainstorage.ChainStorage
	candidateStorage     *chainstorage.ChainStorage
	mainChain            commitment.ID
	candidateChain       commitment.ID
	optsBaseDirectory    string
	optsSettingsFileName string
	optsSnapshotPath     string
	// optsSolidificationOptions []options.Option[solidification.Requester]
	optsCongestionControlOptions []options.Option[congestioncontrol.CongestionControl]
	optsEngineOptions            []options.Option[engine.Engine]
	optsTipManagerOptions        []options.Option[tipmanager.TipManager]

	*logger.Logger
}

func New(dispatcher network.Endpoint, opts ...options.Option[Protocol]) (protocol *Protocol) {
	return options.Apply(&Protocol{
		Events: NewEvents(),

		dispatcher: dispatcher,

		optsBaseDirectory:    "",
		optsSettingsFileName: "settings.bin",
	}, opts,
		(*Protocol).initDisk,
		(*Protocol).initCongestionControl,
		(*Protocol).initMainChainStorage,
		(*Protocol).initMainEngine,
		(*Protocol).importSnapshot,
	)
}

func (p *Protocol) Run() {
	p.activateEngine(p.engine)
	p.initNetworkProtocol()
	p.initTipManager()
}

func (p *Protocol) initDisk() {
	p.disk = diskutil.New(p.optsBaseDirectory)
}

func (p *Protocol) initMainChainStorage() {
	p.storage = lo.PanicOnErr(chainstorage.NewChainStorage(p.disk.Path(mainBaseDir), DatabaseVersion))
}

func (p *Protocol) initCongestionControl() {
	p.CongestionControl = congestioncontrol.New(p.optsCongestionControlOptions...)

	p.Events.CongestionControl = p.CongestionControl.Events
}

func (p *Protocol) importSnapshot() {
	if err := p.importSnapshotFile(p.optsSnapshotPath); err != nil {
		panic(err)
	}
}

func (p *Protocol) initNetworkProtocol() {
	p.networkProtocol = network.NewProtocol(p.dispatcher)

	p.networkProtocol.Events.BlockRequestReceived.Attach(event.NewClosure(func(event *network.BlockRequestReceivedEvent) {
		if block, exists := p.Engine().Block(event.BlockID); exists {
			p.networkProtocol.SendBlock(block, event.Source)
		}
	}))

	p.networkProtocol.Events.BlockReceived.Attach(event.NewClosure(func(event *network.BlockReceivedEvent) {
		p.ProcessBlock(event.Block, event.Source)
	}))

	p.networkProtocol.Events.EpochCommitmentReceived.Attach(event.NewClosure(func(event *network.EpochCommitmentReceivedEvent) {
		p.chainManager.ProcessCommitment(event.Commitment)
	}))

	p.networkProtocol.Events.EpochCommitmentRequestReceived.Attach(event.NewClosure(func(event *network.EpochCommitmentRequestReceivedEvent) {
		if commitment, _ := p.chainManager.Commitment(event.CommitmentID); commitment != nil {
			p.networkProtocol.SendEpochCommitment(commitment.Commitment(), event.Source)
		}
	}))

	p.Events.CongestionControl.Scheduler.BlockScheduled.Attach(event.NewClosure(func(block *scheduler.Block) {
		p.networkProtocol.SendBlock(block.ModelsBlock)
	}))

	p.Events.Engine.BlockRequester.Tick.Attach(event.NewClosure(func(blockID models.BlockID) {
		p.networkProtocol.RequestBlock(blockID)
	}))

	p.chainManager.CommitmentRequester.Events.Tick.Attach(event.NewClosure(func(commitmentID commitment.ID) {
		p.networkProtocol.RequestCommitment(commitmentID)
	}))
}

func (p *Protocol) initMainEngine() {
	p.engine = engine.New(p.storage, p.optsEngineOptions...)
}

func (p *Protocol) initChainManager() {
	p.chainManager = chainmanager.NewManager(p.Engine().GenesisCommitment)
}

func (p *Protocol) initTipManager() {
	// TODO: SWITCH ENGINE SIMILAR TO REQUESTER
	p.TipManager = tipmanager.New(p.CongestionControl.Block, p.optsTipManagerOptions...)

	p.Events.CongestionControl.Scheduler.BlockScheduled.Attach(event.NewClosure(p.TipManager.AddTip))

	p.Events.Engine.Consensus.Acceptance.BlockAccepted.Attach(event.NewClosure(func(block *acceptance.Block) {
		p.TipManager.RemoveStrongParents(block.ModelsBlock)
	}))

	p.Events.Engine.Tangle.BlockDAG.BlockOrphaned.Hook(event.NewClosure(func(block *blockdag.Block) {
		if schedulerBlock, exists := p.CongestionControl.Block(block.ID()); exists {
			p.TipManager.DeleteTip(schedulerBlock)
		}
	}))

	p.Events.Engine.Tangle.BlockDAG.BlockUnorphaned.Hook(event.NewClosure(func(block *blockdag.Block) {
		if schedulerBlock, exists := p.CongestionControl.Block(block.ID()); exists {
			p.TipManager.AddTip(schedulerBlock)
		}
	}))

	p.Events.Engine.Tangle.BlockDAG.AllChildrenOrphaned.Hook(event.NewClosure(func(block *blockdag.Block) {
		schedulerBlock, exists := p.CongestionControl.Scheduler().Block(block.ID())
		if exists {
			fmt.Println("Add tip because all children orphaned", block.ID())
			p.TipManager.AddTip(schedulerBlock)
		}
	}))

	p.Events.TipManager = p.TipManager.Events
}

func (p *Protocol) ProcessBlock(block *models.Block, src identity.ID) {
	isSolid, chain, _ := p.chainManager.ProcessCommitment(block.Commitment())
	if !isSolid {
		return
	}

	if mainChain := p.storage.Chain(); chain.ForkingPoint.ID() == mainChain {
		p.Engine().ProcessBlockFromPeer(block, src)
	}

	if candidateEngine, candidateStorage := p.CandidateEngine(), p.CandidateStorage(); candidateEngine != nil && candidateStorage != nil {
		if candidateChain := candidateStorage.Chain(); chain.ForkingPoint.ID() == candidateChain {
			candidateEngine.ProcessBlockFromPeer(block, src)
		}
	}
}

func (p *Protocol) Engine() (instance *engine.Engine) {
	p.activeEngineMutex.RLock()
	defer p.activeEngineMutex.RUnlock()

	return p.engine
}

func (p *Protocol) CandidateEngine() (instance *engine.Engine) {
	p.activeEngineMutex.RLock()
	defer p.activeEngineMutex.RUnlock()

	return p.candidateEngine
}

func (p *Protocol) CandidateStorage() (chainstorage *chainstorage.ChainStorage) {
	p.activeEngineMutex.RLock()
	defer p.activeEngineMutex.RUnlock()

	return p.candidateStorage
}

func (p *Protocol) importSnapshotFile(filePath string) (err error) {
	if err = p.disk.WithFile(filePath, func(fileHandle *os.File) {
		snapshot.ReadSnapshot(fileHandle, p.Engine())
	}); err != nil {
		if os.IsNotExist(err) {
			return nil
		}

		return errors.Errorf("failed to read snapshot from file '%s': %w", filePath, err)
	}

	/*
		chainID := snapshotHeader.LatestECRecord.ID()
		chainDirectory := p.disk.Path(fmt.Sprintf("%x", lo.PanicOnErr(chainID.Bytes())))

		if !p.disk.Exists(chainDirectory) {
			if err = p.disk.CreateDir(chainDirectory); err != nil {
				return errors.Errorf("failed to create chain directory '%s': %w", chainDirectory, err)
			}
		}

		if p.disk.CopyFile(filePath, diskutil.New(chainDirectory).Path("snapshot.bin")) != nil {
			return errors.Errorf("failed to copy snapshot file '%s' to chain directory '%s': %w", filePath, chainDirectory, err)
		}
		// TODO: we can't move the file because it might be mounted through Docker
		// if err = diskutil.ReplaceFile(filePath, diskutil.New(chainDirectory).Path("snapshot.bin")); err != nil {
		// 	return errors.Errorf("failed to move snapshot file '%s' to chain directory '%s': %w", filePath, chainDirectory, err)
		// }

		// TODO: load snapshot into main ChainStorage
		p.storage.SetChain(chainID)
	*/

	return
}

func (p *Protocol) activateEngine(engine *engine.Engine) (err error) {
	p.TipManager.Reset()
	p.TipManager.ActivateEngine(engine)
	p.Events.Engine.LinkTo(engine.Events)
	p.CongestionControl.LinkTo(engine)

	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Options //////////////////////////////////////////////////////////////////////////////////////////////////////

func WithBaseDirectory(baseDirectory string) options.Option[Protocol] {
	return func(n *Protocol) {
		n.optsBaseDirectory = baseDirectory
	}
}

func WithSnapshotPath(snapshot string) options.Option[Protocol] {
	return func(n *Protocol) {
		n.optsSnapshotPath = snapshot
	}
}

func WithCongestionControlOptions(opts ...options.Option[congestioncontrol.CongestionControl]) options.Option[Protocol] {
	return func(e *Protocol) {
		e.optsCongestionControlOptions = opts
	}
}

func WithTipManagerOptions(opts ...options.Option[tipmanager.TipManager]) options.Option[Protocol] {
	return func(p *Protocol) {
		p.optsTipManagerOptions = opts
	}
}

// func WithSolidificationOptions(opts ...options.Option[solidification.Requester]) options.Option[Protocol] {
// 	return func(n *Protocol) {
// 		n.optsSolidificationOptions = opts
// 	}
// }

func WithEngineOptions(opts ...options.Option[engine.Engine]) options.Option[Protocol] {
	return func(n *Protocol) {
		n.optsEngineOptions = opts
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
