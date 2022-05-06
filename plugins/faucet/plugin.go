package faucet

import (
	"context"
	"runtime"
	"sync"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/generics/orderedmap"
	"github.com/iotaledger/hive.go/node"
	"github.com/iotaledger/hive.go/workerpool"
	"github.com/mr-tron/base58"
	"go.uber.org/atomic"
	"go.uber.org/dig"

	walletseed "github.com/iotaledger/goshimmer/client/wallet/packages/seed"
	"github.com/iotaledger/goshimmer/packages/faucet"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/mana"
	"github.com/iotaledger/goshimmer/packages/pow"
	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
)

const (
	// PluginName is the name of the faucet plugin.
	PluginName = "Faucet"
)

var (
	// Plugin is the "plugin" instance of the faucet application.
	Plugin                   *node.Plugin
	_faucet                  *StateManager
	powVerifier              = pow.New()
	fundingWorkerPool        *workerpool.NonBlockingQueuedWorkerPool
	fundingWorkerCount       = runtime.GOMAXPROCS(0)
	fundingWorkerQueueSize   = 500
	preparingWorkerPool      *workerpool.NonBlockingQueuedWorkerPool
	preparingWorkerCount     = runtime.GOMAXPROCS(0)
	preparingWorkerQueueSize = MaxFaucetOutputsCount + 1
	targetPoWDifficulty      int
	// blacklist makes sure that an address might only request tokens once.
	blacklist         *orderedmap.OrderedMap[string, bool]
	blacklistCapacity int
	blackListMutex    sync.RWMutex
	// signals that the faucet has initialized itself and can start funding requests.
	initDone atomic.Bool

	waitForManaWindow = 5 * time.Second
	deps              = new(dependencies)
)

type dependencies struct {
	dig.In

	Local  *peer.Local
	Tangle *tangle.Tangle
}

func init() {
	Plugin = node.NewPlugin(PluginName, deps, node.Disabled, configure, run)
}

// newFaucet gets the faucet component instance the faucet plugin has initialized.
func newFaucet() *StateManager {
	if Parameters.Seed == "" {
		Plugin.LogFatal("a seed must be defined when enabling the faucet plugin")
	}
	seedBytes, err := base58.Decode(Parameters.Seed)
	if err != nil {
		Plugin.LogFatalf("configured seed for the faucet is invalid: %s", err)
	}
	if Parameters.TokensPerRequest <= 0 {
		Plugin.LogFatalf("the amount of tokens to fulfill per request must be above zero")
	}
	if Parameters.MaxTransactionBookedAwaitTime <= 0 {
		Plugin.LogFatalf("the max transaction booked await time must be more than 0")
	}
	if Parameters.SupplyOutputsCount <= 0 {
		Plugin.LogFatalf("the number of faucet supply outputs should be more than 0")
	}
	if Parameters.SplittingMultiplier <= 0 {
		Plugin.LogFatalf("the number of outputs for each supply transaction during funds splitting should be more than 0")
	}
	if Parameters.GenesisTokenAmount <= 0 {
		Plugin.LogFatalf("the total supply should be more than 0")
	}
	return NewStateManager(
		uint64(Parameters.TokensPerRequest),
		walletseed.NewSeed(seedBytes),
		uint64(Parameters.SupplyOutputsCount),
		uint64(Parameters.SplittingMultiplier),

		Parameters.MaxTransactionBookedAwaitTime,
	)
}

func configure(plugin *node.Plugin) {
	targetPoWDifficulty = Parameters.PowDifficulty
	blacklist = orderedmap.New[string, bool]()
	blacklistCapacity = Parameters.BlacklistCapacity
	_faucet = newFaucet()

	fundingWorkerPool = workerpool.NewNonBlockingQueuedWorkerPool(func(task workerpool.Task) {
		msg := task.Param(0).(*tangle.Message)
		addr := msg.Payload().(*faucet.Payload).Address()
		msg, txID, err := _faucet.FulFillFundingRequest(msg)
		if err != nil {
			plugin.LogWarnf("couldn't fulfill funding request to %s: %s", addr.Base58(), err)
			return
		}
		plugin.LogInfof("sent funds to address %s via tx %s and msg %s", addr.Base58(), txID, msg.ID())
	}, workerpool.WorkerCount(fundingWorkerCount), workerpool.QueueSize(fundingWorkerQueueSize))

	preparingWorkerPool = workerpool.NewNonBlockingQueuedWorkerPool(_faucet.prepareTransactionTask,
		workerpool.WorkerCount(preparingWorkerCount), workerpool.QueueSize(preparingWorkerQueueSize))

	configureEvents()
}

func run(plugin *node.Plugin) {
	if err := daemon.BackgroundWorker(PluginName, func(ctx context.Context) {
		defer plugin.LogInfof("Stopping %s ... done", PluginName)

		plugin.LogInfo("Waiting for node to become synced...")
		if !waitUntilSynced(ctx) {
			return
		}
		plugin.LogInfo("Waiting for node to become synced... done")

		plugin.LogInfo("Waiting for node to have sufficient access mana")
		if err := waitForMana(ctx); err != nil {
			plugin.LogErrorf("failed to get sufficient access mana: %s", err)
			return
		}
		plugin.LogInfo("Waiting for node to have sufficient access mana... done")

		plugin.LogInfof("Deriving faucet state from the ledger...")

		// determine state, prepare more outputs if needed
		if err := _faucet.DeriveStateFromTangle(ctx); err != nil {
			plugin.LogErrorf("failed to derive state: %s", err)
			return
		}
		plugin.LogInfo("Deriving faucet state from the ledger... done")

		defer fundingWorkerPool.Stop()
		defer preparingWorkerPool.Stop()

		initDone.Store(true)

		<-ctx.Done()
		plugin.LogInfof("Stopping %s ...", PluginName)
	}, shutdown.PriorityFaucet); err != nil {
		plugin.Logger().Panicf("Failed to start daemon: %s", err)
	}
}

func waitUntilSynced(ctx context.Context) bool {
	synced := make(chan struct{}, 1)
	closure := events.NewClosure(func(e *tangle.SyncChangedEvent) {
		if e.Synced {
			// use non-blocking send to prevent deadlocks in rare cases when the SyncedChanged events is spammed
			select {
			case synced <- struct{}{}:
			default:
			}
		}
	})
	deps.Tangle.TimeManager.Events.SyncChanged.Attach(closure)
	defer deps.Tangle.TimeManager.Events.SyncChanged.Detach(closure)

	// if we are already synced, there is no need to wait for the event
	if deps.Tangle.TimeManager.Synced() {
		return true
	}

	// block until we are either synced or shutting down
	select {
	case <-synced:
		return true
	case <-ctx.Done():
		return false
	}
}

func waitForMana(ctx context.Context) error {
	nodeID := deps.Tangle.Options.Identity.ID()
	for {
		// stop polling, if we are shutting down
		select {
		case <-ctx.Done():
			return errors.New("faucet shutting down")
		default:
		}

		aMana, _, err := messagelayer.GetAccessMana(nodeID)
		// ignore ErrNodeNotFoundInBaseManaVector and treat it as 0 mana
		if err != nil && !errors.Is(err, mana.ErrNodeNotFoundInBaseManaVector) {
			return err
		}
		if aMana >= tangle.MinMana {
			return nil
		}
		Plugin.LogDebugf("insufficient access mana: %f < %f", aMana, tangle.MinMana)
		time.Sleep(waitForManaWindow)
	}
}

func configureEvents() {
	deps.Tangle.ApprovalWeightManager.Events.MessageProcessed.Attach(events.NewClosure(func(messageID tangle.MessageID) {
		// Do not start picking up request while waiting for initialization.
		// If faucet nodes crashes and you restart with a clean db, all previous faucet req msgs will be enqueued
		// and addresses will be funded again. Therefore, do not process any faucet request messages until we are in
		// sync and initialized.
		if !initDone.Load() {
			return
		}
		deps.Tangle.Storage.Message(messageID).Consume(func(message *tangle.Message) {
			if !faucet.IsFaucetReq(message) {
				return
			}
			fundingRequest := message.Payload().(*faucet.Payload)
			addr := fundingRequest.Address()

			// verify PoW
			leadingZeroes, err := powVerifier.LeadingZeros(fundingRequest.Bytes())
			if err != nil {
				Plugin.LogInfof("couldn't verify PoW of funding request for address %s", addr.Base58())
				return
			}

			if leadingZeroes < targetPoWDifficulty {
				Plugin.LogInfof("funding request for address %s doesn't fulfill PoW requirement %d vs. %d", addr.Base58(), targetPoWDifficulty, leadingZeroes)
				return
			}

			if IsAddressBlackListed(addr) {
				Plugin.LogInfof("can't fund address %s since it is blacklisted", addr.Base58())
				return
			}

			// finally add it to the faucet to be processed
			_, added := fundingWorkerPool.TrySubmit(message)
			if !added {
				RemoveAddressFromBlacklist(addr)
				Plugin.LogInfof("dropped funding request for address %s as queue is full", addr.Base58())
				return
			}
			Plugin.LogInfof("enqueued funding request for address %s", addr.Base58())
		})
	}))
}

// IsAddressBlackListed returns if an address is blacklisted.
// adds the given address to the blacklist and removes the oldest blacklist entry if it would go over capacity.
func IsAddressBlackListed(address ledgerstate.Address) bool {
	blackListMutex.Lock()
	defer blackListMutex.Unlock()

	// see if it was already blacklisted
	_, blacklisted := blacklist.Get(address.Base58())

	if blacklisted {
		return true
	}

	// add it to the blacklist
	blacklist.Set(address.Base58(), true)
	if blacklist.Size() > blacklistCapacity {
		var headKey string
		blacklist.ForEach(func(key string, value bool) bool {
			headKey = key
			return false
		})
		blacklist.Delete(headKey)
	}

	return false
}

// RemoveAddressFromBlacklist removes an address from the blacklist.
func RemoveAddressFromBlacklist(address ledgerstate.Address) {
	blackListMutex.Lock()
	defer blackListMutex.Unlock()

	blacklist.Delete(address.Base58())
}
