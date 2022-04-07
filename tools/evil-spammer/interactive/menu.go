package interactive

import "fmt"

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
	nOut := 12
	p.Println("Evil Wallet status:", 2)
	p.PrintlnPoint(fmt.Sprintf("Available faucet outputs: %d", nOut), 2)
	p.PrintlnPoint(fmt.Sprintf("Available reuse outputs: %d", nOut), 2)
	p.PrintlnPoint(fmt.Sprintf("Spammed messages: %d", nOut), 2)
	p.PrintlnPoint(fmt.Sprintf("Spammed transactions: %d", nOut), 2)
	p.PrintlnPoint(fmt.Sprintf("Spammed conflicts: %d", nOut), 2)

	p.PrintLine()
}

func (p *Printer) FarewellMessage() {
	//p.mode.evilWallet.
	p.PrintThickLine()
	fmt.Println("                              GOODBYE")
	p.PrintLine()
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
