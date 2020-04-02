package main

import (
	"fmt"
	"time"

	client "github.com/iotaledger/goshimmer/client"
	"github.com/iotaledger/goshimmer/plugins/config"
	"github.com/iotaledger/goshimmer/plugins/logger"
)

func testBroadcastData(api *client.GoShimmerAPI) (string, error) {
	msgId, err := api.BroadcastData([]byte(msgData))
	if err != nil {
		return "", fmt.Errorf("broadcast failed: %w", err)
	}
	return msgId, nil
}

func testTargetGetMessagess(api *client.GoShimmerAPI, msgId string) error {
	// query target node for broadcasted data
	if _, err := api.FindMessageById([]string{msgId}); err != nil {
		return fmt.Errorf("querying the target node failed: %w", err)
	}
	return nil
}

func testNodesGetMessages(msgId string) error {
	// query nodes node for broadcasted data
	for _, n := range nodes {
		nodesApi := client.NewGoShimmerAPI(n)
		if _, err := nodesApi.FindMessageById([]string{msgId}); err != nil {
			return fmt.Errorf("querying node %s failed: %w", n, err)
		}
		fmt.Printf("msg found in node %s\n", n)
	}
	return nil
}

func main() {
	config.Init()
	logger.Init()

	InitConfig()

	api := client.NewGoShimmerAPI(target)
	for i := 0; i < repeat; i++ {
		msgId, err := testBroadcastData(api)
		if err != nil {
			fmt.Printf("%s\n", err)
			break
		}
		fmt.Printf("msgId: %s\n", msgId)

		// cooldown time
		time.Sleep(time.Duration(cooldownTime) * time.Second)

		// query target node
		err = testTargetGetMessagess(api, msgId)
		if err != nil {
			fmt.Printf("%s\n", err)
			break
		}

		// query test nodes
		err = testNodesGetMessages(msgId)
		if err != nil {
			fmt.Printf("%s\n", err)
			break
		}
	}
}
