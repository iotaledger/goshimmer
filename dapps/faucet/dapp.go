package faucet

import (
	"crypto"
	"runtime"
	"sync"
	"time"

	faucet "github.com/iotaledger/goshimmer/dapps/faucet/packages"
	faucetpayload "github.com/iotaledger/goshimmer/dapps/faucet/packages/payload"
	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/message"
	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/tangle"
	"github.com/iotaledger/goshimmer/packages/pow"
	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/plugins/config"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
	"github.com/iotaledger/hive.go/workerpool"
	"github.com/mr-tron/base58"
	flag "github.com/spf13/pflag"
)

const (
	// PluginName is the name of the faucet dApp.
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
)

func init() {
	flag.String(CfgFaucetSeed, "", "the base58 encoded seed of the faucet, must be defined if this dApp is enabled")
	flag.Int(CfgFaucetTokensPerRequest, 1337, "the amount of tokens the faucet should send for each request")
	flag.Int(CfgFaucetMaxTransactionBookedAwaitTimeSeconds, 5, "the max amount of time for a funding transaction to become booked in the value layer")
	flag.Int(CfgFaucetPoWDifficulty, 25, "defines the PoW difficulty for faucet payloads")
	flag.Int(CfgFaucetBlacklistCapacity, 10000, "holds the maximum amount the address blacklist holds")
}

var (
	// App is the "plugin" instance of the faucet application.
	plugin                 *node.Plugin
	pluginOnce             sync.Once
	_faucet                *faucet.Faucet
	faucetOnce             sync.Once
	log                    *logger.Logger
	powVerifier            = pow.New(crypto.BLAKE2b_512)
	fundingWorkerPool      *workerpool.WorkerPool
	fundingWorkerCount     = runtime.GOMAXPROCS(0)
	fundingWorkerQueueSize = 500
)

// App returns the plugin instance of the faucet dApp.
func App() *node.Plugin {
	pluginOnce.Do(func() {
		plugin = node.NewPlugin(PluginName, node.Disabled, configure, run)
	})
	return plugin
}

// Faucet gets the faucet instance the faucet dApp has initialized.
func Faucet() *faucet.Faucet {
	faucetOnce.Do(func() {
		base58Seed := config.Node().GetString(CfgFaucetSeed)
		if len(base58Seed) == 0 {
			log.Fatal("a seed must be defined when enabling the faucet dApp")
		}
		seedBytes, err := base58.Decode(base58Seed)
		if err != nil {
			log.Fatalf("configured seed for the faucet is invalid: %s", err)
		}
		tokensPerRequest := config.Node().GetInt64(CfgFaucetTokensPerRequest)
		if tokensPerRequest <= 0 {
			log.Fatalf("the amount of tokens to fulfill per request must be above zero")
		}
		maxTxBookedAwaitTime := config.Node().GetInt64(CfgFaucetMaxTransactionBookedAwaitTimeSeconds)
		if maxTxBookedAwaitTime <= 0 {
			log.Fatalf("the max transaction booked await time must be more than 0")
		}
		blacklistCapacity := config.Node().GetInt(CfgFaucetBlacklistCapacity)
		_faucet = faucet.New(seedBytes, tokensPerRequest, blacklistCapacity, time.Duration(maxTxBookedAwaitTime)*time.Second)
	})
	return _faucet
}

func configure(*node.Plugin) {
	log = logger.NewLogger(PluginName)
	Faucet()

	fundingWorkerPool = workerpool.New(func(task workerpool.Task) {
		msg := task.Param(0).(*message.Message)
		addr := msg.Payload().(*faucetpayload.Payload).Address()
		msg, txID, err := Faucet().SendFunds(msg)
		if err != nil {
			log.Warnf("couldn't fulfill funding request to %s: %s", addr, err)
			return
		}
		log.Infof("sent funds to address %s via tx %s and msg %s", addr, txID, msg.ID().String())
	}, workerpool.WorkerCount(fundingWorkerCount), workerpool.QueueSize(fundingWorkerQueueSize))

	configureEvents()
}

func run(*node.Plugin) {
	if err := daemon.BackgroundWorker("[Faucet]", func(shutdownSignal <-chan struct{}) {
		fundingWorkerPool.Start()
		defer fundingWorkerPool.Stop()
		<-shutdownSignal
	}, shutdown.PriorityFaucet); err != nil {
		log.Panicf("Failed to start daemon: %s", err)
	}
}

func configureEvents() {
	messagelayer.Tangle().Events.MessageSolid.Attach(events.NewClosure(func(cachedMessage *message.CachedMessage, cachedMessageMetadata *tangle.CachedMessageMetadata) {
		defer cachedMessage.Release()
		defer cachedMessageMetadata.Release()

		msg := cachedMessage.Unwrap()
		if msg == nil || !faucetpayload.IsFaucetReq(msg) {
			return
		}

		fundingRequest := msg.Payload().(*faucetpayload.Payload)
		addr := fundingRequest.Address()
		if Faucet().IsAddressBlacklisted(addr) {
			log.Debugf("can't fund address %s since it is blacklisted", addr)
			return
		}

		// verify PoW
		leadingZeroes, err := powVerifier.LeadingZeros(fundingRequest.Bytes())
		if err != nil {
			log.Warnf("couldn't verify PoW of funding request for address %s", addr)
			return
		}
		targetPoWDifficulty := config.Node().GetInt(CfgFaucetPoWDifficulty)
		if leadingZeroes < targetPoWDifficulty {
			log.Debugf("funding request for address %s doesn't fulfill PoW requirement %d vs. %d", addr, targetPoWDifficulty, leadingZeroes)
			return
		}

		// finally add it to the faucet to be processed
		_, added := fundingWorkerPool.TrySubmit(msg)
		if !added {
			log.Info("dropped funding request for address %s as queue is full", addr)
			return
		}
		log.Infof("enqueued funding request for address %s", addr)
	}))
}
