package wallet

import (
	"fmt"
	"testing"

	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/address"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/balance"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/transaction"
	"github.com/iotaledger/hive.go/bitmask"
	"github.com/stretchr/testify/assert"
)

func TestWallet_SendFunds(t *testing.T) {
	// create test seed
	seed := NewSeed()

	// define sub-tests by providing a list of parameters and a validator function
	testCases := []struct {
		name       string
		parameters []SendFundsOption
		validator  func(t *testing.T, err error)
	}{
		// test if not providing a destination triggers an error
		{
			name: "missingDestination",
			parameters: []SendFundsOption{
				Remainder(AddressEmpty),
			},
			validator: func(t *testing.T, err error) {
				assert.Error(t, err, "calling SendFunds without a Destination should trigger an error")
				assert.Equal(t, "you need to provide at least one Destination for a valid transfer to be issued", err.Error(), "the error message is wrong")
			},
		},

		// test if providing an invalid destination (amount <= 0) triggers an error
		{
			name: "zeroAmount",
			parameters: []SendFundsOption{
				Destination(address.Empty, 1),
				Destination(address.Empty, 0),
				Destination(address.Empty, 123),
			},
			validator: func(t *testing.T, err error) {
				assert.Error(t, err, "calling SendFunds without an invalid Destination (amount <= 0) should trigger an error")
				assert.Equal(t, "the amount provided in the destinations needs to be larger than 0", err.Error(), "the error message is wrong")
			},
		},

		// test if providing an invalid destination (amount <= 0) triggers an error
		{
			name: "validTransfer",
			parameters: []SendFundsOption{
				Destination(address.Empty, 1200),
			},
			validator: func(t *testing.T, err error) {
				fmt.Println(err)
			},
		},
	}

	// execute sub-tests and hand in the results to the validator function
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			// create mocked connector
			mockedConnector := newMockConnector(
				Output{
					address:       seed.Address(0),
					transactionID: transaction.GenesisID,
					balances: map[balance.Color]uint64{
						balance.ColorIOTA: 1337,
						balance.Color{3}:  1338,
					},
					inclusionState: InclusionState{
						Liked:     true,
						Confirmed: true,
					},
				},
				Output{
					address:       seed.Address(0),
					transactionID: transaction.ID{3},
					balances: map[balance.Color]uint64{
						balance.ColorIOTA: 663,
						balance.Color{4}:  1338,
					},
					inclusionState: InclusionState{
						Liked:     true,
						Confirmed: true,
					},
				},
			)

			// create our test wallet
			wallet := New(
				Import(seed, 0, []bitmask.BitMask{}),
				GenericConnector(mockedConnector),
			)

			// validate the result of the function call
			testCase.validator(t, wallet.SendFunds(testCase.parameters...))
		})
	}
}

type mockConnector struct {
	outputs map[Address]map[transaction.ID]Output
}

func (connector *mockConnector) SendTransaction(tx *transaction.Transaction) {
	fmt.Println("SENT TRANSACTION: ", tx)
}

func newMockConnector(outputs ...Output) (connector *mockConnector) {
	connector = &mockConnector{
		outputs: make(map[Address]map[transaction.ID]Output),
	}

	for _, output := range outputs {
		if _, addressExists := connector.outputs[output.address]; !addressExists {
			connector.outputs[output.address] = make(map[transaction.ID]Output)
		}

		connector.outputs[output.address][output.transactionID] = output
	}

	return
}

func (connector *mockConnector) UnspentOutputs(addresses ...Address) (outputs map[Address]map[transaction.ID]Output) {
	outputs = make(map[Address]map[transaction.ID]Output)

	for _, addr := range addresses {
		for transactionID, output := range connector.outputs[addr] {
			if !output.inclusionState.Spent {
				if _, outputsExist := outputs[addr]; !outputsExist {
					outputs[addr] = make(map[transaction.ID]Output)
				}

				outputs[addr][transactionID] = output
			}
		}
	}

	return
}
