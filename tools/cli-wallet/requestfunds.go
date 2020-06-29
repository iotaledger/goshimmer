package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/wallet"
)

func execRequestFundsCommand(command *flag.FlagSet, cliWallet *wallet.Wallet) {
	err := command.Parse(os.Args[2:])
	if err != nil {
		printUsage(err.Error())
	}

	// request funds
	err = cliWallet.RequestFaucetFunds()
	if err != nil {
		panic(err)
	}
	fmt.Println("Requesting funds from faucet ... [DONE]")
}
