package main

import (
	"fmt"
	"strings"
	"time"

	"go.uber.org/dig"

	"github.com/iotaledger/goshimmer/client"
	"github.com/iotaledger/goshimmer/plugins/config"
	"github.com/iotaledger/goshimmer/plugins/logger"
)

func testBroadcastData(api *client.GoShimmerAPI) (string, error) {
	blkID, err := api.Data([]byte(blkData))
	if err != nil {
		return "", fmt.Errorf("broadcast failed: %w", err)
	}
	return blkID, nil
}

func testTargetGetBlocks(api *client.GoShimmerAPI, blkID string) error {
	// query target node for broadcasted data
	if _, err := api.GetBlock(blkID); err != nil {
		return fmt.Errorf("querying the target node failed: %w", err)
	}
	return nil
}

func testNodesGetBlocks(blkID string) error {
	// query nodes node for broadcasted data
	for _, n := range nodes {
		nodesAPI := client.NewGoShimmerAPI(n)
		if _, err := nodesAPI.GetBlock(blkID); err != nil {
			return fmt.Errorf("querying node %s failed: %w", n, err)
		}
		fmt.Printf("blk found in node %s\n", n)
	}
	return nil
}

func main() {
	container := dig.New()
	config.Init(container)
	logger.Init(container)

	initConfig()

	api := client.NewGoShimmerAPI(target)
	for i := 0; i < repeat; i++ {
		blkID, err := testBroadcastData(api)
		if err != nil {
			fmt.Printf("%s\n", strings.ReplaceAll(err.Error(), "\n", ""))
			break
		}
		fmt.Printf("blkID: %s\n", blkID)

		// cooldown time
		time.Sleep(cooldownTime)

		// query target node
		err = testTargetGetBlocks(api, blkID)
		if err != nil {
			fmt.Printf("%s\n", strings.ReplaceAll(err.Error(), "\n", ""))
			break
		}

		// query test nodes
		err = testNodesGetBlocks(blkID)
		if err != nil {
			fmt.Printf("%s\n", strings.ReplaceAll(err.Error(), "\n", ""))
			break
		}
	}
}
