package main

import (
	"flag"
	"fmt"
	"github.com/iotaledger/goshimmer/client/wallet"
	"github.com/iotaledger/goshimmer/client/wallet/packages/consolidatefunds_options"
	"os"
)

func execConsolidateFundsCommand(command *flag.FlagSet, cliWallet *wallet.Wallet) {
	command.Usage = func() {
		printUsage(command)
	}

	helpPtr := command.Bool("help", false, "show this help screen")
	accessManaPledgeIDPtr := command.String("access-mana-id", "", "node ID to pledge access mana to")
	consensusManaPledgeIDPtr := command.String("consensus-mana-id", "", "node ID to pledge consensus mana to")

	err := command.Parse(os.Args[2:])
	if err != nil {
		panic(err)
	}

	if *helpPtr {
		printUsage(command)
	}

	_, err = cliWallet.ConsolidateFunds(
		consolidatefunds_options.AccessManaPledgeID(*accessManaPledgeIDPtr),
		consolidatefunds_options.ConsensusManaPledgeID(*consensusManaPledgeIDPtr),
	)
	if err != nil {
		printUsage(command, err.Error())
	}

	fmt.Println()
	fmt.Println("Consolidating funds... [DONE]")

}
