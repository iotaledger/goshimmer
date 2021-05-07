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

	// fetch timelocked balances
	confirmedTimelocked, pendingTimelocked, err := cliWallet.TimelockedBalances()
	if err != nil {
		printUsage(nil, err.Error())
	}

	if len(confirmedTimelocked) > 0 || len(pendingTimelocked) > 0 {
		// initialize tab writer
		wT := new(tabwriter.Writer)
		wT.Init(os.Stdout, 0, 8, 2, '\t', 0)
		//defer w.Flush()
		// print header
		fmt.Println()
		fmt.Println("Timelocked Token Balances")
		fmt.Println()
		_, _ = fmt.Fprintf(wT, "%s\t%s\t%s\t%s\t%s\n", "STATUS", "LOCKED UNTIL", "BALANCE", "COLOR", "TOKEN NAME")
		_, _ = fmt.Fprintf(wT, "%s\t%s\t%s\t%s\t%s\n", "------", "------------------------------", "---------------", "--------------------------------------------", "-------------------------")
		if confirmedTimelocked != nil {
			confirmedTimelocked.Sort()
		}
		for _, timelockedBalance := range confirmedTimelocked {
			// print balances
			for color, amount := range timelockedBalance.Balance {
				_, _ = fmt.Fprintf(wT, "%s\t%s\t%d %s\t%s\t%s\n",
					"[ OK ]", timelockedBalance.LockedUntil.String(), amount, cliWallet.AssetRegistry().Symbol(color), color.String(),
					cliWallet.AssetRegistry().Name(color))
			}
		}
		if pendingTimelocked != nil {
			pendingTimelocked.Sort()
		}
		for _, timelockedBalance := range pendingTimelocked {
			// print balances
			for color, amount := range timelockedBalance.Balance {
				_, _ = fmt.Fprintf(wT, "%s\t%s\t%d %s\t%s\t%s\n",
					"[PEND]", timelockedBalance.LockedUntil.String(), amount, cliWallet.AssetRegistry().Symbol(color), color.String(),
					cliWallet.AssetRegistry().Name(color))
			}
		}
		wT.Flush()
	}

	// fetch conditional balances
	confirmedConditional, pendingConditional, err := cliWallet.ConditionalBalances()
	if err != nil {
		printUsage(nil, err.Error())
	}

	if len(confirmedConditional) > 0 || len(pendingConditional) > 0 {
		// initialize tab writer
		wC := new(tabwriter.Writer)
		wC.Init(os.Stdout, 0, 8, 2, '\t', 0)
		//defer w.Flush()
		// print header
		fmt.Println()
		fmt.Println("Conditional Token Balances - execute `claim-conditional` command to sweep these funds into wallet")
		fmt.Println()
		_, _ = fmt.Fprintf(wC, "%s\t%s\t%s\t%s\t%s\n", "STATUS", "OWNED UNTIL", "BALANCE", "COLOR", "TOKEN NAME")
		_, _ = fmt.Fprintf(wC, "%s\t%s\t%s\t%s\t%s\n", "------", "------------------------------", "---------------", "--------------------------------------------", "-------------------------")
		if confirmedConditional != nil {
			confirmedConditional.Sort()
		}
		for _, conditionalBalance := range confirmedConditional {
			// print balances
			for color, amount := range conditionalBalance.Balance {
				_, _ = fmt.Fprintf(wC, "%s\t%s\t%d %s\t%s\t%s\n",
					"[ OK ]", conditionalBalance.FallbackDeadline.String(), amount, cliWallet.AssetRegistry().Symbol(color), color.String(),
					cliWallet.AssetRegistry().Name(color))
			}
		}
		if pendingConditional != nil {
			pendingConditional.Sort()
		}
		for _, conditionalBalance := range pendingConditional {
			// print balances
			for color, amount := range conditionalBalance.Balance {
				_, _ = fmt.Fprintf(wC, "%s\t%s\t%d %s\t%s\t%s\n",
					"[PEND]", conditionalBalance.FallbackDeadline.String(), amount, cliWallet.AssetRegistry().Symbol(color), color.String(),
					cliWallet.AssetRegistry().Name(color))
			}
		}
		wC.Flush()
	}

	// fetch balances from wallet
	confirmedGovAliasBalance, confirmedStateAliasBalance, pendingGovAliasBalance, pendingStateAliasBalance, err := cliWallet.AliasBalance()
	confirmedDel, pendingDel, err := cliWallet.DelegatedAliasBalance()

	// process returned data, prepare for printing

	// remove alias outputs from confirmedStateAliasBalance that are state & governance controlled
	for aliasID, _ := range confirmedStateAliasBalance {
		if _, has := confirmedGovAliasBalance[aliasID]; has {
			delete(confirmedStateAliasBalance, aliasID)
		}
	}
	// remove alias outputs from pendingStateAliasBalance that are state & governance controlled
	for aliasID, _ := range pendingStateAliasBalance {
		if _, has := pendingGovAliasBalance[aliasID]; has {
			delete(pendingStateAliasBalance, aliasID)
		}
	}

	// remove confirmed delegated alias outputs from governance controlled balance
	for aliasID, _ := range confirmedDel {
		if _, has := confirmedGovAliasBalance[aliasID]; has {
			delete(confirmedGovAliasBalance, aliasID)
		}
	}
	// remove pending delegated alias outputs from governance controlled balance
	for aliasID, _ := range pendingDel {
		if _, has := pendingGovAliasBalance[aliasID]; has {
			delete(pendingGovAliasBalance, aliasID)
		}
	}

	if err != nil {
		printUsage(nil, err.Error())
	}

	// first print governed aliases
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
			if _, has := confirmedDel[aliasID]; has {
				continue
			}
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
			if _, has := pendingDel[aliasID]; has {
				continue
			}
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

	// then print only state controlled aliases
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

	// finally print delegated aliases
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
