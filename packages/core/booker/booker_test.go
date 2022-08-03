package booker

import (
	"fmt"
	"testing"

	"github.com/iotaledger/goshimmer/packages/core/ledger"
)

func Test(t *testing.T) {
	testLedger := ledger.New()
	testBooker := New(testLedger)

	fmt.Println(testBooker)
}
