package spamming

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/iotaledger/hive.go/core/logger"
	"github.com/iotaledger/hive.go/core/node"
	"github.com/labstack/echo"
	"go.uber.org/dig"

	"github.com/iotaledger/goshimmer/client"
	walletseed "github.com/iotaledger/goshimmer/client/wallet/packages/seed"
	"github.com/iotaledger/goshimmer/packages/app/jsonmodels"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/vm/devnetvm"
)

// PluginName is the name of the web API data endpoint plugin.
const PluginName = "WebAPISpammingEndpoint"

type dependencies struct {
	dig.In

	Server *echo.Echo
}

var (
	// Plugin is the plugin instance of the web API data endpoint plugin.
	Plugin *node.Plugin
	deps   = new(dependencies)
	log    *logger.Logger
)

const (
	awaitConfirmation = 3 * time.Second
	maxAttempt        = 5
	spentAmount       = 1000000
)

func init() {
	Plugin = node.NewPlugin(PluginName, deps, node.Disabled, configure)
}

func configure(_ *node.Plugin) {
	log = logger.NewLogger(PluginName)
	deps.Server.GET("spam/doubleSpend", doubleSpend)
}

func doubleSpend(c echo.Context) error {
	if len(Parameters.Endpoints) < 2 {
		return c.JSON(http.StatusInternalServerError, jsonmodels.ErrorResponse{
			Error: "Not enough endpoints to double spend",
		})
	}

	node1, node2 := Parameters.Endpoints[0], Parameters.Endpoints[1]
	if node1 == node2 {
		return c.JSON(http.StatusInternalServerError, jsonmodels.ErrorResponse{
			Error: "Please use 2 different nodes to issue a double-spend",
		})
	}

	clients := make([]*client.GoShimmerAPI, 2)
	fmt.Println(node1, node2)
	clients[0] = client.NewGoShimmerAPI(node1, client.WithHTTPClient(http.Client{Timeout: 60 * time.Second}))
	clients[1] = client.NewGoShimmerAPI(node2, client.WithHTTPClient(http.Client{Timeout: 60 * time.Second}))

	tmpSeed := walletseed.NewSeed()
	fundAddr := tmpSeed.Address(0)
	if _, err := clients[0].BroadcastFaucetRequest(fundAddr.Address().Base58(), -1); err != nil {
		return c.JSON(http.StatusInternalServerError, jsonmodels.ErrorResponse{Error: err.Error()})
	}

	var input string
	var confirmed bool
	// wait for the funds
	for i := 0; i < maxAttempt; i++ {
		time.Sleep(awaitConfirmation)
		resp, err := clients[0].PostAddressUnspentOutputs([]string{fundAddr.Address().Base58()})
		if err != nil {
			return c.JSON(http.StatusInternalServerError, jsonmodels.ErrorResponse{Error: err.Error()})
		}
		log.Info("Waiting for funds to be confirmed...")
		for _, v := range resp.UnspentOutputs {
			if len(v.Outputs) > 0 {
				input = v.Outputs[0].Output.OutputID.Base58
				confirmed = v.Outputs[0].ConfirmationState.IsAccepted()
				break
			}
		}
		if input != "" && confirmed {
			break
		}
	}

	if input == "" {
		return c.JSON(http.StatusInternalServerError, jsonmodels.ErrorResponse{Error: "could not find outputID"})
	}

	if !confirmed {
		return c.JSON(http.StatusInternalServerError, jsonmodels.ErrorResponse{Error: "outputID not confirmed"})
	}

	var inputID utxo.OutputID
	if err := inputID.FromBase58(input); err != nil {
		return c.JSON(http.StatusInternalServerError, jsonmodels.ErrorResponse{Error: "malformed confirmed outputID found"})
	}

	// issue transactions which spend the same output
	conflictingTxs := make([]*devnetvm.Transaction, 2)
	receiverSeeds := make([]*walletseed.Seed, 2)

	var wg sync.WaitGroup
	for i := range conflictingTxs {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()

			// create a new receiver wallet for the given conflict
			receiverSeeds[i] = walletseed.NewSeed()
			destAddr := receiverSeeds[i].Address(0)

			output := devnetvm.NewSigLockedSingleOutput(uint64(spentAmount), destAddr.Address())
			txEssence := devnetvm.NewTransactionEssence(0, time.Now(), identity.ID{}, identity.ID{}, devnetvm.NewInputs(devnetvm.NewUTXOInput(inputID)), devnetvm.NewOutputs(output))
			kp := *tmpSeed.KeyPair(0)
			sig := devnetvm.NewED25519Signature(kp.PublicKey, kp.PrivateKey.Sign(lo.PanicOnErr(txEssence.Bytes())))
			unlockBlock := devnetvm.NewSignatureUnlockBlock(sig)
			tx := devnetvm.NewTransaction(txEssence, devnetvm.UnlockBlocks{unlockBlock})
			conflictingTxs[i] = tx

			// issue the tx
			resp, err2 := clients[i].PostTransaction(lo.PanicOnErr(tx.Bytes()))
			if err2 != nil {
				panic(err2)
			}

			log.Infof("Double spend TX%d issued: %s", i, resp.TransactionID)
		}(i)
	}
	wg.Wait()

	return c.NoContent(http.StatusOK)
}
