package protocol

import (
	"fmt"
	"os"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/logger"

	"github.com/iotaledger/goshimmer/packages/core/commitment"
	"github.com/iotaledger/goshimmer/packages/core/diskutil"
	"github.com/iotaledger/goshimmer/packages/core/snapshot"
	"github.com/iotaledger/goshimmer/packages/network"
	"github.com/iotaledger/goshimmer/packages/network/gossip"
	"github.com/iotaledger/goshimmer/packages/protocol/database"
	"github.com/iotaledger/goshimmer/packages/protocol/instance"
	"github.com/iotaledger/goshimmer/packages/protocol/instance/engine/congestioncontrol/icca/scheduler"
	"github.com/iotaledger/goshimmer/packages/protocol/instancemanager"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger"
)

// region Protocol /////////////////////////////////////////////////////////////////////////////////////////////////////

type Protocol struct {
	Events *Events

	network         *network.Network
	diskUtil        *diskutil.DiskUtil
	settings        *Settings
	instanceManager *instancemanager.InstanceManager
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
		p.diskUtil = diskutil.New(p.optsBaseDirectory)
		p.settings = NewSettings(p.diskUtil.RelativePath(p.optsSettingsFileName))

		snapshotCommitment, err := p.snapshotCommitment(p.diskUtil.RelativePath(p.optsSnapshotFile))
		if err != nil {
			fmt.Println(errors.Errorf("failed to retrieve snapshot commitment: %w", err))

			panic(err)
		}
		p.instanceManager = instancemanager.New(snapshotCommitment, log)

		p.Events.InstanceManager = p.instanceManager.Events

		p.Events.InstanceManager.Instance.Engine.CongestionControl.Scheduler.BlockScheduled.Attach(event.NewClosure(func(block *scheduler.Block) {
			p.network.SendBlock(block.Block.Block.Block.Block)
		}))

		p.network.Events.Gossip.BlockReceived.Attach(event.NewClosure(func(event *gossip.BlockReceivedEvent) {
			p.instanceManager.DispatchBlockData(event.Data, event.Neighbor)
		}))
	})
}
func (p *Protocol) Instance() (instance *instance.Instance) {
	return p.instanceManager.CurrentInstance()
}

func (p *Protocol) snapshotCommitment(snapshotFile string) (snapshotCommitment *commitment.Commitment, err error) {
	checksum, err := p.diskUtil.FileChecksum(snapshotFile)
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
	if err = p.diskUtil.WithFile(snapshotFile, func(file *os.File) (err error) {
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
