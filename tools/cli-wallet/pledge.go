package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/iotaledger/goshimmer/client/wallet"
)

func execAllowedPledgeNodeIDsCommand(command *flag.FlagSet, cliWallet *wallet.Wallet) {
	err := command.Parse(os.Args[2:])
	if err != nil {
		printUsage(nil, err.Error())
	}

	fmt.Println("Querying node about allowed pledge nodeIDs ... ")

	// query about pledgeNodeIDs
	res, err := cliWallet.AllowedPledgeNodeIDs()
	if err != nil {
		panic(err)
	}

	for key, value := range res {
		fmt.Printf("\nAllowed %s full nodeIDs: \n", key.String())
		for _, nodeID := range value {
			fmt.Printf("\n   %s", nodeID)
		}
		fmt.Println()
	}
	fmt.Println()
	fmt.Println("Querying node about allowed pledge nodeIDs ... DONE")
}
