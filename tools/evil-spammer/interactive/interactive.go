package interactive

import (
	"github.com/iotaledger/goshimmer/client/evilwallet"
	"github.com/iotaledger/hive.go/types"
)

func Run() {
	mode := NewInteractiveMode()

	printer := NewPrinter(mode)

	printer.printBanner()
	for {
		printer.menu()
		select {
		case <-mode.shutdown:
			return
		}
	}

}

// region Mode /////////////////////////////////////////////////////////////////////////////////////////////////////////

type Mode struct {
	evilWallet *evilwallet.EvilWallet

	shutdown chan types.Empty
}

func NewInteractiveMode() *Mode {
	return &Mode{
		evilWallet: evilwallet.NewEvilWallet(),
		shutdown:   make(chan types.Empty),
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
