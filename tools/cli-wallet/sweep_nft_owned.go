package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/iotaledger/goshimmer/client/wallet"
	"github.com/iotaledger/goshimmer/client/wallet/packages/sweepnftownednftsoptions"
	"github.com/iotaledger/goshimmer/client/wallet/packages/sweepnftownedoptions"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
)

func execSweepNFTOwnedFundsCommand(command *flag.FlagSet, cliWallet *wallet.Wallet) {
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
	options := []sweepnftownedoptions.SweepNFTOwnedFundsOption{
		sweepnftownedoptions.Alias(aliasID.Base58()),
		sweepnftownedoptions.AccessManaPledgeID(*accessManaPledgeIDPtr),
		sweepnftownedoptions.ConsensusManaPledgeID(*consensusManaPledgeIDPtr),
	}

	if *toAddressPtr != "" {
		options = append(options, sweepnftownedoptions.ToAddress(*toAddressPtr))
	}

	_, err = cliWallet.SweepNFTOwnedFunds(options...)
	if err != nil {
		printUsage(command, err.Error())
	}

	fmt.Println()
	fmt.Println("Sweeping NFT owned funds... [DONE]")
}

func execSweepNFTOwnedNFTsCommand(command *flag.FlagSet, cliWallet *wallet.Wallet) {
	command.Usage = func() {
		printUsage(command)
	}

	helpPtr := command.Bool("help", false, "show this help screen")
	nftIDPtr := command.String("id", "", "unique identifier of the nft that should be checked for owning other nfts")
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
	options := []sweepnftownednftsoptions.SweepNFTOwnedNFTsOption{
		sweepnftownednftsoptions.Alias(aliasID.Base58()),
		sweepnftownednftsoptions.AccessManaPledgeID(*accessManaPledgeIDPtr),
		sweepnftownednftsoptions.ConsensusManaPledgeID(*consensusManaPledgeIDPtr),
	}

	if *toAddressPtr != "" {
		options = append(options, sweepnftownednftsoptions.ToAddress(*toAddressPtr))
	}

	var sweptNFTs []*ledgerstate.AliasAddress
	fmt.Println("Sweeping NFT owned NFTs...")
	_, sweptNFTs, err = cliWallet.SweepNFTOwnedNFTs(options...)
	if err != nil {
		printUsage(command, err.Error())
	}

	fmt.Println()
	for _, sweptNFT := range sweptNFTs {
		fmt.Println(fmt.Sprintf("Swept NFT %s into the wallet", sweptNFT.Base58()))
	}
	fmt.Println("Sweeping NFT owned NFTs... [DONE]")
}
