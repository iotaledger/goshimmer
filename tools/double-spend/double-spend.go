package main

import (
	"fmt"
	"net/http"
	"time"

	"github.com/iotaledger/goshimmer/client"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/address"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/address/signaturescheme"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/balance"
	valuepayload "github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/payload"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/transaction"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/wallet"
	"github.com/mr-tron/base58"
)

func main() {

	client := client.NewGoShimmerAPI("http://localhost:8080", http.Client{Timeout: 30 * time.Second})

	// genesis wallet
	genesisSeedBytes, err := base58.Decode("7R1itJx5hVuo9w9hjg5cwKFmek4HMSoBDgJZN8hKGxih")
	if err != nil {
		fmt.Println(err)
	}

	const genesisBalance = 1000000000
	genesisWallet := wallet.New(genesisSeedBytes)
	genesisAddr := genesisWallet.Seed().Address(0)
	genesisOutputID := transaction.NewOutputID(genesisAddr, transaction.GenesisID)

	// issue transactions which spend the same genesis output in all partitions
	conflictingTxs := make([]*transaction.Transaction, 2)
	conflictingTxIDs := make([]string, 2)
	receiverSeeds := make([]*wallet.Seed, 2)
	for i := range conflictingTxs {

		// create a new receiver wallet for the given conflict
		receiverSeeds[i] = wallet.NewSeed()
		destAddr := receiverSeeds[i].Address(0)

		tx := transaction.New(
			transaction.NewInputs(genesisOutputID),
			transaction.NewOutputs(map[address.Address][]*balance.Balance{
				destAddr: {
					{Value: genesisBalance, Color: balance.ColorIOTA},
				},
			}))
		tx = tx.Sign(signaturescheme.ED25519(*genesisWallet.Seed().KeyPair(0)))
		conflictingTxs[i] = tx

		valueObject := valuepayload.New(valuepayload.GenesisID, valuepayload.GenesisID, tx)

		// issue the value object
		txID, err := client.SendPayload(valueObject.Bytes())
		if err != nil {
			fmt.Println(err)
		}
		conflictingTxIDs[i] = txID
		fmt.Printf("issued conflict transaction %s\n", txID)
		//time.Sleep(7 * time.Second)
	}
}
