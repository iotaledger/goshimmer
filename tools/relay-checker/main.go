package main

import (
	"fmt"
	"time"

	client "github.com/iotaledger/goshimmer/client"
	"github.com/iotaledger/goshimmer/plugins/config"
	"github.com/iotaledger/goshimmer/plugins/logger"

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
	if _, err := api.GetTransactionObjectsByHash([]trinary.Hash{txnHash}); err != nil {
		return fmt.Errorf("querying the target node failed: %w", err)
	}
	return nil
}

func testNodesGetTransactions(txnHash trinary.Hash) error {
	// query nodes node for broadcasted data
	for _, n := range nodes {
		nodesApi := client.NewGoShimmerAPI(n)
		if _, err := nodesApi.GetTransactionObjectsByHash([]trinary.Hash{txnHash}); err != nil {
			return fmt.Errorf("querying node %s failed: %w", n, err)
		}
		fmt.Printf("txn found in node %s\n", n)
	}
	return nil
}

func main() {
	config.Init()
	logger.Init()

	InitConfig()

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
