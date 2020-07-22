package main

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/iotaledger/goshimmer/client"
	"github.com/iotaledger/goshimmer/client/wallet"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/address"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/address/signaturescheme"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/balance"
	valuepayload "github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/payload"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/transaction"
	libwallet "github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/wallet"
)

func main() {
	clients := make([]*client.GoShimmerAPI, 2)

	node1APIURL := "http://127.0.0.1:8080"
	node2APIURL := "http://127.0.0.1:8090"

	if node1APIURL == node2APIURL {
		fmt.Println("Please use 2 different nodes to issue a double-spend")
		return
	}

	clients[0] = client.NewGoShimmerAPI(node1APIURL, http.Client{Timeout: 60 * time.Second})
	clients[1] = client.NewGoShimmerAPI(node2APIURL, http.Client{Timeout: 60 * time.Second})

	myWallet := wallet.New()
	myAddr := myWallet.Seed().Address(0)

	if _, err := clients[0].SendFaucetRequest(myAddr.String()); err != nil {
		fmt.Println(err)
		return
	}

	var myOutputID string
	var confirmed bool
	// wait for the funds
	for i := 0; i < 10; i++ {
		time.Sleep(5 * time.Second)
		resp, err := clients[0].GetUnspentOutputs([]string{myAddr.String()})
		if err != nil {
			fmt.Println(err)
			return
		}
		fmt.Println("Waiting for funds to be confirmed...")
		for _, v := range resp.UnspentOutputs {
			if len(v.OutputIDs) > 0 {
				myOutputID = v.OutputIDs[0].ID
				confirmed = v.OutputIDs[0].InclusionState.Confirmed
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

	out, err := transaction.OutputIDFromBase58(myOutputID)
	if err != nil {
		fmt.Println("malformed OutputID")
		return
	}

	// issue transactions which spend the same output
	conflictingTxs := make([]*transaction.Transaction, 2)
	conflictingMsgIDs := make([]string, 2)
	receiverSeeds := make([]*libwallet.Seed, 2)

	var wg sync.WaitGroup
	for i := range conflictingTxs {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()

			fmt.Println(i)

			// create a new receiver wallet for the given conflict
			receiverSeeds[i] = libwallet.NewSeed()
			destAddr := receiverSeeds[i].Address(0)

			tx := transaction.New(
				transaction.NewInputs(out),
				transaction.NewOutputs(map[address.Address][]*balance.Balance{
					destAddr.Address: {
						{Value: 1337, Color: balance.ColorIOTA},
					},
				}))
			tx = tx.Sign(signaturescheme.ED25519(*myWallet.Seed().KeyPair(0)))
			conflictingTxs[i] = tx

			valueObject := valuepayload.New(valuepayload.GenesisID, valuepayload.GenesisID, tx)

			// issue the tx
			conflictingMsgIDs[i], err = clients[i].SendPayload(valueObject.Bytes())
			if err != nil {
				fmt.Println(err)
				return
			}

			fmt.Printf("issued conflict transaction %s\n", conflictingMsgIDs[i])
		}(i)
	}
	wg.Wait()
}
