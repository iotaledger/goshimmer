package wallet

import (
	"testing"

	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/address"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/transaction"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/wallet/sendfunds"
	"github.com/stretchr/testify/assert"
)

func TestWallet_SendFunds(t *testing.T) {
	// define sub-tests by providing a list of parameters and a validator function
	testCases := []struct {
		name       string
		parameters []sendfunds.Option
		validator  func(t *testing.T, err error)
	}{
		// test if not providing a destination triggers an error
		{
			name: "missingDestination",
			parameters: []sendfunds.Option{
				sendfunds.Remainder(address.Empty),
			},
			validator: func(t *testing.T, err error) {
				assert.Error(t, err, "calling SendFunds without a Destination should trigger an error")
				assert.Equal(t, "you need to provide at least one Destination for a valid transfer to be issued", err.Error(), "the error message is wrong")
			},
		},

		// test if providing an invalid destination (amount <= 0) triggers an error
		{
			name: "zeroAmount",
			parameters: []sendfunds.Option{
				sendfunds.Destination(address.Empty, 1),
				sendfunds.Destination(address.Empty, 0),
				sendfunds.Destination(address.Empty, 123),
			},
			validator: func(t *testing.T, err error) {
				assert.Error(t, err, "calling SendFunds without an invalid Destination (amount <= 0) should trigger an error")
				assert.Equal(t, "the amount provided in the destinations needs to be larger than 0", err.Error(), "the error message is wrong")
			},
		},
	}

	// execute sub-tests and hand in the results to the validator function
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			// create our test wallet
			wallet := New(GenericConnector())

			// validate the result of the function call
			testCase.validator(t, wallet.SendFunds(testCase.parameters...))
		})
	}
}

type testMockConnector struct {
	outputs map[address.Address][]Output
}

func newTestMockConnector() *testMockConnector {
	return &testMockConnector{
		outputs: map[address.Address][]Output{
			address.Address{1}: {
				Output{
					id:             transaction.OutputID{},
					balances:       nil,
					inclusionState: 0,
				},
			},
		},
	}
}

func (connector *testMockConnector) UnspentOutputs(addresses ...address.Address) (outputs []Output) {
	outputs = make([]Output, 0)

	for _, addr := range addresses {
		for _, output := range connector.outputs[addr] {
			outputs = append(outputs, output)
		}
	}

	return
}

var _ Connector = &testMockConnector{}
