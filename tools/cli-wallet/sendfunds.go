package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/mr-tron/base58"

	"github.com/iotaledger/goshimmer/client/wallet"
	"github.com/iotaledger/goshimmer/client/wallet/packages/address"
	"github.com/iotaledger/goshimmer/client/wallet/packages/sendoptions"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
)

func execSendFundsCommand(command *flag.FlagSet, cliWallet *wallet.Wallet) {
	helpPtr := command.Bool("help", false, "show this help screen")
	addressPtr := command.String("dest-addr", "", "destination address for the transfer")
	amountPtr := command.Int64("amount", 0, "the amount of tokens that are supposed to be sent")
	colorPtr := command.String("color", "IOTA", "(optional) color of the tokens to transfer")
	timelockPtr := command.Int64("lock-until", 0, "(optional) unix timestamp until which time the sent funds are locked from spending")
	fallbackAddressPtr := command.String("fallb-addr", "", "(optional) fallback address that can claim back the (unspent) sent funds after fallback deadline")
	fallbackDeadlinePtr := command.Int64("fallb-deadline", 0, "(optional) unix timestamp after which only the fallback address can claim the funds back")
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
		printUsage(command, "dest-addr has to be set")
	}
	if *amountPtr <= 0 {
		printUsage(command, "amount has to be set and be bigger than 0")
	}
	if *colorPtr == "" {
		printUsage(command, "color must be set")
	}

	destinationAddress, err := ledgerstate.AddressFromBase58EncodedString(*addressPtr)
	if err != nil {
		printUsage(command, err.Error())
		return
	}

	var color ledgerstate.Color
	switch *colorPtr {
	case "IOTA":
		color = ledgerstate.ColorIOTA
	case "NEW":
		color = ledgerstate.ColorMint
	default:
		colorBytes, parseErr := base58.Decode(*colorPtr)
		if parseErr != nil {
			printUsage(command, parseErr.Error())
		}

		color, _, parseErr = ledgerstate.ColorFromBytes(colorBytes)
		if parseErr != nil {
			printUsage(command, parseErr.Error())
		}
	}

	options := []sendoptions.SendFundsOption{
		sendoptions.Destination(address.Address{
			AddressBytes: destinationAddress.Array(),
		}, uint64(*amountPtr), color),
		sendoptions.AccessManaPledgeID(*accessManaPledgeIDPtr),
		sendoptions.ConsensusManaPledgeID(*consensusManaPledgeIDPtr),
	}

	nowis := time.Now()
	if *timelockPtr > 0 {
		timelock := time.Unix(*timelockPtr, 0)
		if timelock.Before(nowis) {
			printUsage(command, fmt.Sprintf("can't lock funds in the past: lock-until is %s", timelock.String()))
		}
		options = append(options, sendoptions.LockUntil(timelock))
	}

	if *fallbackAddressPtr != "" || *fallbackDeadlinePtr > 0 {
		if !(*fallbackAddressPtr != "" && *fallbackDeadlinePtr > 0) {
			printUsage(command, "please provide both fallb-addr and fallb-deadline arguments for conditional sending")
		}
		fAddy, aErr := ledgerstate.AddressFromBase58EncodedString(*fallbackAddressPtr)
		if aErr != nil {
			printUsage(command, fmt.Sprintf("wrong fallback address: %s", aErr.Error()))
		}
		fDeadline := time.Unix(*fallbackDeadlinePtr, 0)
		if fDeadline.Before(nowis) {
			printUsage(command, fmt.Sprintf("fallback deadline %s is in the past", fDeadline.String()))
		}
		options = append(options, sendoptions.Fallback(fAddy, fDeadline))
	}
	// set pending outputs explicitly to false (even though it should be false by default)
	options = append(options, sendoptions.UsePendingOutputs(false))
	fmt.Println("Sending funds...")
	_, err = cliWallet.SendFunds(options...)
	if err != nil {
		printUsage(command, err.Error())
	}

	fmt.Println()
	fmt.Println("Sending funds ... [DONE]")
}
