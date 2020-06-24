package faucet

import (
	"sync"

	faucet "github.com/iotaledger/goshimmer/dapps/faucet/packages"
	"github.com/iotaledger/goshimmer/dapps/faucet/packages/payload"
	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/message"
	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/tangle"
	"github.com/iotaledger/goshimmer/plugins/config"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
	"github.com/mr-tron/base58"

	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
	flag "github.com/spf13/pflag"
)

const (
	name = "Faucet" // name of the plugin

	// CfgFaucetSeed defines the base58 encoded seed the faucet uses.
	CfgFaucetSeed = "faucet.seed"
	// CfgFaucetTokensPerRequest defines the amount of tokens the faucet should send for each request.
	CfgFaucetTokensPerRequest = "faucet.tokensPerRequest"
)

func init() {
	flag.String(CfgFaucetSeed, "", "the base58 encoded seed of the faucet, must be defined if this dApp is enabled")
	flag.Int(CfgFaucetTokensPerRequest, 1337, "the amount of tokens the faucet should send for each request")
}

var (
	// App is the "plugin" instance of the faucet application.
	App        = node.NewPlugin(name, node.Disabled, configure, run)
	_faucet    *faucet.Faucet
	faucetOnce sync.Once
	log        *logger.Logger
)

// Faucet gets the faucet instance the faucet dApp has initialized.
func Faucet() *faucet.Faucet {
	faucetOnce.Do(func() {
		base58Seed := config.Node.GetString(CfgFaucetSeed)
		if len(base58Seed) == 0 {
			log.Fatal("a seed must be defined when enabling the faucet dApp")
		}
		seedBytes, err := base58.Decode(base58Seed)
		if err != nil {
			log.Fatalf("configured seed for the faucet is invalid: %s", err)
		}
		tokensPerRequest := config.Node.GetInt64(CfgFaucetTokensPerRequest)
		if tokensPerRequest <= 0 {
			log.Fatalf("the amount of tokens to fulfill per request must be above zero")
		}
		_faucet = faucet.New(seedBytes, tokensPerRequest)
	})
	return _faucet
}

func configure(*node.Plugin) {
	log = logger.NewLogger(name)
	Faucet()
	configureEvents()
}

func configureEvents() {
	messagelayer.Tangle.Events.MessageSolid.Attach(events.NewClosure(func(cachedTransaction *message.CachedMessage, cachedTransactionMetadata *tangle.CachedMessageMetadata) {
		defer cachedTransaction.Release()
		cachedTransactionMetadata.Release()

		msg := cachedTransaction.Unwrap()
		if msg == nil {
			log.Errorf("Fail to unwrap cachedTransaction")
			return
		}

		if !faucetpayload.IsFaucetReq(msg) {
			return
		}
		log.Info("got a faucet request")

		// send funds
		addr := msg.Payload().(*faucetpayload.Payload).Address()
		txID, err := Faucet().SendFunds(msg)
		if err != nil {
			log.Errorf("failed to send funds: %s", err)
			return
		}
		log.Infof("sent funds to %s via txID: %s", addr, txID)
	}))
}

func run(*node.Plugin) {}
