package main

import (
	"fmt"

	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/wallet"
)

func main() {
	fmt.Println(wallet.New().Seed().Address(0))
}
