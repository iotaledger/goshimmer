package main

import (
	"fmt"

	"github.com/iotaledger/goshimmer/client"
)

var c *client.GoShimmerAPI

func main() {
	node1APIURL := "http://node-03.feature.shimmer.iota.cafe:8080"

	c = client.NewGoShimmerAPI(node1APIURL)

	// nextBlock := "B1mhW9J6GhX4YvHBpvSiEvbyiFD9hTcCX1iAhqJFu18N:63"
	// nextBlock := "51P2WAa5PctjvA8kpLkfZjARUBUbMqQSUYJVUGobWAwi:62"
	nextBlock := "Avjy1VSu7iuJbxQN2o8wbW7dTomU88bydN3RZDfMq6HT:62"
	prevBlock := nextBlock

outer:
	for {
		if !invalid(nextBlock) {
			fmt.Println("Block is valid", nextBlock)

			block, err := c.GetBlock(prevBlock)
			if err != nil {
				panic(err)
			}

			for _, p := range block.StrongParents {
				fmt.Println("Strong parent", p)
				if invalid(p) {
					nextBlock = p
					continue outer
				}
			}
			return
		}

		block, err := c.GetBlock(nextBlock)
		if err != nil {
			panic(err)
		}

		fmt.Println("Block is invalid", nextBlock)
		prevBlock = nextBlock
		nextBlock = block.StrongParents[0]
	}
}

func invalid(blockID string) bool {
	blockMeta, err := c.GetBlockMetadata(blockID)
	if err != nil {
		panic(err)
	}

	return blockMeta.M.Invalid
}
