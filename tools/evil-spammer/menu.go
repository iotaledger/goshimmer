package main

import (
	"fmt"
	"github.com/iotaledger/goshimmer/client/evilwallet"
)

// region Printer /////////////////////////////////////////////////////////////////////////////////////////////////////////

type Printer struct {
	mode *Mode
}

func NewPrinter(mode *Mode) *Printer {
	return &Printer{
		mode: mode,
	}
}

func (p *Printer) Println(s string, indent int) {
	pre := "█"
	for i := 0; i < indent; i++ {
		pre += "▓"
	}
	fmt.Println(pre, s)
}

func (p *Printer) PrintlnPoint(s string, indent int) {
	pre := ""
	for i := 0; i < indent; i++ {
		pre += " "
	}
	fmt.Println(pre, "▀▄", s)
}

func (p *Printer) PrintlnInput(s string) {
	fmt.Println("█▓>>", s)
}
func (p *Printer) PrintThickLine() {
	fmt.Println("\n  ooo▄▄▓░░▀▀▀▀▄▓▓░░▄▄▄▓▓░░▄▒▄▀█▒▓▄▓▓░░▄▄▒▄▄█▒▓▄▄▀▀▄▓▒▄▄█▒▓▓▀▓▓░░░░█▒▄▄█▒▓░▄▄ooo")
	fmt.Println()
}

func (p *Printer) PrintLine() {
	fmt.Println("▄▀▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▀▄")
}

func (p *Printer) menu() {

}
func (p *Printer) printBanner() {
	fmt.Println("▓█████  ██▒   █▓ ██▓ ██▓                                                   \n▓█   ▀ ▓██░   █▒▓██▒▓██▒                                                   \n▒███    ▓██  █▒░▒██▒▒██░                                                   \n▒▓█  ▄   ▒██ █░░░██░▒██░                                                   \n░▒████▒   ▒▀█░  ░██░░██████▒                                               \n░░ ▒░ ░   ░ ▐░  ░▓  ░ ▒░▓  ░                                               \n ░ ░  ░   ░ ░░   ▒ ░░ ░ ▒  ░                                               \n   ░        ░░   ▒ ░  ░ ░                                                  \n   ░  ░      ░   ░      ░  ░                                               \n            ░                                                              \n           ██████  ██▓███   ▄▄▄       ███▄ ▄███▓ ███▄ ▄███▓▓█████  ██▀███  \n         ▒██    ▒ ▓██░  ██▒▒████▄    ▓██▒▀█▀ ██▒▓██▒▀█▀ ██▒▓█   ▀ ▓██ ▒ ██▒\n         ░ ▓██▄   ▓██░ ██▓▒▒██  ▀█▄  ▓██    ▓██░▓██    ▓██░▒███   ▓██ ░▄█ ▒\n           ▒   ██▒▒██▄█▓▒ ▒░██▄▄▄▄██ ▒██    ▒██ ▒██    ▒██ ▒▓█  ▄ ▒██▀▀█▄  \n         ▒██████▒▒▒██▒ ░  ░ ▓█   ▓██▒▒██▒   ░██▒▒██▒   ░██▒░▒████▒░██▓ ▒██▒\n         ▒ ▒▓▒ ▒ ░▒▓▒░ ░  ░ ▒▒   ▓▒█░░ ▒░   ░  ░░ ▒░   ░  ░░░ ▒░ ░░ ▒▓ ░▒▓░\n         ░ ░▒  ░ ░░▒ ░       ▒   ▒▒ ░░  ░      ░░  ░      ░ ░ ░  ░  ░▒ ░ ▒░\n         ░  ░  ░  ░░         ░   ▒   ░      ░   ░      ░      ░     ░░   ░ \n               ░                 ░  ░       ░          ░      ░  ░   ░     \n                                                                           ")
	p.PrintThickLine()
	p.Println("Interactive mode enabled", 1)
}

func (p *Printer) EvilWalletStatus() {
	//p.mode.evilWallet.
	p.Println("Evil Wallet status:", 2)
	p.PrintlnPoint(fmt.Sprintf("Available faucet outputs: %d", p.mode.evilWallet.UnspentOutputsLeft(evilwallet.Fresh)), 2)
	p.PrintlnPoint(fmt.Sprintf("Available reuse outputs: %d", p.mode.evilWallet.UnspentOutputsLeft(evilwallet.Reuse)), 2)
	p.PrintlnPoint(fmt.Sprintf("Spammed messages: %d", p.mode.msgSent.Load()), 2)
	p.PrintlnPoint(fmt.Sprintf("Spammed transactions: %d", p.mode.txSent.Load()), 2)
	p.PrintlnPoint(fmt.Sprintf("Spammed conflicts: %d", p.mode.conflictsSent.Load()), 2)

	p.PrintLine()
}

func (p *Printer) FarewellMessage() {
	//p.mode.evilWallet.
	p.PrintLine()
	fmt.Println("                              GOODBYE")
	p.PrintLine()
}

func (p *Printer) SettingFundsMessage() {
	if p.mode.autoFundsPrepareEnabled {
		p.Println("Auto funds creation enabled", 2)
	} else {
		p.Println("Auto funds creation disabled", 2)
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
