package manarefresher

import (
	"fmt"
	"sync"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/node"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/plugins/autopeering/local"
)

var (
	// plugin is the plugin instance of the activity plugin.
	plugin    *node.Plugin
	once      sync.Once
	refresher *Refresher
)

// minRefreshInterval is the minimum refresh interval allowed for delegated outputs.
const minRefreshInterval = 1 // minutes

// Plugin gets the plugin instance.
func Plugin() *node.Plugin {
	once.Do(func() {
		plugin = node.NewPlugin("ManaRefresher", node.Enabled, configure, run)
	})
	return plugin
}

// configure events
func configure(_ *node.Plugin) {
	plugin.LogInfof("starting node with manarefresher plugin")
	nodeIDPrivateKey, err := local.GetInstance().Database().LocalPrivateKey()
	if err != nil {
		panic(errors.Wrap(err, "couldn't load private key of node identity"))
	}
	localWallet := newWalletFromPrivateKey(nodeIDPrivateKey)
	refresher = NewRefresher(localWallet, &DelegationReceiver{wallet: localWallet})

	if Parameters.RefreshInterval < minRefreshInterval {
		panic(fmt.Sprintf("manarefresh interval of %d minutes is too small, minimum is %d minutes", Parameters.RefreshInterval, minRefreshInterval))
	}
}

func run(_ *node.Plugin) {
	if err := daemon.BackgroundWorker("ManaRefresher-plugin", func(shutdownSignal <-chan struct{}) {
		ticker := time.NewTicker(time.Duration(Parameters.RefreshInterval) * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-shutdownSignal:
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
