package main

import (
	"fmt"

	"github.com/iotaledger/goshimmer/client"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
)

func createBlock() {

	url := "http://localhost:8080"

	emptyBlock := new(models.Block)
	b, err := emptyBlock.Bytes()
	if err != nil {
		panic(err)
	}
	clt := client.NewGoShimmerAPI(url)
	ID, err := clt.SendPayload(b)
	if err != nil {
		panic(err)
	}
	fmt.Println("Block posted, ID: " + ID)
}
