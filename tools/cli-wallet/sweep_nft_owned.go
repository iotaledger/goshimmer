package main

import (
	"flag"
	"fmt"
	"github.com/iotaledger/goshimmer/client/wallet"
	"github.com/iotaledger/goshimmer/client/wallet/packages/sweepnftownedfunds_options"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"os"
)

func execSweepNFTOwnedCommand(command *flag.FlagSet, cliWallet *wallet.Wallet) {
	command.Usage = func() {
		printUsage(command)
	}

	helpPtr := command.Bool("help", false, "show this help screen")
	nftIDPtr := command.String("id", "", "unique identifier of the nft that should be checked for outputs with funds")
	toAddressPtr := command.String("to", "", "optional address where to sweep")
	accessManaPledgeIDPtr := command.String("access-mana-id", "", "node ID to pledge access mana to")
	consensusManaPledgeIDPtr := command.String("consensus-mana-id", "", "node ID to pledge consensus mana to")

	err := command.Parse(os.Args[2:])
	if err != nil {
		panic(err)
	}

	if *helpPtr {
		printUsage(command)
	}

	if *nftIDPtr == "" {
		printUsage(command, "an nft (alias) ID must be given for sweeping")
	}

	aliasID, err := ledgerstate.AliasAddressFromBase58EncodedString(*nftIDPtr)
	if err != nil {
		printUsage(command, err.Error())
	}
	options := []sweepnftownedfunds_options.SweepNFTOwnedFundsOption{
		sweepnftownedfunds_options.Alias(aliasID.Base58()),
		sweepnftownedfunds_options.AccessManaPledgeID(*accessManaPledgeIDPtr),
		sweepnftownedfunds_options.ConsensusManaPledgeID(*consensusManaPledgeIDPtr),
	}

	if *toAddressPtr != "" {
		options = append(options, sweepnftownedfunds_options.ToAddress(*toAddressPtr))
	}

	_, err = cliWallet.SweepNFTOwnedFunds(options...)
	if err != nil {
		printUsage(command, err.Error())
	}

	fmt.Println()
	fmt.Println("Sweeping NFT owned funds... [DONE]")

}
