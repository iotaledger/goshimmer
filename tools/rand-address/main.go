package main

import (
	"fmt"

	"github.com/iotaledger/goshimmer/client/wallet"
)

func main() {
	fmt.Println(wallet.NewSeed().Address(0))
}
