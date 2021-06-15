package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/iotaledger/goshimmer/client/wallet"
)

func execServerStatusCommand(command *flag.FlagSet, cliWallet *wallet.Wallet) {
	err := command.Parse(os.Args[2:])
	if err != nil {
		printUsage(nil, err.Error())
	}

	fmt.Println()

	// request funds
	status, err := cliWallet.ServerStatus()
	if err != nil {
		panic(err)
	}
	fmt.Println("Server ID: ", status.ID)
	fmt.Println("Server Synced: ", status.Synced)
	fmt.Println("Server Version: ", status.Version)
	fmt.Println("Delegation Address: ", status.DelegationAddress)
}
