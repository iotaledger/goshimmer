package protocol

import (
	"fmt"
	"os"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/logger"

	"github.com/iotaledger/goshimmer/packages/core/commitment"
	"github.com/iotaledger/goshimmer/packages/core/diskutil"
	"github.com/iotaledger/goshimmer/packages/core/snapshot"
	"github.com/iotaledger/goshimmer/packages/network"
	"github.com/iotaledger/goshimmer/packages/protocol/chainmanager"
	"github.com/iotaledger/goshimmer/packages/protocol/database"
	"github.com/iotaledger/goshimmer/packages/protocol/instance"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger"
)

// region Protocol /////////////////////////////////////////////////////////////////////////////////////////////////////

type Protocol struct {
	Events *Events

	network            *network.Network
	disk               *diskutil.DiskUtil
	settings           *Settings
	chainManager       *chainmanager.Manager
	activeInstance     *instance.Instance
	instancesByChainID map[commitment.ID]*instance.Instance
	// solidification  *solidification.Solidification

	optsBaseDirectory    string
	optsSettingsFileName string
	optsSnapshotFile     string
	optsDBManagerOptions []options.Option[database.Manager]
	// optsSolidificationOptions []options.Option[solidification.Solidification]

	*logger.Logger
}

func New(networkInstance *network.Network, log *logger.Logger, opts ...options.Option[Protocol]) (protocol *Protocol) {
	return options.Apply(&Protocol{
		Events: NewEvents(),

		instancesByChainID:   make(map[commitment.ID]*instance.Instance),
		optsBaseDirectory:    "",
		optsSettingsFileName: "settings.bin",
		optsSnapshotFile:     "snapshot.bin",

		Logger: log,
	}, opts, func(p *Protocol) {
		p.network = networkInstance
		p.disk = diskutil.New(p.optsBaseDirectory)
		p.settings = NewSettings(p.disk.Path(p.optsSettingsFileName))

		if err := p.importSnapshot(p.disk.Path(p.optsSnapshotFile)); err != nil {
			panic(err)
		}

		for chains := p.settings.Chains().Iterator(); chains.HasNext(); {
			chainID := chains.Next()

			p.instancesByChainID[chainID] = instance.New(p.disk.Path(fmt.Sprintf("%x", chainID.Bytes())), p.Logger)
		}

		if len(p.instancesByChainID) == 0 {
			panic("no chains found - please provide a snapshot file")
		}

		p.activeInstance = p.instancesByChainID[p.settings.ActiveChainID()]

		// CHAIN SPECIFIC //////////////////////////////////////////////////////////////////////////////////////////////

		// add instance to instancesByChainID

		// p.chainManager = chainmanager.NewManager(snapshotCommitment)

		//		p.instanceManager = instancemanager.New(p.disk, log)

		// p.Events.InstanceManager = p.instanceManager.Events
		//
		// p.Events.InstanceManager.Instance.Engine.CongestionControl.Scheduler.BlockScheduled.Attach(event.NewClosure(func(block *scheduler.Block) {
		// 	p.network.SendBlock(block.Block.Block.Block.Block)
		// }))
		//
		// p.network.Events.Gossip.BlockReceived.Attach(event.NewClosure(func(event *gossip.BlockReceivedEvent) {
		// 	p.instanceManager.DispatchBlockData(event.Data, event.Neighbor)
		// }))
	})
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
	p.settings.SetActiveChainID(chainID)
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
