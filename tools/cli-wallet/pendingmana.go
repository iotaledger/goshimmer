package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/iotaledger/goshimmer/client/wallet"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/vm/devnetvm"
)

func execPendingMana(command *flag.FlagSet, cliWallet *wallet.Wallet) {
	err := command.Parse(os.Args[2:])
	if err != nil {
		printUsage(nil, err.Error())
	}

	fmt.Println("\nPending Mana for all unspent outputs")
	fmt.Println("-----------------------------------")
	unspentOutputs := cliWallet.UnspentOutputs()
	for addr, v := range unspentOutputs {
		fmt.Printf("Address: %s\n", addr.Base58())
		for ID, output := range v {
			var total float64
			output.Object.Balances().ForEach(func(color devnetvm.Color, balance uint64) bool {
				total += float64(balance)
				return true
			})
			fmt.Printf("\tOutputID: %s - Pending Mana: %f\n", ID.Base58(), total)
		}
	}
	fmt.Println()
}
