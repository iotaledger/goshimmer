package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/mr-tron/base58"

	"github.com/iotaledger/goshimmer/client/wallet"
	"github.com/iotaledger/goshimmer/client/wallet/packages/address"
	"github.com/iotaledger/goshimmer/client/wallet/packages/delegateoptions"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
)

func execDelegateFundsCommand(command *flag.FlagSet, cliWallet *wallet.Wallet) {
	command.Usage = func() {
		printUsage(command)
	}

	helpPtr := command.Bool("help", false, "show this help screen")
	amountPtr := command.Int64("amount", 0, "the amount of tokens that should be delegated")
	colorPtr := command.String("color", "IOTA", "color of the tokens that should delegated")
	delegationAddressPtr := command.String("del-addr", "", "address to delegate funds to. when omitted, wallet delegates to the node it is connected to")
	timelockUntilPtr := command.Int64("until", 0, "unix timestamp until which the delegated funds are timelocked")
	accessManaPledgeIDPtr := command.String("access-mana-id", "", "node ID to pledge access mana to")
	consensusManaPledgeIDPtr := command.String("consensus-mana-id", "", "node ID to pledge consensus mana to")

	err := command.Parse(os.Args[2:])
	if err != nil {
		panic(err)
	}

	if *helpPtr {
		printUsage(command)
	}
	if *amountPtr < int64(ledgerstate.DustThresholdAliasOutputIOTA) {
		printUsage(command, fmt.Sprintf("delegation amount must be greater than %d", ledgerstate.DustThresholdAliasOutputIOTA))
	}
	if *colorPtr == "" {
		printUsage(command, "color must be set")
	}
	var delegationAddress ledgerstate.Address
	delegateToConnectedNode := false
	var status wallet.ServerStatus
	if *delegationAddressPtr == "" {
		var statusErr error
		status, statusErr = cliWallet.ServerStatus()
		if statusErr != nil {
			printUsage(command, fmt.Sprintf("failed to get delegation address from connected node: %s", statusErr.Error()))
		}
		delegationAddress, err = ledgerstate.AddressFromBase58EncodedString(status.DelegationAddress)
		if err != nil {
			printUsage(command, fmt.Sprintf("failed to parse connected node's delegation adddress: %s", err.Error()))
		}
		delegateToConnectedNode = true
	} else {
		delegationAddress, err = ledgerstate.AddressFromBase58EncodedString(*delegationAddressPtr)
		if err != nil {
			printUsage(command, fmt.Sprintf("provided delelegation address %s is not a valid IOTA address: %s", *delegationAddressPtr, err.Error()))
		}
	}

	var fundsColor ledgerstate.Color
	if *amountPtr >= int64(ledgerstate.DustThresholdAliasOutputIOTA) {
		// get color
		switch *colorPtr {
		case "IOTA":
			fundsColor = ledgerstate.ColorIOTA
		case "NEW":
			fundsColor = ledgerstate.ColorMint
		default:
			colorBytes, parseErr := base58.Decode(*colorPtr)
			if parseErr != nil {
				printUsage(command, parseErr.Error())
			}

			fundsColor, _, parseErr = ledgerstate.ColorFromBytes(colorBytes)
			if parseErr != nil {
				printUsage(command, parseErr.Error())
			}
		}
	}

	options := []delegateoptions.DelegateFundsOption{
		delegateoptions.AccessManaPledgeID(*accessManaPledgeIDPtr),
		delegateoptions.ConsensusManaPledgeID(*consensusManaPledgeIDPtr),
	}

	if fundsColor == ledgerstate.ColorIOTA {
		// when we are delegating IOTA, we automatically fulfill the minimum dust requirement of the alias
		options = append(options, delegateoptions.Destination(
			address.Address{AddressBytes: delegationAddress.Array()}, map[ledgerstate.Color]uint64{
				fundsColor: uint64(*amountPtr),
			}))
	} else {
		// when we are delegating anything else, we need IOTAs
		options = append(options, delegateoptions.Destination(
			address.Address{AddressBytes: delegationAddress.Array()}, map[ledgerstate.Color]uint64{
				ledgerstate.ColorIOTA: ledgerstate.DustThresholdAliasOutputIOTA,
				fundsColor:            uint64(*amountPtr),
			}))
	}

	if *timelockUntilPtr != 0 {
		if time.Now().Unix() > *timelockUntilPtr {
			printUsage(command, fmt.Sprintf("delegation timelock %s is in the past. now is: %s", time.Unix(*timelockUntilPtr, 0).String(), time.Unix(time.Now().Unix(), 0).String()))
		} else {
			options = append(options, delegateoptions.DelegateUntil(time.Unix(*timelockUntilPtr, 0)))
		}
	}
	fmt.Println("Delegating funds...")
	_, delegationIDs, err := cliWallet.DelegateFunds(options...)
	if err != nil {
		printUsage(command, err.Error())
	}

	fmt.Println()
	if delegateToConnectedNode {
		fmt.Printf("\nDelegating to node %s, delegation address %s\n", status.ID, delegationAddress.Base58())
	} else {
		fmt.Printf("\nDelegating to address %s\n", delegationAddress.Base58())
	}
	for _, id := range delegationIDs {
		fmt.Println("Delegation ID is: ", id.Base58())
	}
	fmt.Println("Delegating funds... [DONE]")
}
