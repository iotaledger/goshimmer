package tests

import (
	"fmt"
	"os"
	"testing"

	"github.com/iotaledger/goshimmer/tools/integration-tests/tester/framework"
	"github.com/stretchr/testify/assert"
)

var f *framework.Framework

func TestMain(m *testing.M) {
	f = framework.New()

	// call the tests
	os.Exit(m.Run())
}

func TestExample(t *testing.T) {
	fmt.Println("This is an independent test.")

	fmt.Printf("There are %d peers in the Docker network available.\n", len(f.Peers()))
	assert.Equal(t, 6, len(f.Peers()))
}
