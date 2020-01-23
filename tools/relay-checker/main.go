package main

import (
	"fmt"
	"time"

	client "github.com/iotaledger/goshimmer/client"
	"github.com/iotaledger/goshimmer/packages/errors"
	"github.com/iotaledger/iota.go/trinary"
)

func testBroadcastData(api *client.GoShimmerAPI) (trinary.Hash, error) {
	txnHash, err := api.BroadcastData(txnAddr, txnData)
	if err != nil {
		return "", errors.Wrapf(err, "Broadcast failed")
	}
	return txnHash, nil
}

func testTargetGetTransactions(api *client.GoShimmerAPI, txnHash trinary.Hash) error {
	// query target node for broadcasted data
	_, err := api.GetTransactionObjectsByHash([]trinary.Hash{txnHash})
	if err != nil {
		return errors.Wrapf(err, "Query target failed")
	}
	return nil
}

func testNodesGetTransactions(txnHash trinary.Hash) error {
	// query nodes node for broadcasted data
	for _, n := range nodes {
		nodesApi := client.NewGoShimmerAPI(n)
		_, err := nodesApi.GetTransactionObjectsByHash([]trinary.Hash{txnHash})
		if err != nil {
			return errors.Wrapf(err, "Query %s failed", n)
		}
		fmt.Printf("txn found in %s\n", n)
	}
	return nil
}

func main() {
	LoadConfig()
	SetConfig()

	api := client.NewGoShimmerAPI(target)
	for i := 0; i < maxQuery; i++ {
		txnHash, err := testBroadcastData(api)
		if err != nil {
			fmt.Printf("%s\n", err)
			break
		}
		fmt.Printf("txnHash: %s\n", txnHash)

		// cooldown time
		time.Sleep(time.Duration(cooldown) * time.Second)

		// query target node
		err = testTargetGetTransactions(api, txnHash)
		if err != nil {
			fmt.Printf("%s\n", err)
			break
		}

		// query nodes node
		err = testNodesGetTransactions(txnHash)
		if err != nil {
			fmt.Printf("%s\n", err)
			break
		}
	}
}
