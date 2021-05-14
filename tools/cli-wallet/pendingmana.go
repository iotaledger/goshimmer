package main

import (
	"flag"
	"fmt"
	"math"
	"os"
	"time"

	"github.com/iotaledger/goshimmer/client/wallet"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
)

func execPendingMana(command *flag.FlagSet, cliWallet *wallet.Wallet) {
	err := command.Parse(os.Args[2:])
	if err != nil {
		printUsage(nil, err.Error())
	}

	status, err := cliWallet.ServerStatus()
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
			output.Object.Balances().ForEach(func(color ledgerstate.Color, balance uint64) bool {
				total += float64(balance)
				return true
			})
			pendingMana := getPendingMana(total, time.Since(output.Metadata.Timestamp), status.ManaDecay)
			fmt.Printf("\tOutputID: %s - Pending Mana: %f\n", ID.Base58(), pendingMana)
		}
	}
	fmt.Println()
}

func getPendingMana(value float64, n time.Duration, decay float64) float64 {
	return value * (1 - math.Pow(math.E, -decay*(n.Seconds())))
}
