package main

import (
	"fmt"
	"time"

	client "github.com/iotaledger/goshimmer/client"
	"github.com/iotaledger/iota.go/trinary"
)

func testBroadcastData(api *client.GoShimmerAPI) (trinary.Hash, error) {
	txnHash, err := api.BroadcastData(txnAddr, txnData)
	if err != nil {
		return "", fmt.Errorf("broadcast failed: %w", err)
	}
	return txnHash, nil
}

func testTargetGetTransactions(api *client.GoShimmerAPI, txnHash trinary.Hash) error {
	// query target node for broadcasted data
	res, err := api.GetTransactionObjectsByHash([]trinary.Hash{txnHash})
	if err != nil {
		return fmt.Errorf("querying the target node failed: %w", err)
	}
	if len(res) == 0 || res[0].Hash != txnHash {
		return fmt.Errorf("txn %s not found on target", txnHash)
	}
	return nil
}

func testNodesGetTransactions(txnHash trinary.Hash) error {
	// query nodes node for broadcasted data
	for _, n := range nodes {
		nodesApi := client.NewGoShimmerAPI(n)
		res, err := nodesApi.GetTransactionObjectsByHash([]trinary.Hash{txnHash})
		if err != nil {
			return fmt.Errorf("querying node %s failed: %w", n, err)
		}
		if len(res) == 0 || res[0].Hash != txnHash {
			fmt.Printf("WARNING: txn not found on node %s\n", n)
		} else {
			fmt.Printf("txn found on node %s\n", n)
		}
	}
	return nil
}

func main() {
	LoadConfig()
	SetConfig()

	api := client.NewGoShimmerAPI(target)
	for i := 0; i < repeat; i++ {
		txnHash, err := testBroadcastData(api)
		if err != nil {
			fmt.Printf("%s\n", err)
			break
		}
		fmt.Printf("txnHash: %s\n", txnHash)

		// cooldown time
		time.Sleep(time.Duration(cooldownTime) * time.Second)

		// query target node
		err = testTargetGetTransactions(api, txnHash)
		if err != nil {
			fmt.Printf("%s\n", err)
			break
		}

		// query test nodes
		err = testNodesGetTransactions(txnHash)
		if err != nil {
			fmt.Printf("%s\n", err)
			break
		}
	}
}
