package faucet

import (
	"runtime"
	"sync"
	"time"

	faucet "github.com/iotaledger/goshimmer/dapps/faucet/packages"
	faucetpayload "github.com/iotaledger/goshimmer/dapps/faucet/packages/payload"
	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/message"
	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/tangle"
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
)

func init() {
	flag.String(CfgFaucetSeed, "", "the base58 encoded seed of the faucet, must be defined if this dApp is enabled")
	flag.Int(CfgFaucetTokensPerRequest, 1337, "the amount of tokens the faucet should send for each request")
	flag.Int(CfgFaucetMaxTransactionBookedAwaitTimeSeconds, 5, "the max amount of time for a funding transaction to become booked in the value layer.")
}

var (
	// App is the "plugin" instance of the faucet application.
	plugin                 *node.Plugin
	pluginOnce             sync.Once
	_faucet                *faucet.Faucet
	faucetOnce             sync.Once
	log                    *logger.Logger
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
		_faucet = faucet.New(seedBytes, tokensPerRequest, time.Duration(maxTxBookedAwaitTime)*time.Second)
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
			log.Errorf("couldn't fulfill funding request to %s: %s", addr, err)
			return
		}
		log.Infof("sent funds to address %s via tx %s and msg %s", addr, txID, msg.Id().String())
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

		addr := msg.Payload().(*faucetpayload.Payload).Address()
		_, added := fundingWorkerPool.TrySubmit(msg)
		if !added {
			log.Info("dropped funding request for address %s as queue is full", addr)
			return
		}
		log.Infof("enqueued funding request for address %s", addr)
	}))
}
