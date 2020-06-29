package main

import (
	"flag"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/wallet"
)

func execBalanceCommand(command *flag.FlagSet, cliWallet *wallet.Wallet) {
	err := command.Parse(os.Args[2:])
	if err != nil {
		panic(err)
	}

	confirmedBalance, pendingBalance, err := cliWallet.Balance()
	if err != nil {
		printUsage(err.Error())
	}

	// initialize tabwriter
	w := new(tabwriter.Writer)
	w.Init(os.Stdout, 0, 8, 2, '\t', 0)
	defer w.Flush()

	_, _ = fmt.Fprintf(w, "%s\t%s\n", "BALANCE", "COLOR (status)")
	_, _ = fmt.Fprintf(w, "%s\t%s\n", "-------", "---------------------------")

	if len(confirmedBalance) == 0 && len(pendingBalance) == 0 {
		_, _ = fmt.Fprintf(w, "%s\t%s\n", "<EMPTY>", "<EMPTY>")
	}

	for color, amount := range confirmedBalance {
		_, _ = fmt.Fprintf(w, "%d\t%s\n", amount, color.String())
	}
	for color, amount := range pendingBalance {
		_, _ = fmt.Fprintf(w, "%d\t%s\n", amount, color.String()+" (pending)")
	}
}
