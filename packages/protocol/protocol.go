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

	"github.com/iotaledger/goshimmer/packages/core/commitment"
	"github.com/iotaledger/goshimmer/packages/core/diskutil"
	"github.com/iotaledger/goshimmer/packages/core/snapshot"
	"github.com/iotaledger/goshimmer/packages/network"
	"github.com/iotaledger/goshimmer/packages/protocol/chainmanager"
	"github.com/iotaledger/goshimmer/packages/protocol/engine"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/congestioncontrol/icca/scheduler"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
	"github.com/iotaledger/goshimmer/packages/protocol/solidification"
)

// region Protocol /////////////////////////////////////////////////////////////////////////////////////////////////////

type Protocol struct {
	Events *Events

	network              network.Network
	networkProtocol      *network.Protocol
	disk                 *diskutil.DiskUtil
	settings             *Settings
	chainManager         *chainmanager.Manager
	Requester            *solidification.Solidification
	activeInstance       *engine.Engine
	activeInstanceMutex  sync.RWMutex
	instancesByChainID   map[commitment.ID]*engine.Engine
	optsBaseDirectory    string
	optsSettingsFileName string
	optsSnapshotFileName string
	// optsSolidificationOptions []options.Option[solidification.Solidification]
	optsInstanceOptions []options.Option[engine.Engine]

	*logger.Logger
}

func New(networkInstance network.Network, log *logger.Logger, opts ...options.Option[Protocol]) (protocol *Protocol) {
	return options.Apply(
		&Protocol{
			Events: NewEvents(),

			network:            networkInstance,
			networkProtocol:    network.NewProtocol(networkInstance),
			instancesByChainID: make(map[commitment.ID]*engine.Engine),

			optsBaseDirectory:    "",
			optsSettingsFileName: "settings.bin",
			optsSnapshotFileName: "snapshot.bin",
		}, opts,
		(*Protocol).initDisk,
		(*Protocol).initSettings,
		(*Protocol).importSnapshot,
		(*Protocol).initEngines,
		func(p *Protocol) {
			p.chainManager = chainmanager.NewManager(p.Engine().GenesisCommitment)

			// setup gossip events
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

			p.Events.Engine.CongestionControl.Scheduler.BlockScheduled.Attach(event.NewClosure(func(block *scheduler.Block) {
				p.networkProtocol.SendBlock(block.ModelsBlock)
			}))
		},
		(*Protocol).initRequester,
	)

}

func (p *Protocol) initDisk() {
	p.disk = diskutil.New(p.optsBaseDirectory)
}

func (p *Protocol) initSettings() {
	p.settings = NewSettings(p.disk.Path(p.optsSettingsFileName))
}

func (p *Protocol) importSnapshot() {
	if err := p.importSnapshotFile(p.disk.Path(p.optsSnapshotFileName)); err != nil {
		panic(err)
	}
}

func (p *Protocol) initEngines() {
	if p.instantiateEngines() == 0 {
		panic("no chains found (please provide a snapshot file)")
	}

	if err := p.activateMainEngine(); err != nil {
		panic(err)
	}
}

func (p *Protocol) initRequester() {
	p.Requester = solidification.New(p.networkProtocol)

	p.chainManager.Events.CommitmentMissing.Attach(event.NewClosure(p.Requester.RequestCommitment))
	p.Events.Engine.Tangle.BlockDAG.BlockMissing.Attach(event.NewClosure(p.Requester.RequestBlock))
	p.Events.Engine.Tangle.BlockDAG.MissingBlockAttached.Attach(event.NewClosure(p.Requester.CancelBlockRequest))
}

func (p *Protocol) ProcessBlock(block *models.Block, src identity.ID) {
	chain, _ := p.chainManager.ProcessCommitment(block.Commitment())
	if chain == nil {
		return
	}

	if targetInstance, exists := p.instancesByChainID[chain.ForkingPoint.ID()]; exists {
		targetInstance.ProcessBlockFromPeer(block, src)
	}
}

func (p *Protocol) Engine() (instance *engine.Engine) {
	p.activeInstanceMutex.RLock()
	defer p.activeInstanceMutex.RUnlock()

	return p.activeInstance
}

func (p *Protocol) importSnapshotFile(fileName string) (err error) {
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
	chainDirectory := p.disk.Path(fmt.Sprintf("%x", lo.PanicOnErr(chainID.Bytes())))

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

func (p *Protocol) instantiateEngines() (chainCount int) {
	for chains := p.settings.Chains().Iterator(); chains.HasNext(); {
		chainID := chains.Next()

		p.instancesByChainID[chainID] = engine.New(DatabaseVersion, p.disk.Path(fmt.Sprintf("%x", lo.PanicOnErr(chainID.Bytes()))), p.Logger)
	}

	return len(p.instancesByChainID)
}

func (p *Protocol) activateMainEngine() (err error) {
	chainID := p.settings.MainChainID()

	mainInstance, exists := p.instancesByChainID[chainID]
	if !exists {
		return errors.Errorf("instance for chain '%s' does not exist", chainID)
	}

	p.activeInstance = mainInstance
	p.Events.Engine.LinkTo(mainInstance.Events)

	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Options //////////////////////////////////////////////////////////////////////////////////////////////////////

func WithBaseDirectory(baseDirectory string) options.Option[Protocol] {
	return func(n *Protocol) {
		n.optsBaseDirectory = baseDirectory
	}
}

func WithSnapshotFileName(snapshot string) options.Option[Protocol] {
	return func(n *Protocol) {
		n.optsSnapshotFileName = snapshot
	}
}

func WithSettingsFileName(settings string) options.Option[Protocol] {
	return func(n *Protocol) {
		n.optsSettingsFileName = settings
	}
}

// func WithSolidificationOptions(opts ...options.Option[solidification.Solidification]) options.Option[Protocol] {
// 	return func(n *Protocol) {
// 		n.optsSolidificationOptions = opts
// 	}
// }

func WithInstanceOptions(opts ...options.Option[engine.Engine]) options.Option[Protocol] {
	return func(n *Protocol) {
		n.optsInstanceOptions = opts
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
