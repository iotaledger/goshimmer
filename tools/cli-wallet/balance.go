package main

import (
	"flag"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/iotaledger/goshimmer/client/wallet"
)

func execBalanceCommand(command *flag.FlagSet, cliWallet *wallet.Wallet) {
	err := command.Parse(os.Args[2:])
	if err != nil {
		panic(err)
	}

	confirmedBalance, pendingBalance, err := cliWallet.Balance()
	if err != nil {
		printUsage(nil, err.Error())
	}

	// initialize tab writer
	w := new(tabwriter.Writer)
	w.Init(os.Stdout, 0, 8, 2, '\t', 0)
	defer w.Flush()

	// print header
	fmt.Println()
	_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", "STATUS", "BALANCE", "COLOR", "TOKEN NAME")
	_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", "------", "---------------", "--------------------------------------------", "-------------------------")

	// print empty if no balances founds
	if len(confirmedBalance) == 0 && len(pendingBalance) == 0 {
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", "<EMPTY>", "<EMPTY>", "<EMPTY>", "<EMPTY>")

		return
	}

	// print balances
	for color, amount := range confirmedBalance {
		_, _ = fmt.Fprintf(w, "%s\t%d %s\t%s\t%s\n", "[ OK ]", amount, cliWallet.AssetRegistry().Symbol(color), color.String(), cliWallet.AssetRegistry().Name(color))
	}
	for color, amount := range pendingBalance {
		_, _ = fmt.Fprintf(w, "%s\t%d\t%s\t%s\n", "[PEND]", amount, color.String(), cliWallet.AssetRegistry().Name(color))
	}
}
