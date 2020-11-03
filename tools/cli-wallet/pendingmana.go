package main

import (
	"flag"
	"fmt"
	"math"
	"os"
	"time"

	"github.com/iotaledger/goshimmer/client/wallet"
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
		fmt.Printf("Address: %s\n", addr.String())
		for _, output := range v {
			var total float64
			for _, balance := range output.Balances {
				total += float64(balance)
			}
			pendingMana := getBM2(total, time.Since(output.Metadata.Timestamp), status.ManaDecay)
			fmt.Printf("\tOutputID: %s - Pending Mana: %f\n", output.ID.String(), pendingMana)
		}
	}
	fmt.Println()
}

func getBM2(value float64, n time.Duration, decay float64) float64 {
	return value * (1 - math.Pow(math.E, -decay*(n.Seconds())))
}
