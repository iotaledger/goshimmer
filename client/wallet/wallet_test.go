package wallet

import (
	"crypto/rand"
	"testing"

	walletaddr "github.com/iotaledger/goshimmer/client/wallet/packages/address"
	walletseed "github.com/iotaledger/goshimmer/client/wallet/packages/seed"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/address"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/balance"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/transaction"
	"github.com/iotaledger/hive.go/bitmask"
	"github.com/stretchr/testify/assert"
)

func TestWallet_SendFunds(t *testing.T) {
	// create test seed
	senderSeed := walletseed.NewSeed()
	receiverSeed := walletseed.NewSeed()

	// define sub-tests by providing a list of parameters and a validator function
	testCases := []struct {
		name       string
		parameters []SendFundsOption
		validator  func(t *testing.T, tx *transaction.Transaction, err error)
	}{
		// test if not providing a destination triggers an error
		{
			name: "missingDestination",
			parameters: []SendFundsOption{
				Remainder(walletaddr.AddressEmpty),
			},
			validator: func(t *testing.T, tx *transaction.Transaction, err error) {
				assert.True(t, tx == nil, "the transaction should be nil")
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
			validator: func(t *testing.T, tx *transaction.Transaction, err error) {
				assert.True(t, tx == nil, "the transaction should be nil")
				assert.Error(t, err, "calling SendFunds without an invalid Destination (amount <= 0) should trigger an error")
				assert.Equal(t, "the amount provided in the destinations needs to be larger than 0", err.Error(), "the error message is wrong")
			},
		},

		// test if a valid transaction can be created
		{
			name: "validTransfer",
			parameters: []SendFundsOption{
				Destination(receiverSeed.Address(0).Address, 1200),
			},
			validator: func(t *testing.T, tx *transaction.Transaction, err error) {
				assert.False(t, tx == nil, "there should be a transaction created")
				assert.Nil(t, err)
			},
		},

		// test if a valid transaction having a colored coin can be created
		{
			name: "validColoredTransfer",
			parameters: []SendFundsOption{
				Destination(receiverSeed.Address(0).Address, 1200, balance.ColorNew),
			},
			validator: func(t *testing.T, tx *transaction.Transaction, err error) {
				assert.False(t, tx == nil, "there should be a transaction created")
				assert.Nil(t, err)
			},
		},
	}

	// execute sub-tests and hand in the results to the validator function
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			// create mocked connector
			mockedConnector := newMockConnector(
				&Output{
					Address:       senderSeed.Address(0).Address,
					TransactionID: transaction.GenesisID,
					Balances: map[balance.Color]uint64{
						balance.ColorIOTA: 1337,
						{3}:               1338,
					},
					InclusionState: InclusionState{
						Liked:     true,
						Confirmed: true,
					},
				},
				&Output{
					Address:       senderSeed.Address(0).Address,
					TransactionID: transaction.ID{3},
					Balances: map[balance.Color]uint64{
						balance.ColorIOTA: 663,
						{4}:               1338,
					},
					InclusionState: InclusionState{
						Liked:     true,
						Confirmed: true,
					},
				},
			)

			// create our test wallet
			wallet := New(
				Import(senderSeed, 1, []bitmask.BitMask{}, NewAssetRegistry()),
				GenericConnector(mockedConnector),
			)

			// validate the result of the function call
			tx, err := wallet.SendFunds(testCase.parameters...)
			testCase.validator(t, tx, err)
		})
	}
}

type mockConnector struct {
	outputs map[address.Address]map[transaction.ID]*Output
}

func (connector *mockConnector) RequestFaucetFunds(addr walletaddr.Address) (err error) {
	// generate random transaction id
	idBytes := make([]byte, transaction.IDLength)
	_, err = rand.Read(idBytes)
	if err != nil {
		return
	}
	transactionID, _, err := transaction.IDFromBytes(idBytes)
	if err != nil {
		return
	}

	newOutput := &Output{
		Address:       addr.Address,
		TransactionID: transactionID,
		Balances: map[balance.Color]uint64{
			balance.ColorIOTA: 1337,
		},
		InclusionState: InclusionState{
			Liked:       true,
			Confirmed:   true,
			Rejected:    false,
			Conflicting: false,
			Spent:       false,
		},
	}

	if _, addressExists := connector.outputs[addr.Address]; !addressExists {
		connector.outputs[addr.Address] = make(map[transaction.ID]*Output)
	}
	connector.outputs[addr.Address][transactionID] = newOutput

	return
}

func (connector *mockConnector) SendTransaction(tx *transaction.Transaction) (err error) {
	// mark outputs as spent
	tx.Inputs().ForEach(func(outputId transaction.OutputID) bool {
		connector.outputs[outputId.Address()][outputId.TransactionID()].InclusionState.Spent = true

		return true
	})

	// create new outputs
	tx.Outputs().ForEach(func(addr address.Address, balances []*balance.Balance) bool {
		// initialize missing address entry
		if _, addressExists := connector.outputs[addr]; !addressExists {
			connector.outputs[addr] = make(map[transaction.ID]*Output)
		}

		// translate balances to mockConnector specific balances
		outputBalances := make(map[balance.Color]uint64)
		for _, coloredBalance := range balances {
			outputBalances[coloredBalance.Color] += uint64(coloredBalance.Value)
		}

		// store new output
		connector.outputs[addr][tx.ID()] = &Output{
			Address:       addr,
			TransactionID: tx.ID(),
			Balances:      outputBalances,
			InclusionState: InclusionState{
				Liked:       true,
				Confirmed:   true,
				Rejected:    false,
				Conflicting: false,
				Spent:       false,
			},
		}

		return true
	})

	return
}

func newMockConnector(outputs ...*Output) (connector *mockConnector) {
	connector = &mockConnector{
		outputs: make(map[address.Address]map[transaction.ID]*Output),
	}

	for _, output := range outputs {
		if _, addressExists := connector.outputs[output.Address]; !addressExists {
			connector.outputs[output.Address] = make(map[transaction.ID]*Output)
		}

		connector.outputs[output.Address][output.TransactionID] = output
	}

	return
}

func (connector *mockConnector) UnspentOutputs(addresses ...walletaddr.Address) (outputs map[walletaddr.Address]map[transaction.ID]*Output, err error) {
	outputs = make(map[walletaddr.Address]map[transaction.ID]*Output)

	for _, addr := range addresses {
		for transactionID, output := range connector.outputs[addr.Address] {
			if !output.InclusionState.Spent {
				if _, outputsExist := outputs[addr]; !outputsExist {
					outputs[addr] = make(map[transaction.ID]*Output)
				}

				outputs[addr][transactionID] = output
			}
		}
	}

	return
}
