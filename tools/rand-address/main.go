package main

import (
	"fmt"

	walletseed "github.com/iotaledger/goshimmer/client/wallet/packages/seed"
)

func main() {
	fmt.Println(walletseed.NewSeed().Address(0).Address().Base58())
}
