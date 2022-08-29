package faucet

import (
	"context"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/core/autopeering/peer"
	"github.com/iotaledger/hive.go/core/daemon"
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/iotaledger/hive.go/core/node"
	"github.com/mr-tron/base58"
	"go.uber.org/atomic"
	"go.uber.org/dig"

	"github.com/iotaledger/goshimmer/packages/core/ledger/vm/devnetvm"
	"github.com/iotaledger/goshimmer/packages/core/ledger/vm/devnetvm/indexer"
	"github.com/iotaledger/goshimmer/packages/core/mana"
	"github.com/iotaledger/goshimmer/packages/core/pow"
	"github.com/iotaledger/goshimmer/packages/core/tangleold"

	walletseed "github.com/iotaledger/goshimmer/client/wallet/packages/seed"
	"github.com/iotaledger/goshimmer/packages/app/faucet"
	"github.com/iotaledger/goshimmer/packages/core/bootstrapmanager"
	"github.com/iotaledger/goshimmer/packages/node/shutdown"
	"github.com/iotaledger/goshimmer/plugins/blocklayer"
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
	initDone     atomic.Bool
	bootstrapped chan bool

	waitForManaWindow = 5 * time.Second
	deps              = new(dependencies)
)

type dependencies struct {
	dig.In

	Local            *peer.Local
	Tangle           *tangleold.Tangle
	BootstrapManager *bootstrapmanager.Manager
	Indexer          *indexer.Indexer
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
		Plugin.LogFatalfAndExit("configured seed for the faucet is invalid: %s", err)
	}
	if Parameters.TokensPerRequest <= 0 {
		Plugin.LogFatalfAndExit("the amount of tokens to fulfill per request must be above zero")
	}
	if Parameters.MaxTransactionBookedAwaitTime <= 0 {
		Plugin.LogFatalfAndExit("the max transaction booked await time must be more than 0")
	}

	return NewFaucet(walletseed.NewSeed(seedBytes))
}

func configure(plugin *node.Plugin) {
	targetPoWDifficulty = Parameters.PowDifficulty
	bootstrapped = make(chan bool, 1)

	configureEvents()
}

func run(plugin *node.Plugin) {
	if err := daemon.BackgroundWorker(PluginName, func(ctx context.Context) {
		defer plugin.LogInfof("Stopping %s ... done", PluginName)

		plugin.LogInfo("Waiting for node to become bootstrapped...")
		if !waitUntilBootstrapped(ctx) {
			return
		}
		plugin.LogInfo("Waiting for node to become bootstrapped... done")

		plugin.LogInfo("Waiting for node to have sufficient access mana")
		if err := checkForMana(ctx); err != nil {
			plugin.LogErrorf("failed to get sufficient access mana: %s", err)
			return
		}
		plugin.LogInfo("Waiting for node to have sufficient access mana... done")

		initDone.Store(true)

		_faucet = newFaucet()
		_faucet.Start(ctx, requestChan)

		close(requestChan)
	}, shutdown.PriorityFaucet); err != nil {
		plugin.Logger().Panicf("Failed to start daemon: %s", err)
	}
}

func waitUntilBootstrapped(ctx context.Context) bool {
	// if we are already bootstrapped, there is no need to wait for the event
	if deps.BootstrapManager.Bootstrapped() {
		return true
	}

	// block until we are either bootstrapped or shutting down
	select {
	case <-bootstrapped:
		return true
	case <-ctx.Done():
		return false
	}
}

func checkForMana(ctx context.Context) error {
	nodeID := deps.Tangle.Options.Identity.ID()

	aMana, _, err := blocklayer.GetAccessMana(nodeID)
	// ignore ErrNodeNotFoundInBaseManaVector and treat it as 0 mana
	if err != nil && !errors.Is(err, mana.ErrNodeNotFoundInBaseManaVector) {
		return err
	}
	if aMana < tangleold.MinMana {
		return errors.Errorf("insufficient access mana: %f < %f", aMana, tangleold.MinMana)
	}
	return nil
}

func configureEvents() {
	deps.Tangle.ApprovalWeightManager.Events.BlockProcessed.Attach(event.NewClosure(func(event *tangleold.BlockProcessedEvent) {
		onBlockProcessed(event.BlockID)
	}))
	deps.BootstrapManager.Events.Bootstrapped.Attach(event.NewClosure(func(event *bootstrapmanager.BootstrappedEvent) {
		bootstrapped <- true
	}))
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

func onBlockProcessed(blockID tangleold.BlockID) {
	// Do not start picking up request while waiting for initialization.
	// If faucet nodes crashes, and you restart with a clean db, all previous faucet req blks will be enqueued
	// and addresses will be funded again. Therefore, do not process any faucet request blocks until we are in
	// sync and initialized.
	if !initDone.Load() {
		return
	}
	deps.Tangle.Storage.Block(blockID).Consume(func(block *tangleold.Block) {
		if !faucet.IsFaucetReq(block) {
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
	})
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
