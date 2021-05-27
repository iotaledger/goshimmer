package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/iotaledger/goshimmer/client/wallet"
	"github.com/iotaledger/goshimmer/client/wallet/packages/claimconditionaloptions"
)

func execClaimConditionalCommand(command *flag.FlagSet, cliWallet *wallet.Wallet) {
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

	fmt.Println("Claiming conditionally owned funds... [this might take a while]")
	_, err = cliWallet.ClaimConditionalFunds(
		claimconditionaloptions.AccessManaPledgeID(*accessManaPledgeIDPtr),
		claimconditionaloptions.ConsensusManaPledgeID(*consensusManaPledgeIDPtr),
	)
	if err != nil {
		printUsage(command, err.Error())
	}

	fmt.Println()
	fmt.Println("Claiming conditionally owned funds... [DONE]")
}
