package faucet

import (
	"runtime"
	"sync"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/datastructure/orderedmap"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
	"github.com/iotaledger/hive.go/workerpool"
	"github.com/mr-tron/base58"
	flag "github.com/spf13/pflag"
	"go.uber.org/atomic"

	walletseed "github.com/iotaledger/goshimmer/client/wallet/packages/seed"
	"github.com/iotaledger/goshimmer/packages/faucet"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/mana"
	"github.com/iotaledger/goshimmer/packages/pow"
	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/goshimmer/plugins/config"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
)

const (
	// PluginName is the name of the faucet plugin.
	PluginName = "Faucet"

	// CfgFaucetSeed defines the base58 encoded seed the faucet uses.
	CfgFaucetSeed = "faucet.seed"
	// CfgFaucetTokensPerRequest defines the amount of tokens the faucet should send for each request.
	CfgFaucetTokensPerRequest = "faucet.tokensPerRequest"
	// CfgFaucetMaxTransactionBookedAwaitTimeSeconds defines the time to await for the transaction fulfilling a funding request
	// to become booked in the value layer.
	CfgFaucetMaxTransactionBookedAwaitTimeSeconds = "faucet.maxTransactionBookedAwaitTimeSeconds"
	// CfgFaucetPoWDifficulty defines the PoW difficulty for faucet payloads.
	CfgFaucetPoWDifficulty = "faucet.powDifficulty"
	// CfgFaucetBlacklistCapacity holds the maximum amount the address blacklist holds.
	// An address for which a funding was done in the past is added to the blacklist and eventually is removed from it.
	CfgFaucetBlacklistCapacity = "faucet.blacklistCapacity"
	// CfgFaucetPreparedOutputsCount is the number of outputs the faucet prepares for requests.
	CfgFaucetPreparedOutputsCount = "faucet.preparedOutputsCount"
	// CfgFaucetStartIndex defines from which address index the faucet should start gathering outputs.
	CfgFaucetStartIndex = "faucet.startIndex"
)

func init() {
	flag.String(CfgFaucetSeed, "", "the base58 encoded seed of the faucet, must be defined if this faucet is enabled")
	flag.Int(CfgFaucetTokensPerRequest, 1000000, "the amount of tokens the faucet should send for each request")
	flag.Int(CfgFaucetMaxTransactionBookedAwaitTimeSeconds, 5, "the max amount of time for a funding transaction to become booked in the value layer")
	flag.Int(CfgFaucetPoWDifficulty, 22, "defines the PoW difficulty for faucet payloads")
	flag.Int(CfgFaucetBlacklistCapacity, 10000, "holds the maximum amount the address blacklist holds")
	flag.Int(CfgFaucetPreparedOutputsCount, 126, "number of outputs the faucet prepares")
	flag.Int(CfgFaucetStartIndex, 0, "address index to start faucet with")
}

var (
	// Plugin is the "plugin" instance of the faucet application.
	plugin                 *node.Plugin
	pluginOnce             sync.Once
	_faucet                *StateManager
	faucetOnce             sync.Once
	log                    *logger.Logger
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
		base58Seed := config.Node().String(CfgFaucetSeed)
		if base58Seed == "" {
			log.Fatal("a seed must be defined when enabling the faucet plugin")
		}
		seedBytes, err := base58.Decode(base58Seed)
		if err != nil {
			log.Fatalf("configured seed for the faucet is invalid: %s", err)
		}
		tokensPerRequest := config.Node().Int64(CfgFaucetTokensPerRequest)
		if tokensPerRequest <= 0 {
			log.Fatalf("the amount of tokens to fulfill per request must be above zero")
		}
		maxTxBookedAwaitTime := config.Node().Int64(CfgFaucetMaxTransactionBookedAwaitTimeSeconds)
		if maxTxBookedAwaitTime <= 0 {
			log.Fatalf("the max transaction booked await time must be more than 0")
		}
		preparedOutputsCount := config.Node().Int(CfgFaucetPreparedOutputsCount)
		if preparedOutputsCount <= 0 {
			log.Fatalf("the number of faucet prepared outputs should be more than 0")
		}
		_faucet = NewStateManager(
			uint64(tokensPerRequest),
			walletseed.NewSeed(seedBytes),
			uint64(preparedOutputsCount),
			time.Duration(maxTxBookedAwaitTime)*time.Second,
		)
	})
	return _faucet
}

func configure(*node.Plugin) {
	log = logger.NewLogger(PluginName)
	targetPoWDifficulty = config.Node().Int(CfgFaucetPoWDifficulty)
	startIndex = config.Node().Int(CfgFaucetStartIndex)
	blacklist = orderedmap.New()
	blacklistCapacity = config.Node().Int(CfgFaucetBlacklistCapacity)
	Faucet()

	fundingWorkerPool = workerpool.NewNonBlockingQueuedWorkerPool(func(task workerpool.Task) {
		msg := task.Param(0).(*tangle.Message)
		addr := msg.Payload().(*faucet.Request).Address()
		msg, txID, err := Faucet().FulFillFundingRequest(msg)
		if err != nil {
			log.Warnf("couldn't fulfill funding request to %s: %s", addr.Base58(), err)
			return
		}
		log.Infof("sent funds to address %s via tx %s and msg %s", addr.Base58(), txID, msg.ID())
	}, workerpool.WorkerCount(fundingWorkerCount), workerpool.QueueSize(fundingWorkerQueueSize))

	configureEvents()
}

func run(*node.Plugin) {
	if err := daemon.BackgroundWorker(PluginName, func(shutdownSignal <-chan struct{}) {
		defer log.Infof("Stopping %s ... done", PluginName)

		log.Info("Waiting for node to become synced...")
		if !waitUntilSynced(shutdownSignal) {
			return
		}
		log.Info("Waiting for node to become synced... done")

		log.Info("Waiting for node to have sufficient access mana")
		if err := waitForMana(shutdownSignal); err != nil {
			log.Errorf("failed to get sufficient access mana: %s", err)
			return
		}
		log.Info("Waiting for node to have sufficient access mana... done")

		log.Infof("Deriving faucet state from the ledger...")
		// determine state, prepare more outputs if needed
		if err := Faucet().DeriveStateFromTangle(startIndex); err != nil {
			log.Errorf("failed to derive state: %s", err)
			return
		}
		log.Info("Deriving faucet state from the ledger... done")

		defer fundingWorkerPool.Stop()
		initDone.Store(true)

		<-shutdownSignal
		log.Infof("Stopping %s ...", PluginName)
	}, shutdown.PriorityFaucet); err != nil {
		log.Panicf("Failed to start daemon: %s", err)
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
		plugin.LogDebugf("insufficient access mana: %f < %f", aMana, tangle.MinMana)
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
				log.Infof("couldn't verify PoW of funding request for address %s", addr.Base58())
				return
			}

			if leadingZeroes < targetPoWDifficulty {
				log.Infof("funding request for address %s doesn't fulfill PoW requirement %d vs. %d", addr.Base58(), targetPoWDifficulty, leadingZeroes)
				return
			}

			if IsAddressBlackListed(addr) {
				log.Infof("can't fund address %s since it is blacklisted", addr.Base58())
				return
			}

			// finally add it to the faucet to be processed
			_, added := fundingWorkerPool.TrySubmit(message)
			if !added {
				RemoveAddressFromBlacklist(addr)
				log.Info("dropped funding request for address %s as queue is full", addr.Base58())
				return
			}
			log.Infof("enqueued funding request for address %s", addr.Base58())
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
