package main

import (
	"fmt"
	"time"

	"go.uber.org/dig"

	client "github.com/iotaledger/goshimmer/client"
	"github.com/iotaledger/goshimmer/plugins/config"
	"github.com/iotaledger/goshimmer/plugins/logger"
)

func testBroadcastData(api *client.GoShimmerAPI) (string, error) {
	msgID, err := api.Data([]byte(msgData))
	if err != nil {
		return "", fmt.Errorf("broadcast failed: %w", err)
	}
	return msgID, nil
}

func testTargetGetMessages(api *client.GoShimmerAPI, msgID string) error {
	// query target node for broadcasted data
	if _, err := api.GetMessage(msgID); err != nil {
		return fmt.Errorf("querying the target node failed: %w", err)
	}
	return nil
}

func testNodesGetMessages(msgID string) error {
	// query nodes node for broadcasted data
	for _, n := range nodes {
		nodesAPI := client.NewGoShimmerAPI(n)
		if _, err := nodesAPI.GetMessage(msgID); err != nil {
			return fmt.Errorf("querying node %s failed: %w", n, err)
		}
		fmt.Printf("msg found in node %s\n", n)
	}
	return nil
}

func main() {
	container := dig.New()
	config.Init(container)
	logger.Init()

	initConfig()

	api := client.NewGoShimmerAPI(target)
	for i := 0; i < repeat; i++ {
		msgID, err := testBroadcastData(api)
		if err != nil {
			fmt.Printf("%s\n", err)
			break
		}
		fmt.Printf("msgID: %s\n", msgID)

		// cooldown time
		time.Sleep(cooldownTime)

		// query target node
		err = testTargetGetMessages(api, msgID)
		if err != nil {
			fmt.Printf("%s\n", err)
			break
		}

		// query test nodes
		err = testNodesGetMessages(msgID)
		if err != nil {
			fmt.Printf("%s\n", err)
			break
		}
	}
}
