package evilspammer

import (
	"github.com/iotaledger/goshimmer/client/evilwallet"
	"time"
)

// BigWalletsNeeded calculates how many big wallets needs to be prepared for a spam based on provided spam details.
func BigWalletsNeeded(rate int, timeUnit, duration time.Duration) int {
	bigWalletSize := evilwallet.FaucetRequestSplitNumber * evilwallet.FaucetRequestSplitNumber
	outputsNeeded := rate * int(duration/timeUnit)
	walletsNeeded := outputsNeeded/bigWalletSize + 1
	return walletsNeeded
}
