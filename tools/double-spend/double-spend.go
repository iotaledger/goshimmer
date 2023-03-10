package main

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/iotaledger/goshimmer/client"
	walletseed "github.com/iotaledger/goshimmer/client/wallet/packages/seed"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/vm/devnetvm"
	"github.com/iotaledger/hive.go/crypto/identity"
	"github.com/iotaledger/hive.go/lo"
)

func main() {
	clients := make([]*client.GoShimmerAPI, 2)

	node1APIURL := "http://127.0.0.1:8080"
	node2APIURL := "http://127.0.0.1:8090"

	if node1APIURL == node2APIURL {
		fmt.Println("Please use 2 different nodes to issue a double-spend")
		return
	}

	clients[0] = client.NewGoShimmerAPI(node1APIURL, client.WithHTTPClient(http.Client{Timeout: 60 * time.Second}))
	clients[1] = client.NewGoShimmerAPI(node2APIURL, client.WithHTTPClient(http.Client{Timeout: 60 * time.Second}))

	mySeed := walletseed.NewSeed()
	myAddr := mySeed.Address(0)

	if _, err := clients[0].BroadcastFaucetRequest(myAddr.Address().Base58(), -1); err != nil {
		fmt.Println(strings.ReplaceAll(err.Error(), "\n", ""))
		return
	}

	var myOutputID string
	var confirmed bool
	// wait for the funds
	for i := 0; i < 10; i++ {
		time.Sleep(5 * time.Second)
		resp, err := clients[0].PostAddressUnspentOutputs([]string{myAddr.Address().Base58()})
		if err != nil {
			fmt.Println(strings.ReplaceAll(err.Error(), "\n", ""))
			return
		}
		fmt.Println("Waiting for funds to be confirmed...")
		for _, v := range resp.UnspentOutputs {
			if len(v.Outputs) > 0 {
				myOutputID = v.Outputs[0].Output.OutputID.Base58
				confirmed = v.Outputs[0].ConfirmationState.IsAccepted()
				break
			}
		}
		if myOutputID != "" && confirmed {
			break
		}
	}

	if myOutputID == "" {
		fmt.Println("Could not find OutputID")
		return
	}

	if !confirmed {
		fmt.Println("OutputID not confirmed")
		return
	}

	var out utxo.OutputID
	if err := out.FromBase58(myOutputID); err != nil {
		fmt.Println("malformed OutputID")
		return
	}

	// issue transactions which spend the same output
	conflictingTxs := make([]*devnetvm.Transaction, 2)
	conflictingBlkIDs := make([]string, 2)
	receiverSeeds := make([]*walletseed.Seed, 2)

	var wg sync.WaitGroup
	for i := range conflictingTxs {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()

			// create a new receiver wallet for the given conflict
			receiverSeeds[i] = walletseed.NewSeed()
			destAddr := receiverSeeds[i].Address(0)

			output := devnetvm.NewSigLockedColoredOutput(devnetvm.NewColoredBalances(map[devnetvm.Color]uint64{
				devnetvm.ColorIOTA: uint64(1000000),
			}), destAddr.Address())
			txEssence := devnetvm.NewTransactionEssence(0, time.Now(), identity.ID{}, identity.ID{}, devnetvm.NewInputs(devnetvm.NewUTXOInput(out)), devnetvm.NewOutputs(output))
			kp := *mySeed.KeyPair(0)
			sig := devnetvm.NewED25519Signature(kp.PublicKey, kp.PrivateKey.Sign(lo.PanicOnErr(txEssence.Bytes())))
			unlockBlock := devnetvm.NewSignatureUnlockBlock(sig)
			tx := devnetvm.NewTransaction(txEssence, devnetvm.UnlockBlocks{unlockBlock})
			conflictingTxs[i] = tx

			// issue the tx
			resp, err2 := clients[i].PostTransaction(lo.PanicOnErr(tx.Bytes()))
			if err2 != nil {
				panic(err2)
			}
			fmt.Println(resp.TransactionID)

			fmt.Printf("issued conflict transaction %s\n", conflictingBlkIDs[i])
		}(i)
	}
	wg.Wait()
}
