package main

import (
	"flag"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/iotaledger/goshimmer/client/wallet"
)

func execAddressCommand(command *flag.FlagSet, cliWallet *wallet.Wallet) {
	command.Usage = func() {
		printUsage(command)
	}

	receivePtr := command.Bool("receive", false, "show the latest receive address")
	newReceiveAddressPtr := command.Bool("new", false, "generate a new receive address")
	listPtr := command.Bool("list", false, "list all addresses")
	listUnspentPtr := command.Bool("listunspent", false, "list all unspent addresses")
	listSpentPtr := command.Bool("listspent", false, "list all spent addresses")
	helpPtr := command.Bool("help", false, "display this help screen")

	err := command.Parse(os.Args[2:])
	if err != nil {
		printUsage(command, err.Error())
	}
	if *helpPtr {
		printUsage(command)
	}

	// sanitize flags
	setFlagCount := 0
	if *receivePtr {
		setFlagCount++
	}
	if *listPtr {
		setFlagCount++
	}
	if *listUnspentPtr {
		setFlagCount++
	}
	if *listSpentPtr {
		setFlagCount++
	}
	if *newReceiveAddressPtr {
		setFlagCount++
	}
	if setFlagCount == 0 {
		printUsage(command)
	}
	if setFlagCount > 1 {
		printUsage(command, "please provide only one option at a time")
	}

	if *receivePtr {
		fmt.Println()
		fmt.Println("Latest Receive Address: " + cliWallet.ReceiveAddress().Address().Base58())
	}

	if *newReceiveAddressPtr {
		fmt.Println()
		fmt.Println("New Receive Address: " + cliWallet.NewReceiveAddress().Address().Base58())
	}

	if *listPtr {
		// initialize tab writer
		w := new(tabwriter.Writer)
		w.Init(os.Stdout, 0, 8, 2, '\t', 0)
		defer w.Flush()

		// print header
		fmt.Println()
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\n", "INDEX", "ADDRESS", "SPENT")
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\n", "-----", "--------------------------------------------", "-----")

		addressPrinted := false
		for _, addr := range cliWallet.AddressManager().Addresses() {
			_, _ = fmt.Fprintf(w, "%d\t%s\t%t\n", addr.Index, addr.Base58(), cliWallet.AddressManager().IsAddressSpent(addr.Index))

			addressPrinted = true
		}

		if !addressPrinted {
			_, _ = fmt.Fprintf(w, "%s\t%s\t%s\n", "<EMPTY>", "<EMPTY>", "<EMPTY>")
		}
	}

	if *listUnspentPtr {
		// initialize tab writer
		w := new(tabwriter.Writer)
		w.Init(os.Stdout, 0, 8, 2, '\t', 0)
		defer w.Flush()

		// print header
		fmt.Println()
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\n", "INDEX", "ADDRESS", "SPENT")
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\n", "-------", "--------------------------------------------", "-------")

		addressPrinted := false
		for _, addr := range cliWallet.AddressManager().UnspentAddresses() {
			_, _ = fmt.Fprintf(w, "%d\t%s\t%t\n", addr.Index, addr.String(), cliWallet.AddressManager().IsAddressSpent(addr.Index))

			addressPrinted = true
		}

		if !addressPrinted {
			_, _ = fmt.Fprintf(w, "%s\t%s\t%s\n", "<EMPTY>", "<EMPTY>", "<EMPTY>")
		}
	}

	if *listSpentPtr {
		// initialize tab writer
		w := new(tabwriter.Writer)
		w.Init(os.Stdout, 0, 8, 2, '\t', 0)
		defer w.Flush()

		// print header
		fmt.Println()
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\n", "INDEX", "ADDRESS", "SPENT")
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\n", "-------", "--------------------------------------------", "-------")

		addressPrinted := false
		for _, addr := range cliWallet.AddressManager().SpentAddresses() {
			_, _ = fmt.Fprintf(w, "%d\t%s\t%t\n", addr.Index, addr.Base58(), cliWallet.AddressManager().IsAddressSpent(addr.Index))

			addressPrinted = true
		}

		if !addressPrinted {
			_, _ = fmt.Fprintf(w, "%s\t%s\t%s\n", "<EMPTY>", "<EMPTY>", "<EMPTY>")
		}
	}
}
