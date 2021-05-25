package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/mr-tron/base58"

	"github.com/iotaledger/goshimmer/client/wallet"
	"github.com/iotaledger/goshimmer/client/wallet/packages/deposittonftoptions"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
)

func execDepositToNFTCommand(command *flag.FlagSet, cliWallet *wallet.Wallet) {
	command.Usage = func() {
		printUsage(command)
	}

	helpPtr := command.Bool("help", false, "show this help screen")
	nftIDPtr := command.String("id", "", "unique identifier of the nft to deposit to")
	colorPtr := command.String("color", "IOTA", "color of funds to deposit")
	amountPtr := command.Int64("amount", 0, "the amount of tokens that are supposed to be deposited")
	accessManaPledgeIDPtr := command.String("access-mana-id", "", "node ID to pledge access mana to")
	consensusManaPledgeIDPtr := command.String("consensus-mana-id", "", "node ID to pledge consensus mana to")

	err := command.Parse(os.Args[2:])
	if err != nil {
		printUsage(command, err.Error())
	}
	if *helpPtr {
		printUsage(command)
	}

	if *nftIDPtr == "" {
		printUsage(command, "id of the nft to deposit to should be given")
	}

	if *amountPtr <= 0 {
		printUsage(command, "amount has to be set and be bigger than 0")
	}
	if *colorPtr == "" {
		printUsage(command, "color must be set")
	}

	aliasID, err := ledgerstate.AliasAddressFromBase58EncodedString(*nftIDPtr)
	if err != nil {
		printUsage(command, err.Error())
	}

	depositBalance := map[ledgerstate.Color]uint64{}
	if *amountPtr > 0 {
		var initColor ledgerstate.Color
		// get color
		switch *colorPtr {
		case "IOTA":
			initColor = ledgerstate.ColorIOTA
		case "NEW":
			initColor = ledgerstate.ColorMint
		default:
			colorBytes, parseErr := base58.Decode(*colorPtr)
			if parseErr != nil {
				printUsage(command, parseErr.Error())
			}

			initColor, _, parseErr = ledgerstate.ColorFromBytes(colorBytes)
			if parseErr != nil {
				printUsage(command, parseErr.Error())
			}
		}
		depositBalance[initColor] = uint64(*amountPtr)
	}
	fmt.Println("Depositing funds into NFT...")
	_, err = cliWallet.DepositFundsToNFT(
		deposittonftoptions.Alias(aliasID.Base58()),
		deposittonftoptions.Amount(depositBalance),
		deposittonftoptions.AccessManaPledgeID(*accessManaPledgeIDPtr),
		deposittonftoptions.ConsensusManaPledgeID(*consensusManaPledgeIDPtr),
	)
	if err != nil {
		printUsage(command, err.Error())
	}
	fmt.Println("Depositing funds into NFT... [DONE]")
}
