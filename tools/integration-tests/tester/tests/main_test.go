// Package tests provides the possibility to write integration tests in regular Go style.
// The integration test framework is initialized before any test in the package runs and
// thus can readily be used to make requests to peers and read their logs.
//
// Each tested feature should reside in its own test file and define tests cases as necessary.
package tests

import (
	"os"
	"testing"

	"github.com/iotaledger/goshimmer/tools/integration-tests/tester/framework"
)

var f *framework.Framework

// TestMain gets called by the test utility and is executed before any other test in this package.
// It is therefore used to initialize the integration testing framework.
func TestMain(m *testing.M) {
	f = framework.New()

	// call the tests
	os.Exit(m.Run())
}
