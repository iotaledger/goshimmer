package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/iotaledger/goshimmer/client/wallet"
	"github.com/iotaledger/goshimmer/client/wallet/packages/reclaimoptions"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
)

func execReclaimDelegatedFundsCommand(command *flag.FlagSet, cliWallet *wallet.Wallet) {
	command.Usage = func() {
		printUsage(command)
	}

	helpPtr := command.Bool("help", false, "show this help screen")
	delegationIDPtr := command.String("id", "", "delegation ID that should be reclaimed")
	toAddressPtr := command.String("to-addr", "", "optional address where to send reclaimed funds, wallet receive address by default")
	accessManaPledgeIDPtr := command.String("access-mana-id", "", "node ID to pledge access mana to")
	consensusManaPledgeIDPtr := command.String("consensus-mana-id", "", "node ID to pledge consensus mana to")

	err := command.Parse(os.Args[2:])
	if err != nil {
		panic(err)
	}

	if *helpPtr {
		printUsage(command)
	}
	if *delegationIDPtr == "" {
		printUsage(command, "delegation ID must be given")
	}

	var toAddress ledgerstate.Address
	if *toAddressPtr != "" {
		toAddress, err = ledgerstate.AddressFromBase58EncodedString(*toAddressPtr)
		if err != nil {
			printUsage(command, fmt.Sprintf("wrong optional toAddress provided: %s", err.Error()))
		}
	}

	delegationID, err := ledgerstate.AliasAddressFromBase58EncodedString(*delegationIDPtr)
	if err != nil {
		printUsage(command, fmt.Sprintf("%s is not a valid IOTA alias address: %s", *delegationIDPtr, err.Error()))
	}

	options := []reclaimoptions.ReclaimFundsOption{
		reclaimoptions.Alias(delegationID.Base58()),
		reclaimoptions.AccessManaPledgeID(*accessManaPledgeIDPtr),
		reclaimoptions.ConsensusManaPledgeID(*consensusManaPledgeIDPtr),
	}

	if toAddress != nil {
		options = append(options, reclaimoptions.ToAddress(toAddress.Base58()))
	}
	fmt.Println("Reclaiming delegated fund...")
	_, err = cliWallet.ReclaimDelegatedFunds(options...)

	if err != nil {
		printUsage(command, err.Error())
	}

	fmt.Println()
	fmt.Println("Reclaimed delegation ID is: ", delegationID.Base58())
	fmt.Println("Reclaiming delegated fund... [DONE]")
}
