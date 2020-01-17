package main

import (
	"fmt"
	"time"

	client "github.com/iotaledger/goshimmer/client"
	"github.com/iotaledger/iota.go/trinary"
)

func testBroadcastData(api *client.GoShimmerAPI) (trinary.Hash, bool) {
	txnHash, err := api.BroadcastData(txnAddr, txnData)
	if err != nil {
		fmt.Printf("Relay Check: broadcast data failed.\n %s\n", err)
		return "", true
	}
	fmt.Printf("Relay Check: txnHash: %s\n", txnHash)
	return txnHash, false
}

func testTargetGetTransactions(api *client.GoShimmerAPI, txnHash trinary.Hash) bool {
	// query target node for broadcasted data
	_, err := api.GetTransactions([]trinary.Hash{txnHash})
	if err != nil {
		fmt.Printf("Relay-Check: get transaction from local failed.\n")
		return true
	}
	return false
}

func testNeighborGetTransactions(txnHash trinary.Hash) bool {
	// query neighbor node for broadcasted data
	neighborApi := client.NewGoShimmerAPI(neighbor)
	_, err := neighborApi.GetTransactions([]trinary.Hash{txnHash})
	if err != nil {
		fmt.Printf("Relay-Check: get transaction failed.\n")
		return true
	}
	return false
}

func main() {
	LoadConfig()
	SetConfig()

	api := client.NewGoShimmerAPI(target)
	for i := 0; i < maxQuery; i++ {
		txnHash, err := testBroadcastData(api)
		if err {
			break
		}

		// cooldown time
		time.Sleep(time.Duration(cooldown) * time.Second)

		// query target node
		if err = testTargetGetTransactions(api, txnHash); err {
			break
		}
		// query neighbor node
		if err = testNeighborGetTransactions(txnHash); err {
			break
		}
	}
}
