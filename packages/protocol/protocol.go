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

	network        *network.Network
	disk           *diskutil.DiskUtil
	settings       *Settings
	chainManager   *chainmanager.Manager
	activeInstance *instance.Instance
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

		optsBaseDirectory:    "",
		optsSettingsFileName: "settings.bin",
		optsSnapshotFile:     "snapshot.bin",

		Logger: log,
	}, opts, func(p *Protocol) {
		p.network = networkInstance
		p.disk = diskutil.New(p.optsBaseDirectory)
		p.settings = NewSettings(p.disk.Path(p.optsSettingsFileName))

		// GLOBAL //////////////////////////////////////////////////////////////////////////////////////////////////////

		nodeSnapshotFile := p.disk.Path(p.optsSnapshotFile)
		snapshotCommitment, err := p.snapshotCommitment(nodeSnapshotFile)
		if err != nil {
			fmt.Println(errors.Errorf("failed to retrieve snapshot commitment: %w", err))

			panic(err)
		}

		// CHAIN SPECIFIC //////////////////////////////////////////////////////////////////////////////////////////////

		// create WorkingDirectory for new chain
		chainDirectory := p.disk.Path(fmt.Sprintf("%x", snapshotCommitment.ID().Bytes()))
		if err = p.disk.CreateDir(chainDirectory); err != nil {
			panic(err)
		}

		// copy over current snapshot to that working directory
		chainSnapshotFile := diskutil.New(chainDirectory).Path("snapshot.bin")
		if err = p.disk.CopyFile(nodeSnapshotFile, chainSnapshotFile); err != nil {
			panic(err)
		}

		// start new instance with that working directory
		p.activeInstance = instance.New(chainDirectory, snapshotCommitment, log)

		fmt.Println(diskutil.New(chainDirectory).Path("snapshot.bin"))

		// add instance to instancesByChainID

		p.chainManager = chainmanager.NewManager(snapshotCommitment)

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

func (p *Protocol) snapshotCommitment(snapshotFile string) (snapshotCommitment *commitment.Commitment, err error) {
	checksum, err := p.disk.FileChecksum(snapshotFile)
	if err != nil {
		return nil, errors.Errorf("failed to calculate checksum of snapshot file '%s': %w", snapshotFile, err)
	}

	if checksum == p.settings.SnapshotChecksum() {
		return p.settings.SnapshotCommitment(), nil
	}

	snapshotHeader, err := p.readSnapshotHeader(snapshotFile)
	if err != nil {
		return nil, errors.Errorf("failed to read snapshot commitment: %w", err)
	}

	p.settings.SetSnapshotChecksum(checksum)
	p.settings.SetSnapshotCommitment(snapshotHeader.LatestECRecord)
	p.settings.Persist()

	return snapshotHeader.LatestECRecord, nil
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
