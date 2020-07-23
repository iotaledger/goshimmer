package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/iotaledger/goshimmer/client/wallet"
)

func execRequestFundsCommand(command *flag.FlagSet, cliWallet *wallet.Wallet) {
	err := command.Parse(os.Args[2:])
	if err != nil {
		printUsage(nil, err.Error())
	}

	fmt.Println()
	fmt.Println("Requesting funds from faucet ... [PERFORMING POW]          (this can take a while)")

	// request funds
	err = cliWallet.RequestFaucetFunds()
	if err != nil {
		panic(err)
	}
	fmt.Println("Requesting funds from faucet ... [DONE]")
}
