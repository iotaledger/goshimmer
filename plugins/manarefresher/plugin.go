package manarefresher

import (
	"context"
	"fmt"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/node"
	"go.uber.org/dig"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/packages/tangle"
)

var (
	// Plugin is the plugin instance of the manarefresher plugin.
	Plugin    *node.Plugin
	deps      = new(dependencies)
	refresher *Refresher
)

type dependencies struct {
	dig.In
	Local  *peer.Local
	Tangle *tangle.Tangle
}

// minRefreshInterval is the minimum refresh interval allowed for delegated outputs.
const minRefreshInterval = 1 * time.Minute

func init() {
	Plugin = node.NewPlugin("ManaRefresher", deps, node.Enabled, configure, run)
}

// configure events.
func configure(plugin *node.Plugin) {
	plugin.LogInfof("starting node with manarefresher plugin")
	nodeIDPrivateKey, err := deps.Local.Database().LocalPrivateKey()
	if err != nil {
		panic(errors.Wrap(err, "couldn't load private key of node identity"))
	}
	localWallet := newWalletFromPrivateKey(nodeIDPrivateKey)
	refresher = NewRefresher(localWallet, &DelegationReceiver{wallet: localWallet})

	if Parameters.RefreshInterval < minRefreshInterval {
		panic(fmt.Sprintf("manarefresh interval of %d is too small, minimum is %d ", Parameters.RefreshInterval, minRefreshInterval))
	}
}

func run(plugin *node.Plugin) {
	if err := daemon.BackgroundWorker("ManaRefresher-plugin", func(ctx context.Context) {
		ticker := time.NewTicker(Parameters.RefreshInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return

			case <-ticker.C:
				err := refresher.Refresh()
				if err != nil {
					plugin.LogErrorf("couldn't refresh mana: %w", err)
				}
			}
		}
	}, shutdown.PriorityManaRefresher); err != nil {
		plugin.Panicf("Failed to start as daemon: %s", err)
	}
}

// DelegationAddress returns the current mana delegation address of the node.
func DelegationAddress() (ledgerstate.Address, error) {
	if refresher != nil {
		return refresher.receiver.Address(), nil
	}
	return nil, errors.Errorf("manarefresher plugin is disabled")
}

// TotalDelegatedFunds returns the amount of funds delegated to the node.
func TotalDelegatedFunds() (amount uint64) {
	if refresher == nil {
		return 0
	}
	// need to scan to get the most recent
	_ = refresher.receiver.Scan()
	return refresher.receiver.TotalDelegatedFunds()
}

// DelegatedOutputs returns all confirmed, unspent outputs that are delegated to the node.
func DelegatedOutputs() (delegated ledgerstate.Outputs, err error) {
	if refresher == nil {
		err = errors.Errorf("manarefresher plugin is not running, node doesn't process delegated outputs")
		return
	}
	// need to scan to get the most recent
	found := refresher.receiver.Scan()
	if len(found) == 0 {
		err = errors.Errorf("no confirmed delegated outputs found")
		return
	}
	delegated = make(ledgerstate.Outputs, len(found))
	for i := range found {
		delegated[i] = found[i]
	}
	return delegated, nil
}
