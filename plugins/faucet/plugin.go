package faucet

import (
	"runtime"
	"sync"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/datastructure/orderedmap"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/node"
	"github.com/iotaledger/hive.go/workerpool"
	"github.com/mr-tron/base58"
	"go.uber.org/atomic"

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
	plugin                 *node.Plugin
	pluginOnce             sync.Once
	_faucet                *StateManager
	faucetOnce             sync.Once
	powVerifier            = pow.New()
	fundingWorkerPool      *workerpool.NonBlockingQueuedWorkerPool
	fundingWorkerCount     = runtime.GOMAXPROCS(0)
	fundingWorkerQueueSize = 500
	targetPoWDifficulty    int
	startIndex             int
	// blacklist makes sure that an address might only request tokens once.
	blacklist         *orderedmap.OrderedMap
	blacklistCapacity int
	blackListMutex    sync.RWMutex
	// signals that the faucet has initialized itself and can start funding requests
	initDone atomic.Bool

	waitForManaWindow = 5 * time.Second
)

// Plugin returns the plugin instance of the faucet plugin.
func Plugin() *node.Plugin {
	pluginOnce.Do(func() {
		plugin = node.NewPlugin(PluginName, node.Disabled, configure, run)
	})
	return plugin
}

// Faucet gets the faucet component instance the faucet plugin has initialized.
func Faucet() *StateManager {
	faucetOnce.Do(func() {
		if Parameters.Seed == "" {
			Plugin().LogFatal("a seed must be defined when enabling the faucet plugin")
		}
		seedBytes, err := base58.Decode(Parameters.Seed)
		if err != nil {
			Plugin().LogFatalf("configured seed for the faucet is invalid: %s", err)
		}
		if Parameters.TokensPerRequest <= 0 {
			Plugin().LogFatalf("the amount of tokens to fulfill per request must be above zero")
		}
		if Parameters.MaxTransactionBookedAwaitTimeSeconds <= 0 {
			Plugin().LogFatalf("the max transaction booked await time must be more than 0")
		}
		if Parameters.PreparedOutputsCount <= 0 {
			Plugin().LogFatalf("the number of faucet prepared outputs should be more than 0")
		}
		_faucet = NewStateManager(
			uint64(Parameters.TokensPerRequest),
			walletseed.NewSeed(seedBytes),
			uint64(Parameters.PreparedOutputsCount),
			time.Duration(Parameters.MaxTransactionBookedAwaitTimeSeconds)*time.Second,
		)
	})
	return _faucet
}

func configure(*node.Plugin) {
	targetPoWDifficulty = Parameters.PowDifficulty
	startIndex = Parameters.StartIndex
	blacklist = orderedmap.New()
	blacklistCapacity = Parameters.BlacklistCapacity
	Faucet()

	fundingWorkerPool = workerpool.NewNonBlockingQueuedWorkerPool(func(task workerpool.Task) {
		msg := task.Param(0).(*tangle.Message)
		addr := msg.Payload().(*faucet.Request).Address()
		msg, txID, err := Faucet().FulFillFundingRequest(msg)
		if err != nil {
			Plugin().LogWarnf("couldn't fulfill funding request to %s: %s", addr.Base58(), err)
			return
		}
		Plugin().LogInfof("sent funds to address %s via tx %s and msg %s", addr.Base58(), txID, msg.ID())
	}, workerpool.WorkerCount(fundingWorkerCount), workerpool.QueueSize(fundingWorkerQueueSize))

	configureEvents()
}

func run(*node.Plugin) {
	if err := daemon.BackgroundWorker(PluginName, func(shutdownSignal <-chan struct{}) {
		defer Plugin().LogInfof("Stopping %s ... done", PluginName)

		Plugin().LogInfo("Waiting for node to become synced...")
		if !waitUntilSynced(shutdownSignal) {
			return
		}
		Plugin().LogInfo("Waiting for node to become synced... done")

		Plugin().LogInfo("Waiting for node to have sufficient access mana")
		if err := waitForMana(shutdownSignal); err != nil {
			Plugin().LogErrorf("failed to get sufficient access mana: %s", err)
			return
		}
		Plugin().LogInfo("Waiting for node to have sufficient access mana... done")

		Plugin().LogInfof("Deriving faucet state from the ledger...")
		// determine state, prepare more outputs if needed
		if err := Faucet().DeriveStateFromTangle(startIndex); err != nil {
			Plugin().LogErrorf("failed to derive state: %s", err)
			return
		}
		Plugin().LogInfo("Deriving faucet state from the ledger... done")

		defer fundingWorkerPool.Stop()
		initDone.Store(true)

		<-shutdownSignal
		Plugin().LogInfof("Stopping %s ...", PluginName)
	}, shutdown.PriorityFaucet); err != nil {
		Plugin().Logger().Panicf("Failed to start daemon: %s", err)
	}
}

func waitUntilSynced(shutdownSignal <-chan struct{}) bool {
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
	messagelayer.Tangle().TimeManager.Events.SyncChanged.Attach(closure)
	defer messagelayer.Tangle().TimeManager.Events.SyncChanged.Detach(closure)

	// if we are already synced, there is no need to wait for the event
	if messagelayer.Tangle().TimeManager.Synced() {
		return true
	}

	// block until we are either synced or shutting down
	select {
	case <-synced:
		return true
	case <-shutdownSignal:
		return false
	}
}

func waitForMana(shutdownSignal <-chan struct{}) error {
	nodeID := messagelayer.Tangle().Options.Identity.ID()
	for {
		// stop polling, if we are shutting down
		select {
		case <-shutdownSignal:
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
		Plugin().LogDebugf("insufficient access mana: %f < %f", aMana, tangle.MinMana)
		time.Sleep(waitForManaWindow)
	}
}

func configureEvents() {
	messagelayer.Tangle().ConsensusManager.Events.MessageOpinionFormed.Attach(events.NewClosure(func(messageID tangle.MessageID) {
		// Do not start picking up request while waiting for initialization.
		// If faucet nodes crashes and you restart with a clean db, all previous faucet req msgs will be enqueued
		// and addresses will be funded again. Therefore, do not process any faucet request messages until we are in
		// sync and initialized.
		if !initDone.Load() {
			return
		}
		messagelayer.Tangle().Storage.Message(messageID).Consume(func(message *tangle.Message) {
			if !faucet.IsFaucetReq(message) {
				return
			}
			fundingRequest := message.Payload().(*faucet.Request)
			addr := fundingRequest.Address()

			// verify PoW
			leadingZeroes, err := powVerifier.LeadingZeros(fundingRequest.Bytes())
			if err != nil {
				Plugin().LogInfof("couldn't verify PoW of funding request for address %s", addr.Base58())
				return
			}

			if leadingZeroes < targetPoWDifficulty {
				Plugin().LogInfof("funding request for address %s doesn't fulfill PoW requirement %d vs. %d", addr.Base58(), targetPoWDifficulty, leadingZeroes)
				return
			}

			if IsAddressBlackListed(addr) {
				Plugin().LogInfof("can't fund address %s since it is blacklisted", addr.Base58())
				return
			}

			// finally add it to the faucet to be processed
			_, added := fundingWorkerPool.TrySubmit(message)
			if !added {
				RemoveAddressFromBlacklist(addr)
				Plugin().LogInfo("dropped funding request for address %s as queue is full", addr.Base58())
				return
			}
			Plugin().LogInfof("enqueued funding request for address %s", addr.Base58())
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
		var headKey interface{}
		blacklist.ForEach(func(key, value interface{}) bool {
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
