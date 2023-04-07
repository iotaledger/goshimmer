package faucet

import (
	"context"

	"github.com/mr-tron/base58"
	"github.com/pkg/errors"
	"go.uber.org/atomic"
	"go.uber.org/dig"

	walletseed "github.com/iotaledger/goshimmer/client/wallet/packages/seed"
	"github.com/iotaledger/goshimmer/packages/app/blockissuer"
	"github.com/iotaledger/goshimmer/packages/app/faucet"
	"github.com/iotaledger/goshimmer/packages/core/pow"
	"github.com/iotaledger/goshimmer/packages/core/shutdown"
	"github.com/iotaledger/goshimmer/packages/node"
	"github.com/iotaledger/goshimmer/packages/protocol"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/vm/devnetvm"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/vm/devnetvm/indexer"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker"
	"github.com/iotaledger/hive.go/app/daemon"
	"github.com/iotaledger/hive.go/crypto/identity"
	"github.com/iotaledger/hive.go/runtime/event"
)

const (
	// PluginName is the name of the faucet plugin.
	PluginName = "Faucet"
)

var (
	// Plugin is the "plugin" instance of the faucet application.
	Plugin              *node.Plugin
	_faucet             *Faucet
	powVerifier         = pow.New()
	requestChanSize     = 300
	requestChan         = make(chan *faucet.Payload, requestChanSize)
	targetPoWDifficulty int

	// signals that the faucet has initialized itself and can start funding requests.
	initDone atomic.Bool
	deps     = new(dependencies)
)

type dependencies struct {
	dig.In

	Protocol    *protocol.Protocol
	Indexer     *indexer.Indexer
	BlockIssuer *blockissuer.BlockIssuer
}

func init() {
	Plugin = node.NewPlugin(PluginName, deps, node.Disabled, configure, run)
}

// newFaucet gets the faucet component instance the faucet plugin has initialized.
func newFaucet() *Faucet {
	if Parameters.Seed == "" {
		Plugin.LogFatalAndExit("a seed must be defined when enabling the faucet plugin")
	}
	seedBytes, err := base58.Decode(Parameters.Seed)
	if err != nil {
		Plugin.LogFatalfAndExitf("configured seed for the faucet is invalid: %s", err)
	}
	if Parameters.TokensPerRequest <= 0 {
		Plugin.LogFatalfAndExitf("the amount of tokens to fulfill per request must be above zero")
	}
	if Parameters.MaxTransactionBookedAwaitTime <= 0 {
		Plugin.LogFatalfAndExitf("the max transaction booked await time must be more than 0")
	}

	return NewFaucet(walletseed.NewSeed(seedBytes), deps.Protocol, deps.BlockIssuer, deps.Indexer)
}

func configure(plugin *node.Plugin) {
	targetPoWDifficulty = Parameters.PowDifficulty

	deps.Protocol.Events.Engine.Tangle.Booker.BlockTracked.Hook(onBlockProcessed, event.WithWorkerPool(plugin.WorkerPool))
}

func run(plugin *node.Plugin) {
	if err := daemon.BackgroundWorker(PluginName, func(ctx context.Context) {
		defer plugin.LogInfof("Stopping %s ... done", PluginName)

		initDone.Store(true)

		_faucet = newFaucet()
		_faucet.Start(ctx, requestChan)

		close(requestChan)
	}, shutdown.PriorityFaucet); err != nil {
		plugin.Logger().Panicf("Failed to start daemon: %s", err)
	}
}

func OnWebAPIRequest(fundingRequest *faucet.Payload) error {
	// Do not start picking up request while waiting for initialization.
	// If faucet nodes crashes and you restart with a clean db, all previous faucet req blks will be enqueued
	// and addresses will be funded again. Therefore, do not process any faucet request blocks until we are in
	// sync and initialized.
	if !initDone.Load() {
		return errors.New("faucet plugin is not done initializing")
	}

	if err := handleFaucetRequest(fundingRequest); err != nil {
		return err
	}

	return nil
}

func onBlockProcessed(block *booker.Block) {
	// Do not start picking up request while waiting for initialization.
	// If faucet nodes crashes, and you restart with a clean db, all previous faucet req blks will be enqueued
	// and addresses will be funded again. Therefore, do not process any faucet request blocks until we are in
	// sync and initialized.
	if !initDone.Load() {
		return
	}
	if !faucet.IsFaucetReq(block.ModelsBlock) {
		return
	}
	fundingRequest := block.Payload().(*faucet.Payload)

	// pledge mana to requester if not specified in the request
	emptyID := identity.ID{}
	var aManaPledge identity.ID
	if fundingRequest.AccessManaPledgeID() == emptyID {
		aManaPledge = identity.NewID(block.IssuerPublicKey())
	}

	var cManaPledge identity.ID
	if fundingRequest.ConsensusManaPledgeID() == emptyID {
		cManaPledge = identity.NewID(block.IssuerPublicKey())
	}

	_ = handleFaucetRequest(fundingRequest, aManaPledge, cManaPledge)
}

func handleFaucetRequest(fundingRequest *faucet.Payload, pledge ...identity.ID) error {
	addr := fundingRequest.Address()

	if !isFaucetRequestPoWValid(fundingRequest, addr) {
		return errors.New("PoW requirement is not satisfied")
	}

	emptyID := identity.ID{}
	if len(pledge) == 2 {
		if fundingRequest.AccessManaPledgeID() == emptyID {
			fundingRequest.SetAccessManaPledgeID(pledge[0])
		}
		if fundingRequest.ConsensusManaPledgeID() == emptyID {
			fundingRequest.SetConsensusManaPledgeID(pledge[1])
		}
	}

	// finally add it to the faucet to be processed
	requestChan <- fundingRequest

	Plugin.LogInfof("enqueued funding request for address %s", addr.Base58())
	return nil
}

func isFaucetRequestPoWValid(fundingRequest *faucet.Payload, addr devnetvm.Address) bool {
	requestBytes, err := fundingRequest.Bytes()
	if err != nil {
		Plugin.LogInfof("couldn't serialize faucet request: %w", err)
		return false
	}
	// verify PoW
	leadingZeroes, err := powVerifier.LeadingZeros(requestBytes)
	if err != nil {
		Plugin.LogInfof("couldn't verify PoW of funding request for address %s: %w", addr.Base58(), err)
		return false
	}
	if leadingZeroes < targetPoWDifficulty {
		Plugin.LogInfof("funding request for address %s doesn't fulfill PoW requirement %d vs. %d", addr.Base58(), targetPoWDifficulty, leadingZeroes)
		return false
	}

	return true
}
