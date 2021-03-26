package wallet

import (
	"crypto/rand"
	"testing"

	"github.com/iotaledger/hive.go/bitmask"
	"github.com/iotaledger/hive.go/identity"
	"github.com/mr-tron/base58"
	"github.com/stretchr/testify/assert"

	"github.com/iotaledger/goshimmer/client/wallet/packages/address"
	walletaddr "github.com/iotaledger/goshimmer/client/wallet/packages/address"
	walletseed "github.com/iotaledger/goshimmer/client/wallet/packages/seed"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/mana"
)

func TestWallet_SendFunds(t *testing.T) {
	// create test seed
	senderSeed := walletseed.NewSeed()
	receiverSeed := walletseed.NewSeed()

	// define sub-tests by providing a list of parameters and a validator function
	testCases := []struct {
		name       string
		parameters []SendFundsOption
		validator  func(t *testing.T, tx *ledgerstate.Transaction, err error)
	}{
		// test if not providing a destination triggers an error
		{
			name: "missingDestination",
			parameters: []SendFundsOption{
				Remainder(address.AddressEmpty),
			},
			validator: func(t *testing.T, tx *ledgerstate.Transaction, err error) {
				assert.True(t, tx == nil, "the transaction should be nil")
				assert.Error(t, err, "calling SendFunds without a Destination should trigger an error")
				assert.Equal(t, "you need to provide at least one Destination for a valid transfer to be issued", err.Error(), "the error message is wrong")
			},
		},

		// test if providing an invalid destination (amount <= 0) triggers an error
		{
			name: "zeroAmount",
			parameters: []SendFundsOption{
				Destination(address.AddressEmpty, 1),
				Destination(address.AddressEmpty, 0),
				Destination(address.AddressEmpty, 123),
			},
			validator: func(t *testing.T, tx *ledgerstate.Transaction, err error) {
				assert.True(t, tx == nil, "the transaction should be nil")
				assert.Error(t, err, "calling SendFunds without an invalid Destination (amount <= 0) should trigger an error")
				assert.Equal(t, "the amount provided in the destinations needs to be larger than 0", err.Error(), "the error message is wrong")
			},
		},

		// test if a valid transaction can be created
		{
			name: "validTransfer",
			parameters: []SendFundsOption{
				Destination(receiverSeed.Address(0), 1999),
			},
			validator: func(t *testing.T, tx *ledgerstate.Transaction, err error) {
				assert.False(t, tx == nil, "there should be a transaction created")
				assert.Nil(t, err)
				assert.True(t, ledgerstate.UnlockBlocksValid(ledgerstate.NewOutputs(
					ledgerstate.NewSigLockedColoredOutput(ledgerstate.NewColoredBalances(map[ledgerstate.Color]uint64{
						ledgerstate.ColorIOTA: 1337,
						{3}:                   1338,
					}), senderSeed.Address(0).Address()).SetID(ledgerstate.NewOutputID(ledgerstate.TransactionID{3}, 0)),
					ledgerstate.NewSigLockedColoredOutput(ledgerstate.NewColoredBalances(map[ledgerstate.Color]uint64{
						ledgerstate.ColorIOTA: 663,
						{4}:                   1338,
					}), senderSeed.Address(0).Address()).SetID(ledgerstate.NewOutputID(ledgerstate.TransactionID{3}, 0)),
				), tx))
			},
		},

		// test if a valid transaction having a colored coin can be created
		{
			name: "validColoredTransfer",
			parameters: []SendFundsOption{
				Destination(receiverSeed.Address(0), 1200, ledgerstate.ColorMint),
			},
			validator: func(t *testing.T, tx *ledgerstate.Transaction, err error) {
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
					Address:  senderSeed.Address(0),
					OutputID: ledgerstate.NewOutputID(ledgerstate.GenesisTransactionID, 0),
					Balances: ledgerstate.NewColoredBalances(map[ledgerstate.Color]uint64{
						ledgerstate.ColorIOTA: 1337,
						{3}:                   1338,
					}),
					InclusionState: InclusionState{
						Liked:     true,
						Confirmed: true,
					},
				},
				&Output{
					Address:  senderSeed.Address(0),
					OutputID: ledgerstate.NewOutputID(ledgerstate.TransactionID{3}, 0),
					Balances: ledgerstate.NewColoredBalances(map[ledgerstate.Color]uint64{
						ledgerstate.ColorIOTA: 663,
						{4}:                   1338,
					}),
					InclusionState: InclusionState{
						Liked:     true,
						Confirmed: true,
					},
				},
			)

			// create our test wallet
			wallet := New(
				Import(senderSeed, 2, []bitmask.BitMask{}, NewAssetRegistry()),
				GenericConnector(mockedConnector),
			)

			// validate the result of the function call
			tx, err := wallet.SendFunds(testCase.parameters...)
			testCase.validator(t, tx, err)
		})
	}
}

type mockConnector struct {
	outputs map[address.Address]map[ledgerstate.OutputID]*Output
}

func (connector *mockConnector) GetAllowedPledgeIDs() (pledgeIDMap map[mana.Type][]string, err error) {
	res := map[mana.Type][]string{
		mana.AccessMana:    {base58.Encode(identity.GenerateIdentity().ID().Bytes())},
		mana.ConsensusMana: {base58.Encode(identity.GenerateIdentity().ID().Bytes())},
	}
	return res, nil
}

func (connector *mockConnector) RequestFaucetFunds(addr walletaddr.Address) (err error) {
	// generate random transaction id
	idBytes := make([]byte, ledgerstate.OutputIDLength)
	_, err = rand.Read(idBytes)
	if err != nil {
		return
	}
	outputID, _, err := ledgerstate.OutputIDFromBytes(idBytes)
	if err != nil {
		return
	}

	newOutput := &Output{
		Address:  addr,
		OutputID: outputID,
		Balances: ledgerstate.NewColoredBalances(map[ledgerstate.Color]uint64{
			ledgerstate.ColorIOTA: 1337,
		}),
		InclusionState: InclusionState{
			Liked:       true,
			Confirmed:   true,
			Rejected:    false,
			Conflicting: false,
			Spent:       false,
		},
	}

	if _, addressExists := connector.outputs[addr]; !addressExists {
		connector.outputs[addr] = make(map[ledgerstate.OutputID]*Output)
	}
	connector.outputs[addr][outputID] = newOutput

	return
}

func (connector *mockConnector) SendTransaction(tx *ledgerstate.Transaction) (err error) {
	// mark outputs as spent
	//for _, input := range tx.Essence().Inputs() {
	//if input.Type() == ledgerstate.UTXOInputType {
	//utxoInput := input.(*ledgerstate.UTXOInput)
	//outputID := utxoInput.ReferencedOutputID()
	//connector.outputs[.Address()][outputID.TransactionID()].InclusionState.Spent = true
	//}
	//}

	// create new outputs
	//tx.Outputs().ForEach(func(addr address.Address, balances []*balance.Balance) bool {
	//	// initialize missing address entry
	//	if _, addressExists := connector.outputs[addr]; !addressExists {
	//		connector.outputs[addr] = make(map[transaction.ID]*Output)
	//	}
	//
	//	// translate balances to mockConnector specific balances
	//	outputBalances := make(map[balance.Color]uint64)
	//	for _, coloredBalance := range balances {
	//		outputBalances[coloredBalance.Color] += uint64(coloredBalance.Value)
	//	}
	//
	//	// store new output
	//	connector.outputs[addr][tx.ID()] = &Output{
	//		Address:       addr,
	//		TransactionID: tx.ID(),
	//		Balances:      outputBalances,
	//		InclusionState: InclusionState{
	//			Liked:       true,
	//			Confirmed:   true,
	//			Rejected:    false,
	//			Conflicting: false,
	//			Spent:       false,
	//		},
	//	}
	//
	//	return true
	//})

	return
}

func newMockConnector(outputs ...*Output) (connector *mockConnector) {
	connector = &mockConnector{
		outputs: make(map[address.Address]map[ledgerstate.OutputID]*Output),
	}

	for _, output := range outputs {
		if _, addressExists := connector.outputs[output.Address]; !addressExists {
			connector.outputs[output.Address] = make(map[ledgerstate.OutputID]*Output)
		}

		connector.outputs[output.Address][output.OutputID] = output
	}

	return
}

func (connector *mockConnector) UnspentOutputs(addresses ...walletaddr.Address) (outputs map[address.Address]map[ledgerstate.OutputID]*Output, err error) {
	outputs = make(map[address.Address]map[ledgerstate.OutputID]*Output)
	for _, addr := range addresses {
		for outputID, output := range connector.outputs[addr] {
			if !output.InclusionState.Spent {
				if _, outputsExist := outputs[addr]; !outputsExist {
					outputs[addr] = make(map[ledgerstate.OutputID]*Output)
				}

				outputs[addr][outputID] = output
			}
		}
	}

	return
}
