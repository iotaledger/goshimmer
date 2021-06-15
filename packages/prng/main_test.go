package prng

import (
	"os"
	"testing"

	"github.com/iotaledger/goshimmer/packages/testhelper"
)

// TestMain will setup and teardown any unit test
func TestMain(m *testing.M) {
	testhelper.GlobalSetup()
	code := m.Run()
	testhelper.GlobalTeardown()
	os.Exit(code)
}
