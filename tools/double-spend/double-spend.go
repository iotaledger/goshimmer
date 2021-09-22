package main

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/iotaledger/goshimmer/packages/consensus/gof"

	"github.com/iotaledger/hive.go/identity"

	"github.com/iotaledger/goshimmer/client"
	walletseed "github.com/iotaledger/goshimmer/client/wallet/packages/seed"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
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

	if _, err := clients[0].SendFaucetRequest(myAddr.Address().Base58(), -1); err != nil {
		fmt.Println(err)
		return
	}

	var myOutputID string
	var confirmed bool
	// wait for the funds
	for i := 0; i < 10; i++ {
		time.Sleep(5 * time.Second)
		resp, err := clients[0].PostAddressUnspentOutputs([]string{myAddr.Address().Base58()})
		if err != nil {
			fmt.Println(err)
			return
		}
		fmt.Println("Waiting for funds to be confirmed...")
		for _, v := range resp.UnspentOutputs {
			if len(v.Outputs) > 0 {
				myOutputID = v.Outputs[0].Output.OutputID.Base58
				confirmed = v.Outputs[0].GradeOfFinality == gof.High
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

	out, err := ledgerstate.OutputIDFromBase58(myOutputID)
	if err != nil {
		fmt.Println("malformed OutputID")
		return
	}

	// issue transactions which spend the same output
	conflictingTxs := make([]*ledgerstate.Transaction, 2)
	conflictingMsgIDs := make([]string, 2)
	receiverSeeds := make([]*walletseed.Seed, 2)

	var wg sync.WaitGroup
	for i := range conflictingTxs {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()

			// create a new receiver wallet for the given conflict
			receiverSeeds[i] = walletseed.NewSeed()
			destAddr := receiverSeeds[i].Address(0)

			output := ledgerstate.NewSigLockedColoredOutput(ledgerstate.NewColoredBalances(map[ledgerstate.Color]uint64{
				ledgerstate.ColorIOTA: uint64(1000000),
			}), destAddr.Address())
			txEssence := ledgerstate.NewTransactionEssence(0, time.Now(), identity.ID{}, identity.ID{}, ledgerstate.NewInputs(ledgerstate.NewUTXOInput(out)), ledgerstate.NewOutputs(output))
			kp := *mySeed.KeyPair(0)
			sig := ledgerstate.NewED25519Signature(kp.PublicKey, kp.PrivateKey.Sign(txEssence.Bytes()))
			unlockBlock := ledgerstate.NewSignatureUnlockBlock(sig)
			tx := ledgerstate.NewTransaction(txEssence, ledgerstate.UnlockBlocks{unlockBlock})
			conflictingTxs[i] = tx

			// issue the tx
			resp, err2 := clients[i].PostTransaction(tx.Bytes())
			if err2 != nil {
				fmt.Println(err)
				return
			}
			fmt.Println(resp.TransactionID)

			fmt.Printf("issued conflict transaction %s\n", conflictingMsgIDs[i])
		}(i)
	}
	wg.Wait()
}
