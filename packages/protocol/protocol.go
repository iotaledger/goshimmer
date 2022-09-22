package protocol

import (
	"fmt"
	"os"
	"sync"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/logger"

	"github.com/iotaledger/goshimmer/packages/core/commitment"
	"github.com/iotaledger/goshimmer/packages/core/diskutil"
	"github.com/iotaledger/goshimmer/packages/core/snapshot"
	"github.com/iotaledger/goshimmer/packages/network"
	"github.com/iotaledger/goshimmer/packages/network/gossip"
	"github.com/iotaledger/goshimmer/packages/network/p2p"
	"github.com/iotaledger/goshimmer/packages/protocol/chainmanager"
	"github.com/iotaledger/goshimmer/packages/protocol/database"
	"github.com/iotaledger/goshimmer/packages/protocol/instance"
	"github.com/iotaledger/goshimmer/packages/protocol/instance/engine/congestioncontrol/icca/scheduler"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
	"github.com/iotaledger/goshimmer/packages/protocol/solidification"
)

// region Protocol /////////////////////////////////////////////////////////////////////////////////////////////////////

type Protocol struct {
	Events *Events

	network              network.Interface
	disk                 *diskutil.DiskUtil
	settings             *Settings
	chainManager         *chainmanager.Manager
	solidification       *solidification.Solidification
	activeInstance       *instance.Instance
	activeInstanceMutex  sync.RWMutex
	instancesByChainID   map[commitment.ID]*instance.Instance
	optsBaseDirectory    string
	optsSettingsFileName string
	optsSnapshotFile     string
	optsDBManagerOptions []options.Option[database.Manager]
	// optsSolidificationOptions []options.Option[solidification.Solidification]

	*logger.Logger
}

func New(networkInstance network.Interface, log *logger.Logger, opts ...options.Option[Protocol]) (protocol *Protocol) {
	return options.Apply(&Protocol{
		Events: NewEvents(),

		network:              networkInstance,
		solidification:       solidification.New(networkInstance, log),
		instancesByChainID:   make(map[commitment.ID]*instance.Instance),
		optsBaseDirectory:    "",
		optsSettingsFileName: "settings.bin",
		optsSnapshotFile:     "snapshot.bin",

		Logger: log,
	}, opts, func(p *Protocol) {
		p.disk = diskutil.New(p.optsBaseDirectory)
		p.settings = NewSettings(p.disk.Path(p.optsSettingsFileName))

		if err := p.importSnapshot(p.disk.Path(p.optsSnapshotFile)); err != nil {
			panic(err)
		}

		if p.instantiateChains() == 0 {
			panic("no chains found (please provide a snapshot file)")
		}

		if err := p.activateMainChain(); err != nil {
			panic(err)
		}

		p.chainManager = chainmanager.NewManager(p.Instance().GenesisCommitment)

		// setup solidification event
		p.Events.Instance.Engine.Tangle.BlockDAG.BlockMissing.Attach(event.NewClosure(p.solidification.RequestBlock))
		p.Events.Instance.Engine.Tangle.BlockDAG.MissingBlockAttached.Attach(event.NewClosure(p.solidification.CancelBlockRequest))

		// setup gossip events
		p.network.Events().Gossip.BlockRequestReceived.Attach(event.NewClosure(p.processBlockRequest))
		p.network.Events().Gossip.BlockReceived.Attach(event.NewClosure(p.dispatchReceivedBlock))
		p.Events.Instance.Engine.CongestionControl.Scheduler.BlockScheduled.Attach(event.NewClosure(func(block *scheduler.Block) {
			p.network.SendBlock(block.ModelsBlock)
		}))
	})
}

func (p *Protocol) IssueBlock(block *models.Block, optSrc ...*p2p.Neighbor) {
	chain, _ := p.chainManager.ProcessCommitment(block.Commitment())
	if chain == nil {
		// TODO: TRIGGER CHAIN SOLIDIFICATION?
		return
	}

	if targetInstance, exists := p.instancesByChainID[chain.ForkingPoint.ID()]; exists {
		if len(optSrc) > 0 {
			targetInstance.ProcessBlockFromPeer(block, optSrc[0])
		} else {
			targetInstance.ProcessBlockFromPeer(block, nil)
		}
	}
}

func (p *Protocol) Instance() (instance *instance.Instance) {
	p.activeInstanceMutex.RLock()
	defer p.activeInstanceMutex.RUnlock()

	return p.activeInstance
}

func (p *Protocol) importSnapshot(fileName string) (err error) {
	var snapshotHeader *ledger.SnapshotHeader

	if err = p.disk.WithFile(fileName, func(file *os.File) (err error) {
		snapshotHeader, err = snapshot.ReadSnapshotHeader(file)

		return
	}); err != nil {
		if os.IsNotExist(err) {
			p.Logger.Debugf("snapshot file '%s' does not exist", fileName)

			return nil
		}

		return errors.Errorf("failed to read snapshot header from file '%s': %w", fileName, err)
	}

	chainID := snapshotHeader.LatestECRecord.ID()
	chainDirectory := p.disk.Path(fmt.Sprintf("%x", chainID.Bytes()))

	if !p.disk.Exists(chainDirectory) {
		if err = p.disk.CreateDir(chainDirectory); err != nil {
			return errors.Errorf("failed to create chain directory '%s': %w", chainDirectory, err)
		}
	}

	if err = diskutil.ReplaceFile(fileName, diskutil.New(chainDirectory).Path("snapshot.bin")); err != nil {
		return errors.Errorf("failed to copy snapshot file '%s' to chain directory '%s': %w", fileName, chainDirectory, err)
	}

	p.settings.AddChain(chainID)
	p.settings.SetMainChainID(chainID)
	p.settings.Persist()

	return
}

func (p *Protocol) readSnapshotHeader(snapshotFile string) (snapshotHeader *ledger.SnapshotHeader, err error) {
	if err = p.disk.WithFile(snapshotFile, func(file *os.File) (err error) {
		snapshotHeader, err = snapshot.ReadSnapshotHeader(file)
		return
	}); err != nil {
		err = errors.Errorf("failed to read snapshot header from file '%s': %w", snapshotFile, err)
	}

	return snapshotHeader, err
}

func (p *Protocol) instantiateChains() (chainCount int) {
	for chains := p.settings.Chains().Iterator(); chains.HasNext(); {
		chainID := chains.Next()

		p.instancesByChainID[chainID] = instance.New(p.disk.Path(fmt.Sprintf("%x", chainID.Bytes())), p.Logger)
	}

	return len(p.instancesByChainID)
}

func (p *Protocol) activateMainChain() (err error) {
	chainID := p.settings.MainChainID()

	mainInstance, exists := p.instancesByChainID[chainID]
	if !exists {
		return errors.Errorf("instance for chain '%s' does not exist", chainID)
	}

	p.activeInstance = mainInstance
	p.Events.Instance.LinkTo(mainInstance.Events)

	return
}

func (p *Protocol) dispatchReceivedBlock(event *gossip.BlockReceivedEvent) {
	block := new(models.Block)
	if _, err := block.FromBytes(event.Data); err != nil {
		p.Events.InvalidBlockReceived.Trigger(event.Neighbor)
		return
	}

	p.IssueBlock(block, event.Neighbor)
}

func (p *Protocol) processBlockRequest(event *gossip.BlockRequestReceived) {
	if block, exists := p.Instance().Block(event.BlockID); exists {
		p.network.SendBlock(block, event.Neighbor.Peer)
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Options //////////////////////////////////////////////////////////////////////////////////////////////////////

func WithBaseDirectory(baseDirectory string) options.Option[Protocol] {
	return func(n *Protocol) {
		n.optsBaseDirectory = baseDirectory
	}
}

func WithDBManagerOptions(opts ...options.Option[database.Manager]) options.Option[Protocol] {
	return func(n *Protocol) {
		n.optsDBManagerOptions = opts
	}
}

// func WithSolidificationOptions(opts ...options.Option[solidification.Solidification]) options.Option[Protocol] {
// 	return func(n *Protocol) {
// 		n.optsSolidificationOptions = opts
// 	}
// }

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
