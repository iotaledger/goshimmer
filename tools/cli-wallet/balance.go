package main

import (
	"flag"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/iotaledger/goshimmer/client/wallet"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
)

func execBalanceCommand(command *flag.FlagSet, cliWallet *wallet.Wallet) {
	err := command.Parse(os.Args[2:])
	if err != nil {
		panic(err)
	}

	// print token balances

	confirmedBalance, pendingBalance, err := cliWallet.AvailableBalance()
	if err != nil {
		printUsage(nil, err.Error())
	}

	// initialize tab writer
	w := new(tabwriter.Writer)
	w.Init(os.Stdout, 0, 8, 2, '\t', 0)
	//defer w.Flush()
	// print header
	fmt.Println()
	fmt.Println("Available Token Balances")
	fmt.Println()
	_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", "STATUS", "BALANCE", "COLOR", "TOKEN NAME")
	_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", "------", "---------------", "--------------------------------------------", "-------------------------")

	// print empty if no balances founds
	if len(confirmedBalance) == 0 && len(pendingBalance) == 0 {
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", "<EMPTY>", "<EMPTY>", "<EMPTY>", "<EMPTY>")

	} else {
		// print balances
		for color, amount := range confirmedBalance {
			_, _ = fmt.Fprintf(w, "%s\t%d %s\t%s\t%s\n", "[ OK ]", amount, cliWallet.AssetRegistry().Symbol(color), color.String(), cliWallet.AssetRegistry().Name(color))
		}
		for color, amount := range pendingBalance {
			_, _ = fmt.Fprintf(w, "%s\t%d\t%s\t%s\n", "[PEND]", amount, color.String(), cliWallet.AssetRegistry().Name(color))
		}
	}
	w.Flush()

	// print alias balances
	confirmedGovAliasBalance, confirmedStateAliasBalance, pendingGovAliasBalance, pendingStateAliasBalance, err := cliWallet.AliasBalance()

	if err != nil {
		printUsage(nil, err.Error())
	}

	if len(confirmedGovAliasBalance) != 0 || len(pendingGovAliasBalance) != 0 {
		// initialize tab writer
		wAlias := new(tabwriter.Writer)
		wAlias.Init(os.Stdout, 0, 8, 2, '\t', 0)

		// print header
		fmt.Println()
		fmt.Println("Governance Controlled Aliases")
		fmt.Println()
		_, _ = fmt.Fprintf(wAlias, "%s\t%s\t%s\t%s\t%s\n", "STATUS", "ALIAS ID", "BALANCE", "COLOR", "TOKEN NAME")
		_, _ = fmt.Fprintf(wAlias, "%s\t%s\t%s\t%s\t%s\n", "------", "--------------------------------------------", "---------------", "--------------------------------------------", "-------------------------")

		for aliasID, alias := range confirmedGovAliasBalance {
			balances := alias.Balances()
			i := 0
			balances.ForEach(func(color ledgerstate.Color, balance uint64) bool {
				if i == 0 {
					_, _ = fmt.Fprintf(wAlias, "%s\t%s\t%d\t%s\t%s\n", "[ OK ]", aliasID.Base58(), balance, color.String(), cliWallet.AssetRegistry().Name(color))
				} else {
					_, _ = fmt.Fprintf(wAlias, "%s\t%s\t%d\t%s\t%s\n", "", "", balance, color.String(), cliWallet.AssetRegistry().Name(color))
				}
				i++
				return true
			})
		}
		for aliasID, alias := range pendingGovAliasBalance {
			balances := alias.Balances()
			i := 0
			balances.ForEach(func(color ledgerstate.Color, balance uint64) bool {
				if i == 0 {
					_, _ = fmt.Fprintf(wAlias, "%s\t%s\t%d\t%s\t%s\n", "[PEND]", aliasID.Base58(), balance, color.String(), cliWallet.AssetRegistry().Name(color))
				} else {
					_, _ = fmt.Fprintf(wAlias, "%s\t%s\t%d\t%s\t%s\n", "", "", balance, color.String(), cliWallet.AssetRegistry().Name(color))
				}
				i++
				return true
			})
		}

		wAlias.Flush()
	}

	for aliasID, _ := range confirmedStateAliasBalance {
		if _, has := confirmedGovAliasBalance[aliasID]; has {
			delete(confirmedStateAliasBalance, aliasID)
		}
	}
	for aliasID, _ := range pendingStateAliasBalance {
		if _, has := pendingGovAliasBalance[aliasID]; has {
			delete(pendingStateAliasBalance, aliasID)
		}
	}

	if len(confirmedStateAliasBalance) != 0 || len(pendingStateAliasBalance) != 0 {
		// initialize tab writer
		sAlias := new(tabwriter.Writer)
		sAlias.Init(os.Stdout, 0, 8, 2, '\t', 0)

		// print header
		fmt.Println()
		fmt.Println("Only State Controlled Aliases")
		fmt.Println()
		_, _ = fmt.Fprintf(sAlias, "%s\t%s\t%s\t%s\t%s\n", "STATUS", "ALIAS ID", "BALANCE", "COLOR", "TOKEN NAME")
		_, _ = fmt.Fprintf(sAlias, "%s\t%s\t%s\t%s\t%s\n", "------", "--------------------------------------------", "---------------", "--------------------------------------------", "-------------------------")

		for aliasID, alias := range confirmedStateAliasBalance {
			balances := alias.Balances()
			i := 0
			balances.ForEach(func(color ledgerstate.Color, balance uint64) bool {
				if i == 0 {
					_, _ = fmt.Fprintf(sAlias, "%s\t%s\t%d\t%s\t%s\n", "[ OK ]", aliasID.Base58(), balance, color.String(), cliWallet.AssetRegistry().Name(color))
				} else {
					_, _ = fmt.Fprintf(sAlias, "%s\t%s\t%d\t%s\t%s\n", "", "", balance, color.String(), cliWallet.AssetRegistry().Name(color))
				}
				i++
				return true
			})
		}
		for aliasID, alias := range pendingStateAliasBalance {
			balances := alias.Balances()
			i := 0
			balances.ForEach(func(color ledgerstate.Color, balance uint64) bool {
				if i == 0 {
					_, _ = fmt.Fprintf(sAlias, "%s\t%s\t%d\t%s\t%s\n", "[PEND]", aliasID.Base58(), balance, color.String(), cliWallet.AssetRegistry().Name(color))
				} else {
					_, _ = fmt.Fprintf(sAlias, "%s\t%s\t%d\t%s\t%s\n", "", "", balance, color.String(), cliWallet.AssetRegistry().Name(color))
				}
				i++
				return true
			})
		}

		sAlias.Flush()
	}

	confirmedDel, pendingDel, err := cliWallet.DelegatedAliasBalance()
	if len(confirmedDel) != 0 || len(pendingDel) != 0 {
		// initialize tab writer
		wDel := new(tabwriter.Writer)
		wDel.Init(os.Stdout, 0, 8, 2, '\t', 0)

		// print header
		fmt.Println()
		fmt.Println("Delegated Funds")
		fmt.Println()
		_, _ = fmt.Fprintf(wDel, "%s\t%s\t%s\t%s\t%s\n", "STATUS", "DELEGATION (ALIAS) ID", "BALANCE", "COLOR", "TOKEN NAME")
		_, _ = fmt.Fprintf(wDel, "%s\t%s\t%s\t%s\t%s\n", "------", "--------------------------------------------", "---------------", "--------------------------------------------", "-------------------------")

		for aliasID, alias := range confirmedDel {
			balances := alias.Balances()
			i := 0
			balances.ForEach(func(color ledgerstate.Color, balance uint64) bool {
				if i == 0 {
					_, _ = fmt.Fprintf(wDel, "%s\t%s\t%d\t%s\t%s\n", "[ OK ]", aliasID.Base58(), balance, color.String(), cliWallet.AssetRegistry().Name(color))
				} else {
					_, _ = fmt.Fprintf(wDel, "%s\t%s\t%d\t%s\t%s\n", "", "", balance, color.String(), cliWallet.AssetRegistry().Name(color))
				}
				i++
				return true
			})
		}
		for aliasID, alias := range pendingDel {
			balances := alias.Balances()
			i := 0
			balances.ForEach(func(color ledgerstate.Color, balance uint64) bool {
				if i == 0 {
					_, _ = fmt.Fprintf(wDel, "%s\t%s\t%d\t%s\t%s\n", "[PEND]", aliasID.Base58(), balance, color.String(), cliWallet.AssetRegistry().Name(color))
				} else {
					_, _ = fmt.Fprintf(wDel, "%s\t%s\t%d\t%s\t%s\n", "", "", balance, color.String(), cliWallet.AssetRegistry().Name(color))
				}
				i++
				return true
			})
		}

		wDel.Flush()
	}
}
