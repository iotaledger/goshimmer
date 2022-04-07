package interactive

import (
	"fmt"
	"github.com/AlecAivazis/survey/v2"
	"github.com/iotaledger/goshimmer/client/evilwallet"
	"github.com/iotaledger/hive.go/types"
	"github.com/iotaledger/hive.go/workerpool"
	"os"
)

var (
	spammingWorkerPool       *workerpool.NonBlockingQueuedWorkerPool
	faucetRequestsWorkerPool *workerpool.NonBlockingQueuedWorkerPool
	queueSize                = 5
	preparingWorkerCount     = 1
	spammingWorkerCount      = 5
)

func Run() {
	mode := NewInteractiveMode()

	printer := NewPrinter(mode)

	printer.printBanner()
	for {

		select {
		case <-mode.shutdown:
			printer.FarewellMessage()
			os.Exit(0)
			return
		case act := <-mode.action:
			switch act {
			case spam:
				fmt.Println("SPAM")
			case walletDetails:
				printer.EvilWalletStatus()
			default:
				mode.menu()
			}
		}
	}

}

func configure() {
	spammingWorkerPool = workerpool.NewNonBlockingQueuedWorkerPool(func(task workerpool.Task) {
		fmt.Println("Spamming!")
	}, workerpool.WorkerCount(spammingWorkerCount), workerpool.QueueSize(queueSize))

	faucetRequestsWorkerPool = workerpool.NewNonBlockingQueuedWorkerPool(func(task workerpool.Task) {
		fmt.Println("Requesting funds!")
	}, workerpool.WorkerCount(preparingWorkerCount), workerpool.QueueSize(queueSize))

}

// region Mode /////////////////////////////////////////////////////////////////////////////////////////////////////////

type action int

const (
	walletDetails action = iota
	spam
	shutdown
)

var actions = []string{"Evil wallet details", "spam", "close"}

type Mode struct {
	evilWallet *evilwallet.EvilWallet
	action     chan action
	nextAction *actionSurvey
	shutdown   chan types.Empty
}

func NewInteractiveMode() *Mode {
	return &Mode{
		evilWallet: evilwallet.NewEvilWallet(),
		shutdown:   make(chan types.Empty),
	}
}

func (m *Mode) menu() {
	// perform the questions
	err := survey.Ask(actionQuestion, m.nextAction)
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	m.onMenuAction()
}

func (m *Mode) onMenuAction() {
	switch m.nextAction.Action {
	case "spamDetails":
		m.action <- walletDetails
	case "spam":
		m.action <- spam
	case "shutdown":
		m.shutdown <- types.Void
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
