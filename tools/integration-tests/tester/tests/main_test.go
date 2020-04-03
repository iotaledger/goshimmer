package tests

import (
	"os"
	"testing"

	"github.com/iotaledger/goshimmer/tools/integration-tests/tester/framework"
)

var f *framework.Framework

func TestMain(m *testing.M) {
	f = framework.New()

	// call the tests
	os.Exit(m.Run())
}
