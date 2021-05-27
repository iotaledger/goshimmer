package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/iotaledger/goshimmer/client/wallet"
	"github.com/iotaledger/goshimmer/client/wallet/packages/consolidateoptions"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
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

	var txs []*ledgerstate.Transaction
	fmt.Println("Consolidating funds... [this might take a while]")
	txs, err = cliWallet.ConsolidateFunds(
		consolidateoptions.AccessManaPledgeID(*accessManaPledgeIDPtr),
		consolidateoptions.ConsensusManaPledgeID(*consensusManaPledgeIDPtr),
	)
	if err != nil {
		printUsage(command, err.Error())
	}

	fmt.Println()
	fmt.Printf("\nConsolidated funds into %d output(s) via:\n", len(txs))
	for _, tx := range txs {
		fmt.Printf("\n\tTransaction %s\n", tx.ID().Base58())
	}
	fmt.Println("Consolidating funds... [DONE]")
}
