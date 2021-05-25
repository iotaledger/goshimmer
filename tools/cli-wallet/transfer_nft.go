package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/iotaledger/goshimmer/client/wallet"
	"github.com/iotaledger/goshimmer/client/wallet/packages/transfernftoptions"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
)

func execTransferNFTCommand(command *flag.FlagSet, cliWallet *wallet.Wallet) {
	command.Usage = func() {
		printUsage(command)
	}

	helpPtr := command.Bool("help", false, "show this help screen")
	nftIDPtr := command.String("id", "", "unique identifier of the nft that should be transferred")
	addressPtr := command.String("dest-addr", "", "destination address for the transfer")
	resetStateAddrPtr := command.Bool("reset-state-addr", false, "defines whether to set the state address to dest-addr")
	resetDelegationPtr := command.Bool("reset-delegation", false, "defines whether to reset the delegation status of the alias being transferred")
	accessManaPledgeIDPtr := command.String("access-mana-id", "", "node ID to pledge access mana to")
	consensusManaPledgeIDPtr := command.String("consensus-mana-id", "", "node ID to pledge consensus mana to")

	err := command.Parse(os.Args[2:])
	if err != nil {
		panic(err)
	}

	if *helpPtr {
		printUsage(command)
	}

	if *addressPtr == "" {
		printUsage(command, "a destination address must be set for transfer")
	}
	if *nftIDPtr == "" {
		printUsage(command, "an nft (alias) ID must be given for transfer")
	}

	destinationAddress, err := ledgerstate.AddressFromBase58EncodedString(*addressPtr)
	if err != nil {
		printUsage(command, err.Error())
	}

	aliasID, err := ledgerstate.AliasAddressFromBase58EncodedString(*nftIDPtr)
	if err != nil {
		printUsage(command, err.Error())
	}
	fmt.Println("Transferring NFT...")
	_, err = cliWallet.TransferNFT(
		transfernftoptions.Alias(aliasID.Base58()),
		transfernftoptions.ToAddress(destinationAddress.Base58()),
		transfernftoptions.ResetStateAddress(*resetStateAddrPtr),
		transfernftoptions.ResetDelegation(*resetDelegationPtr),
		transfernftoptions.AccessManaPledgeID(*accessManaPledgeIDPtr),
		transfernftoptions.ConsensusManaPledgeID(*consensusManaPledgeIDPtr),
	)
	if err != nil {
		printUsage(command, err.Error())
	}

	fmt.Println()
	fmt.Println("Transferring NFT... [DONE]")
}
