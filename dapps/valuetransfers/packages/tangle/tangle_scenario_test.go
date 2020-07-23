package tangle

import (
	"testing"

	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/address"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/address/signaturescheme"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/balance"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/branchmanager"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/payload"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/transaction"
	"github.com/iotaledger/hive.go/kvstore/mapdb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const (
	GENESIS uint64 = iota
	A
	B
	C
	D
	E
	F
	G
	H
	I
	J
	Y
)

// TODO: clean up create scenario with some helper functions: DRY!

// preparePropagationScenario1 creates a tangle according to `img/scenario1.png`.
func preparePropagationScenario1(t *testing.T) (*eventTangle, map[string]*transaction.Transaction, map[string]*payload.Payload, map[string]branchmanager.BranchID, *seed) {
	// create tangle
	tangle := newEventTangle(t, New(mapdb.NewMapDB()))

	// create seed for testing
	seed := newSeed()

	// initialize tangle with genesis block (+GENESIS)
	tangle.LoadSnapshot(map[transaction.ID]map[address.Address][]*balance.Balance{
		transaction.GenesisID: {
			seed.Address(GENESIS): {
				balance.New(balance.ColorIOTA, 3333),
			},
		},
	})

	// create dictionaries so we can address the created entities by their aliases from the picture
	transactions := make(map[string]*transaction.Transaction)
	valueObjects := make(map[string]*payload.Payload)
	branches := make(map[string]branchmanager.BranchID)

	// [-GENESIS, A+, B+, C+]
	{
		// create transaction + payload
		transactions["[-GENESIS, A+, B+, C+]"] = transaction.New(
			transaction.NewInputs(
				transaction.NewOutputID(seed.Address(GENESIS), transaction.GenesisID),
			),

			transaction.NewOutputs(map[address.Address][]*balance.Balance{
				seed.Address(A): {
					balance.New(balance.ColorIOTA, 1111),
				},
				seed.Address(B): {
					balance.New(balance.ColorIOTA, 1111),
				},
				seed.Address(C): {
					balance.New(balance.ColorIOTA, 1111),
				},
			}),
		)
		transactions["[-GENESIS, A+, B+, C+]"].Sign(signaturescheme.ED25519(*seed.KeyPair(GENESIS)))
		valueObjects["[-GENESIS, A+, B+, C+]"] = payload.New(payload.GenesisID, payload.GenesisID, transactions["[-GENESIS, A+, B+, C+]"])

		// check if signatures are valid
		assert.True(t, transactions["[-GENESIS, A+, B+, C+]"].SignaturesValid())

		tangle.Expect("PayloadAttached", valueObjects["[-GENESIS, A+, B+, C+]"], mock.Anything)
		tangle.Expect("PayloadSolid", valueObjects["[-GENESIS, A+, B+, C+]"], mock.Anything)
		tangle.Expect("TransactionReceived", transactions["[-GENESIS, A+, B+, C+]"], mock.Anything, mock.Anything)
		tangle.Expect("TransactionSolid", transactions["[-GENESIS, A+, B+, C+]"], mock.Anything)
		tangle.Expect("TransactionBooked", transactions["[-GENESIS, A+, B+, C+]"], mock.Anything, true)

		// attach payload
		tangle.AttachPayloadSync(valueObjects["[-GENESIS, A+, B+, C+]"])

		// check if transaction metadata is found in database
		assert.True(t, tangle.TransactionMetadata(transactions["[-GENESIS, A+, B+, C+]"].ID()).Consume(func(transactionMetadata *TransactionMetadata) {
			assert.True(t, transactionMetadata.Solid(), "the transaction is not solid")
			assert.Equal(t, branchmanager.MasterBranchID, transactionMetadata.BranchID(), "the transaction was booked into the wrong branch")
		}))

		// check if payload metadata is found in database
		assert.True(t, tangle.PayloadMetadata(valueObjects["[-GENESIS, A+, B+, C+]"].ID()).Consume(func(payloadMetadata *PayloadMetadata) {
			assert.True(t, payloadMetadata.IsSolid(), "the payload is not solid")
			assert.Equal(t, branchmanager.MasterBranchID, payloadMetadata.BranchID(), "the payload was booked into the wrong branch")
		}))

		// check if the balance on address GENESIS is found in the database
		assert.True(t, tangle.OutputsOnAddress(seed.Address(GENESIS)).Consume(func(output *Output) {
			assert.Equal(t, 1, output.ConsumerCount(), "the output should be spent")
			assert.Equal(t, []*balance.Balance{balance.New(balance.ColorIOTA, 3333)}, output.Balances())
			assert.Equal(t, branchmanager.MasterBranchID, output.BranchID(), "the output was booked into the wrong branch")
			assert.True(t, output.Solid(), "the output is not solid")
		}))

		// check if the balance on address A is found in the database
		assert.True(t, tangle.OutputsOnAddress(seed.Address(A)).Consume(func(output *Output) {
			assert.Equal(t, 0, output.ConsumerCount(), "the output should not be spent")
			assert.Equal(t, []*balance.Balance{balance.New(balance.ColorIOTA, 1111)}, output.Balances())
			assert.Equal(t, branchmanager.MasterBranchID, output.BranchID(), "the output was booked into the wrong branch")
			assert.True(t, output.Solid(), "the output is not solid")
		}))

		// check if the balance on address B is found in the database
		assert.True(t, tangle.OutputsOnAddress(seed.Address(B)).Consume(func(output *Output) {
			assert.Equal(t, 0, output.ConsumerCount(), "the output should not be spent")
			assert.Equal(t, []*balance.Balance{balance.New(balance.ColorIOTA, 1111)}, output.Balances())
			assert.Equal(t, branchmanager.MasterBranchID, output.BranchID(), "the output was booked into the wrong branch")
			assert.True(t, output.Solid(), "the output is not solid")
		}))

		// check if the balance on address C is found in the database
		assert.True(t, tangle.OutputsOnAddress(seed.Address(C)).Consume(func(output *Output) {
			assert.Equal(t, 0, output.ConsumerCount(), "the output should not be spent")
			assert.Equal(t, []*balance.Balance{balance.New(balance.ColorIOTA, 1111)}, output.Balances())
			assert.Equal(t, branchmanager.MasterBranchID, output.BranchID(), "the output was booked into the wrong branch")
			assert.True(t, output.Solid(), "the output is not solid")
		}))
	}

	// [-A, D+]
	{
		// create transaction + payload
		transactions["[-A, D+]"] = transaction.New(
			transaction.NewInputs(
				transaction.NewOutputID(seed.Address(A), transactions["[-GENESIS, A+, B+, C+]"].ID()),
			),

			transaction.NewOutputs(map[address.Address][]*balance.Balance{
				seed.Address(D): {
					balance.New(balance.ColorIOTA, 1111),
				},
			}),
		)
		transactions["[-A, D+]"].Sign(signaturescheme.ED25519(*seed.KeyPair(A)))
		valueObjects["[-A, D+]"] = payload.New(payload.GenesisID, valueObjects["[-GENESIS, A+, B+, C+]"].ID(), transactions["[-A, D+]"])

		// check if signatures are valid
		assert.True(t, transactions["[-A, D+]"].SignaturesValid())

		tangle.Expect("PayloadAttached", valueObjects["[-A, D+]"], mock.Anything)
		tangle.Expect("PayloadSolid", valueObjects["[-A, D+]"], mock.Anything)
		tangle.Expect("TransactionReceived", transactions["[-A, D+]"], mock.Anything, mock.Anything)
		tangle.Expect("TransactionSolid", transactions["[-A, D+]"], mock.Anything)
		tangle.Expect("TransactionBooked", transactions["[-A, D+]"], mock.Anything, true)

		// attach payload
		tangle.AttachPayloadSync(valueObjects["[-A, D+]"])

		// check if payload metadata is found in database
		assert.True(t, tangle.TransactionMetadata(transactions["[-A, D+]"].ID()).Consume(func(transactionMetadata *TransactionMetadata) {
			assert.True(t, transactionMetadata.Solid(), "the transaction is not solid")
			assert.Equal(t, branchmanager.MasterBranchID, transactionMetadata.BranchID(), "the transaction was booked into the wrong branch")
		}))

		// check if transaction metadata is found in database
		assert.True(t, tangle.PayloadMetadata(valueObjects["[-A, D+]"].ID()).Consume(func(payloadMetadata *PayloadMetadata) {
			assert.True(t, payloadMetadata.IsSolid(), "the payload is not solid")
			assert.Equal(t, branchmanager.MasterBranchID, payloadMetadata.BranchID(), "the payload was booked into the wrong branch")
		}))

		// check if the balance on address A is found in the database
		assert.True(t, tangle.OutputsOnAddress(seed.Address(A)).Consume(func(output *Output) {
			assert.Equal(t, 1, output.ConsumerCount(), "the output should be spent")
			assert.Equal(t, []*balance.Balance{balance.New(balance.ColorIOTA, 1111)}, output.Balances())
			assert.Equal(t, branchmanager.MasterBranchID, output.BranchID(), "the output was booked into the wrong branch")
			assert.True(t, output.Solid(), "the output is not solid")
		}))

		// check if the balance on address D is found in the database
		assert.True(t, tangle.OutputsOnAddress(seed.Address(D)).Consume(func(output *Output) {
			assert.Equal(t, 0, output.ConsumerCount(), "the output should not be spent")
			assert.Equal(t, []*balance.Balance{balance.New(balance.ColorIOTA, 1111)}, output.Balances())
			assert.Equal(t, branchmanager.MasterBranchID, output.BranchID(), "the output was booked into the wrong branch")
			assert.True(t, output.Solid(), "the output is not solid")
		}))
	}

	// [-B, -C, E+]
	{
		// create transaction + payload
		transactions["[-B, -C, E+]"] = transaction.New(
			transaction.NewInputs(
				transaction.NewOutputID(seed.Address(B), transactions["[-GENESIS, A+, B+, C+]"].ID()),
				transaction.NewOutputID(seed.Address(C), transactions["[-GENESIS, A+, B+, C+]"].ID()),
			),

			transaction.NewOutputs(map[address.Address][]*balance.Balance{
				seed.Address(E): {
					balance.New(balance.ColorIOTA, 2222),
				},
			}),
		)
		transactions["[-B, -C, E+]"].Sign(signaturescheme.ED25519(*seed.KeyPair(B)))
		transactions["[-B, -C, E+]"].Sign(signaturescheme.ED25519(*seed.KeyPair(C)))
		valueObjects["[-B, -C, E+]"] = payload.New(payload.GenesisID, valueObjects["[-GENESIS, A+, B+, C+]"].ID(), transactions["[-B, -C, E+]"])

		// check if signatures are valid
		assert.True(t, transactions["[-B, -C, E+]"].SignaturesValid())

		tangle.Expect("PayloadAttached", valueObjects["[-B, -C, E+]"], mock.Anything)
		tangle.Expect("PayloadSolid", valueObjects["[-B, -C, E+]"], mock.Anything)
		tangle.Expect("TransactionReceived", transactions["[-B, -C, E+]"], mock.Anything, mock.Anything)
		tangle.Expect("TransactionSolid", transactions["[-B, -C, E+]"], mock.Anything)
		tangle.Expect("TransactionBooked", transactions["[-B, -C, E+]"], mock.Anything, true)

		// attach payload
		tangle.AttachPayloadSync(valueObjects["[-B, -C, E+]"])

		// check if payload metadata is found in database
		assert.True(t, tangle.TransactionMetadata(transactions["[-B, -C, E+]"].ID()).Consume(func(transactionMetadata *TransactionMetadata) {
			assert.True(t, transactionMetadata.Solid(), "the transaction is not solid")
			assert.Equal(t, branchmanager.MasterBranchID, transactionMetadata.BranchID(), "the transaction was booked into the wrong branch")
		}))

		// check if transaction metadata is found in database
		assert.True(t, tangle.PayloadMetadata(valueObjects["[-B, -C, E+]"].ID()).Consume(func(payloadMetadata *PayloadMetadata) {
			assert.True(t, payloadMetadata.IsSolid(), "the payload is not solid")
			assert.Equal(t, branchmanager.MasterBranchID, payloadMetadata.BranchID(), "the payload was booked into the wrong branch")
		}))

		// check if the balance on address B is found in the database
		assert.True(t, tangle.OutputsOnAddress(seed.Address(B)).Consume(func(output *Output) {
			assert.Equal(t, 1, output.ConsumerCount(), "the output should be spent")
			assert.Equal(t, []*balance.Balance{balance.New(balance.ColorIOTA, 1111)}, output.Balances())
			assert.Equal(t, branchmanager.MasterBranchID, output.BranchID(), "the output was booked into the wrong branch")
			assert.True(t, output.Solid(), "the output is not solid")
		}))

		// check if the balance on address C is found in the database
		assert.True(t, tangle.OutputsOnAddress(seed.Address(C)).Consume(func(output *Output) {
			assert.Equal(t, 1, output.ConsumerCount(), "the output should be spent")
			assert.Equal(t, []*balance.Balance{balance.New(balance.ColorIOTA, 1111)}, output.Balances())
			assert.Equal(t, branchmanager.MasterBranchID, output.BranchID(), "the output was booked into the wrong branch")
			assert.True(t, output.Solid(), "the output is not solid")
		}))

		// check if the balance on address E is found in the database
		assert.True(t, tangle.OutputsOnAddress(seed.Address(E)).Consume(func(output *Output) {
			assert.Equal(t, 0, output.ConsumerCount(), "the output should not be spent")
			assert.Equal(t, []*balance.Balance{balance.New(balance.ColorIOTA, 2222)}, output.Balances())
			assert.Equal(t, branchmanager.MasterBranchID, output.BranchID(), "the output was booked into the wrong branch")
			assert.True(t, output.Solid(), "the output is not solid")
		}))
	}

	// [-B, -C, E+] (Reattachment)
	{
		// create payload
		valueObjects["[-B, -C, E+] (Reattachment)"] = payload.New(valueObjects["[-B, -C, E+]"].ID(), valueObjects["[-GENESIS, A+, B+, C+]"].ID(), transactions["[-B, -C, E+]"])

		tangle.Expect("PayloadAttached", valueObjects["[-B, -C, E+] (Reattachment)"], mock.Anything)
		tangle.Expect("PayloadSolid", valueObjects["[-B, -C, E+] (Reattachment)"], mock.Anything)

		// attach payload
		tangle.AttachPayloadSync(valueObjects["[-B, -C, E+] (Reattachment)"])

		// check if transaction metadata is found in database
		assert.True(t, tangle.TransactionMetadata(transactions["[-B, -C, E+]"].ID()).Consume(func(transactionMetadata *TransactionMetadata) {
			assert.True(t, transactionMetadata.Solid(), "the transaction is not solid")
			assert.Equal(t, branchmanager.MasterBranchID, transactionMetadata.BranchID(), "the transaction was booked into the wrong branch")
		}))

		// check if payload metadata is found in database
		assert.True(t, tangle.PayloadMetadata(valueObjects["[-B, -C, E+] (Reattachment)"].ID()).Consume(func(payloadMetadata *PayloadMetadata) {
			assert.True(t, payloadMetadata.IsSolid(), "the payload is not solid")
			assert.Equal(t, branchmanager.MasterBranchID, payloadMetadata.BranchID(), "the payload was booked into the wrong branch")
		}))

		// check if the balance on address B is found in the database
		assert.True(t, tangle.OutputsOnAddress(seed.Address(B)).Consume(func(output *Output) {
			assert.Equal(t, 1, output.ConsumerCount(), "the output should be spent")
			assert.Equal(t, []*balance.Balance{balance.New(balance.ColorIOTA, 1111)}, output.Balances())
			assert.Equal(t, branchmanager.MasterBranchID, output.BranchID(), "the output was booked into the wrong branch")
			assert.True(t, output.Solid(), "the output is not solid")
		}))

		// check if the balance on address C is found in the database
		assert.True(t, tangle.OutputsOnAddress(seed.Address(C)).Consume(func(output *Output) {
			assert.Equal(t, 1, output.ConsumerCount(), "the output should be spent")
			assert.Equal(t, []*balance.Balance{balance.New(balance.ColorIOTA, 1111)}, output.Balances())
			assert.Equal(t, branchmanager.MasterBranchID, output.BranchID(), "the output was booked into the wrong branch")
			assert.True(t, output.Solid(), "the output is not solid")
		}))

		// check if the balance on address E is found in the database
		assert.True(t, tangle.OutputsOnAddress(seed.Address(E)).Consume(func(output *Output) {
			assert.Equal(t, 0, output.ConsumerCount(), "the output should not be spent")
			assert.Equal(t, []*balance.Balance{balance.New(balance.ColorIOTA, 2222)}, output.Balances())
			assert.Equal(t, branchmanager.MasterBranchID, output.BranchID(), "the output was booked into the wrong branch")
			assert.True(t, output.Solid(), "the output is not solid")
		}))
	}

	// [-A, F+]
	{
		// create transaction + payload
		outputA := transaction.NewOutputID(seed.Address(A), transactions["[-GENESIS, A+, B+, C+]"].ID())
		transactions["[-A, F+]"] = transaction.New(
			transaction.NewInputs(
				outputA,
			),

			transaction.NewOutputs(map[address.Address][]*balance.Balance{
				seed.Address(F): {
					balance.New(balance.ColorIOTA, 1111),
				},
			}),
		)
		transactions["[-A, F+]"].Sign(signaturescheme.ED25519(*seed.KeyPair(A)))
		valueObjects["[-A, F+]"] = payload.New(valueObjects["[-B, -C, E+]"].ID(), valueObjects["[-GENESIS, A+, B+, C+]"].ID(), transactions["[-A, F+]"])

		// check if signatures are valid
		assert.True(t, transactions["[-A, F+]"].SignaturesValid())

		tangle.Expect("PayloadAttached", valueObjects["[-A, F+]"], mock.Anything)
		tangle.Expect("PayloadSolid", valueObjects["[-A, F+]"], mock.Anything)
		tangle.Expect("TransactionReceived", transactions["[-A, F+]"], mock.Anything, mock.Anything)
		tangle.Expect("TransactionSolid", transactions["[-A, F+]"], mock.Anything)
		tangle.Expect("TransactionBooked", transactions["[-A, F+]"], mock.Anything, true)
		tangle.Expect("Fork", transactions["[-A, D+]"], mock.Anything, mock.Anything, []transaction.OutputID{outputA})

		// attach payload
		tangle.AttachPayloadSync(valueObjects["[-A, F+]"])

		// create aliases for the branches
		branches["A"] = branchmanager.NewBranchID(transactions["[-A, D+]"].ID())
		branches["B"] = branchmanager.NewBranchID(transactions["[-A, F+]"].ID())

		// check if payload metadata is found in database
		assert.True(t, tangle.TransactionMetadata(transactions["[-A, F+]"].ID()).Consume(func(transactionMetadata *TransactionMetadata) {
			assert.True(t, transactionMetadata.Solid(), "the transaction is not solid")
			assert.Equal(t, branches["B"], transactionMetadata.BranchID(), "the transaction was booked into the wrong branch")
		}))

		// check if transaction metadata is found in database
		assert.True(t, tangle.PayloadMetadata(valueObjects["[-A, F+]"].ID()).Consume(func(payloadMetadata *PayloadMetadata) {
			assert.True(t, payloadMetadata.IsSolid(), "the payload is not solid")
			assert.Equal(t, branches["B"], payloadMetadata.BranchID(), "the payload was booked into the wrong branch")
		}))

		// check if the balance on address A is found in the database
		assert.True(t, tangle.OutputsOnAddress(seed.Address(A)).Consume(func(output *Output) {
			assert.Equal(t, 2, output.ConsumerCount(), "the output should be spent")
			assert.Equal(t, []*balance.Balance{balance.New(balance.ColorIOTA, 1111)}, output.Balances())
			assert.Equal(t, branchmanager.MasterBranchID, output.BranchID(), "the output was booked into the wrong branch")
			assert.True(t, output.Solid(), "the output is not solid")
		}))

		// check if the balance on address F is found in the database
		assert.True(t, tangle.OutputsOnAddress(seed.Address(F)).Consume(func(output *Output) {
			assert.Equal(t, 0, output.ConsumerCount(), "the output should not be spent")
			assert.Equal(t, []*balance.Balance{balance.New(balance.ColorIOTA, 1111)}, output.Balances())
			assert.Equal(t, branches["B"], output.BranchID(), "the output was booked into the wrong branch")
			assert.True(t, output.Solid(), "the output is not solid")
		}))

		// check if the balance on address D is found in the database
		assert.True(t, tangle.OutputsOnAddress(seed.Address(D)).Consume(func(output *Output) {
			assert.Equal(t, 0, output.ConsumerCount(), "the output should be spent")
			assert.Equal(t, []*balance.Balance{balance.New(balance.ColorIOTA, 1111)}, output.Balances())
			assert.Equal(t, branches["A"], output.BranchID(), "the output was booked into the wrong branch")
			assert.True(t, output.Solid(), "the output is not solid")
		}))

		// check if transaction metadata is found in database
		assert.True(t, tangle.PayloadMetadata(valueObjects["[-A, D+]"].ID()).Consume(func(payloadMetadata *PayloadMetadata) {
			assert.True(t, payloadMetadata.IsSolid(), "the payload is not solid")
			assert.Equal(t, branches["A"], payloadMetadata.BranchID(), "the payload was booked into the wrong branch")
		}))

		// check if the branches are conflicting
		branchesConflicting, err := tangle.branchManager.BranchesConflicting(branches["A"], branches["B"])
		require.NoError(t, err)
		assert.True(t, branchesConflicting, "the branches should be conflicting")
	}

	// [-E, -F, G+]
	{
		// create transaction + payload
		transactions["[-E, -F, G+]"] = transaction.New(
			transaction.NewInputs(
				transaction.NewOutputID(seed.Address(E), transactions["[-B, -C, E+]"].ID()),
				transaction.NewOutputID(seed.Address(F), transactions["[-A, F+]"].ID()),
			),

			transaction.NewOutputs(map[address.Address][]*balance.Balance{
				seed.Address(G): {
					balance.New(balance.ColorIOTA, 3333),
				},
			}),
		)
		transactions["[-E, -F, G+]"].Sign(signaturescheme.ED25519(*seed.KeyPair(E)))
		transactions["[-E, -F, G+]"].Sign(signaturescheme.ED25519(*seed.KeyPair(F)))
		valueObjects["[-E, -F, G+]"] = payload.New(valueObjects["[-B, -C, E+]"].ID(), valueObjects["[-A, F+]"].ID(), transactions["[-E, -F, G+]"])

		// check if signatures are valid
		assert.True(t, transactions["[-E, -F, G+]"].SignaturesValid())

		tangle.Expect("PayloadAttached", valueObjects["[-E, -F, G+]"], mock.Anything)
		tangle.Expect("PayloadSolid", valueObjects["[-E, -F, G+]"], mock.Anything)
		tangle.Expect("TransactionReceived", transactions["[-E, -F, G+]"], mock.Anything, mock.Anything)
		tangle.Expect("TransactionSolid", transactions["[-E, -F, G+]"], mock.Anything)
		tangle.Expect("TransactionBooked", transactions["[-E, -F, G+]"], mock.Anything, true)

		// attach payload
		tangle.AttachPayloadSync(valueObjects["[-E, -F, G+]"])

		// check if payload metadata is found in database
		assert.True(t, tangle.TransactionMetadata(transactions["[-E, -F, G+]"].ID()).Consume(func(transactionMetadata *TransactionMetadata) {
			assert.True(t, transactionMetadata.Solid(), "the transaction is not solid")
			assert.Equal(t, branches["B"], transactionMetadata.BranchID(), "the transaction was booked into the wrong branch")
		}))

		// check if transaction metadata is found in database
		assert.True(t, tangle.PayloadMetadata(valueObjects["[-E, -F, G+]"].ID()).Consume(func(payloadMetadata *PayloadMetadata) {
			assert.True(t, payloadMetadata.IsSolid(), "the payload is not solid")
			assert.Equal(t, branches["B"], payloadMetadata.BranchID(), "the payload was booked into the wrong branch")
		}))

		// check if the balance on address E is found in the database
		assert.True(t, tangle.OutputsOnAddress(seed.Address(E)).Consume(func(output *Output) {
			assert.Equal(t, 1, output.ConsumerCount(), "the output should be spent")
			assert.Equal(t, []*balance.Balance{balance.New(balance.ColorIOTA, 2222)}, output.Balances())
			assert.Equal(t, branchmanager.MasterBranchID, output.BranchID(), "the output was booked into the wrong branch")
			assert.True(t, output.Solid(), "the output is not solid")
		}))

		// check if the balance on address F is found in the database
		assert.True(t, tangle.OutputsOnAddress(seed.Address(F)).Consume(func(output *Output) {
			assert.Equal(t, 1, output.ConsumerCount(), "the output should be spent")
			assert.Equal(t, []*balance.Balance{balance.New(balance.ColorIOTA, 1111)}, output.Balances())
			assert.Equal(t, branches["B"], output.BranchID(), "the output was booked into the wrong branch")
			assert.True(t, output.Solid(), "the output is not solid")
		}))

		// check if the balance on address G is found in the database
		assert.True(t, tangle.OutputsOnAddress(seed.Address(G)).Consume(func(output *Output) {
			assert.Equal(t, 0, output.ConsumerCount(), "the output should be spent")
			assert.Equal(t, []*balance.Balance{balance.New(balance.ColorIOTA, 3333)}, output.Balances())
			assert.Equal(t, branches["B"], output.BranchID(), "the output was booked into the wrong branch")
			assert.True(t, output.Solid(), "the output is not solid")
		}))
	}

	// [-F, -D, Y+]
	{
		// create transaction + payload
		transactions["[-F, -D, Y+]"] = transaction.New(
			transaction.NewInputs(
				transaction.NewOutputID(seed.Address(D), transactions["[-A, D+]"].ID()),
				transaction.NewOutputID(seed.Address(F), transactions["[-A, F+]"].ID()),
			),

			transaction.NewOutputs(map[address.Address][]*balance.Balance{
				seed.Address(Y): {
					balance.New(balance.ColorIOTA, 2222),
				},
			}),
		)
		transactions["[-F, -D, Y+]"].Sign(signaturescheme.ED25519(*seed.KeyPair(D)))
		transactions["[-F, -D, Y+]"].Sign(signaturescheme.ED25519(*seed.KeyPair(F)))
		valueObjects["[-F, -D, Y+]"] = payload.New(valueObjects["[-A, F+]"].ID(), valueObjects["[-A, D+]"].ID(), transactions["[-F, -D, Y+]"])

		// check if signatures are valid
		assert.True(t, transactions["[-F, -D, Y+]"].SignaturesValid())

		tangle.Expect("PayloadAttached", valueObjects["[-F, -D, Y+]"], mock.Anything)
		tangle.Expect("PayloadInvalid", valueObjects["[-F, -D, Y+]"], mock.Anything, mock.MatchedBy(func(err error) bool { return assert.Error(t, err) }))
		tangle.Expect("TransactionReceived", transactions["[-F, -D, Y+]"], mock.Anything, mock.Anything)
		tangle.Expect("TransactionInvalid", transactions["[-F, -D, Y+]"], mock.Anything, mock.MatchedBy(func(err error) bool { return assert.Error(t, err) }))

		// attach payload
		tangle.AttachPayloadSync(valueObjects["[-F, -D, Y+]"])

		// check if all of the invalids transactions models were deleted
		assert.False(t, tangle.Transaction(transactions["[-F, -D, Y+]"].ID()).Consume(func(metadata *transaction.Transaction) {}), "the transaction should not be found")
		assert.False(t, tangle.TransactionMetadata(transactions["[-F, -D, Y+]"].ID()).Consume(func(metadata *TransactionMetadata) {}), "the transaction metadata should not be found")
		assert.False(t, tangle.Payload(valueObjects["[-F, -D, Y+]"].ID()).Consume(func(payload *payload.Payload) {}), "the payload should not be found")
		assert.False(t, tangle.PayloadMetadata(valueObjects["[-F, -D, Y+]"].ID()).Consume(func(payloadMetadata *PayloadMetadata) {}), "the payload metadata should not be found")
		assert.True(t, tangle.Approvers(valueObjects["[-A, F+]"].ID()).Consume(func(approver *PayloadApprover) {
			assert.NotEqual(t, approver.ApprovingPayloadID(), valueObjects["[-F, -D, Y+]"].ID(), "the invalid value object should not show up as an approver")
		}), "the should be approvers of the referenced output")
		assert.False(t, tangle.Approvers(valueObjects["[-A, D+]"].ID()).Consume(func(approver *PayloadApprover) {}), "approvers should be empty")
		assert.False(t, tangle.Attachments(transactions["[-F, -D, Y+]"].ID()).Consume(func(attachment *Attachment) {}), "the transaction should not have any attachments")
		assert.False(t, tangle.Consumers(transaction.NewOutputID(seed.Address(D), transactions["[-A, D+]"].ID())).Consume(func(consumer *Consumer) {}), "the consumers of the used input should be empty")
		assert.True(t, tangle.Consumers(transaction.NewOutputID(seed.Address(F), transactions["[-A, F+]"].ID())).Consume(func(consumer *Consumer) {
			assert.NotEqual(t, consumer.TransactionID(), transactions["[-F, -D, Y+]"].ID(), "the consumers should not contain the invalid transaction")
		}), "the consumers should not be empty")
	}

	// [-B, -C, E+] (2nd Reattachment)
	{
		valueObjects["[-B, -C, E+] (2nd Reattachment)"] = payload.New(valueObjects["[-A, F+]"].ID(), valueObjects["[-A, D+]"].ID(), transactions["[-B, -C, E+]"])

		tangle.Expect("PayloadAttached", valueObjects["[-B, -C, E+] (2nd Reattachment)"], mock.Anything)
		tangle.Expect("PayloadInvalid", valueObjects["[-B, -C, E+] (2nd Reattachment)"], mock.Anything, mock.MatchedBy(func(err error) bool { return assert.Error(t, err) }))

		// attach payload
		tangle.AttachPayloadSync(valueObjects["[-B, -C, E+] (2nd Reattachment)"])

		// check if all of the valid transactions models were NOT deleted
		assert.True(t, tangle.Transaction(transactions["[-B, -C, E+]"].ID()).Consume(func(metadata *transaction.Transaction) {}))

		// check if transaction metadata is found in database
		assert.True(t, tangle.TransactionMetadata(transactions["[-B, -C, E+]"].ID()).Consume(func(transactionMetadata *TransactionMetadata) {
			assert.True(t, transactionMetadata.Solid(), "the transaction is not solid")
			assert.Equal(t, branchmanager.MasterBranchID, transactionMetadata.BranchID(), "the transaction was booked into the wrong branch")
		}))

		// check if payload and its corresponding models are not found in the database (payload was invalid)
		assert.False(t, tangle.Payload(valueObjects["[-B, -C, E+] (2nd Reattachment)"].ID()).Consume(func(payload *payload.Payload) {}), "the payload should not exist")
		assert.False(t, tangle.PayloadMetadata(valueObjects["[-B, -C, E+] (2nd Reattachment)"].ID()).Consume(func(payloadMetadata *PayloadMetadata) {}), "the payload metadata should not exist")
		assert.True(t, tangle.Attachments(transactions["[-B, -C, E+]"].ID()).Consume(func(attachment *Attachment) {
			assert.NotEqual(t, valueObjects["[-B, -C, E+] (2nd Reattachment)"].ID(), attachment.PayloadID(), "the attachment to the payload should be deleted")
		}), "there should be attachments of the transaction")
		assert.True(t, tangle.Approvers(valueObjects["[-A, F+]"].ID()).Consume(func(approver *PayloadApprover) {
			assert.NotEqual(t, valueObjects["[-A, F+]"].ID(), approver.ApprovingPayloadID(), "there should not be an approver reference to the invalid payload")
			assert.NotEqual(t, valueObjects["[-A, D+]"].ID(), approver.ApprovingPayloadID(), "there should not be an approver reference to the invalid payload")
		}), "there should be approvers")
		assert.False(t, tangle.Approvers(valueObjects["[-A, D+]"].ID()).Consume(func(approver *PayloadApprover) {}), "there should be no approvers")
	}

	return tangle, transactions, valueObjects, branches, seed
}

// preparePropagationScenario1 creates a tangle according to `img/scenario2.png`.
func preparePropagationScenario2(t *testing.T) (*eventTangle, map[string]*transaction.Transaction, map[string]*payload.Payload, map[string]branchmanager.BranchID, *seed) {
	tangle, transactions, valueObjects, branches, seed := preparePropagationScenario1(t)

	// [-C, H+]
	{
		// create transaction + payload
		outputC := transaction.NewOutputID(seed.Address(C), transactions["[-GENESIS, A+, B+, C+]"].ID())
		transactions["[-C, H+]"] = transaction.New(
			transaction.NewInputs(
				outputC,
			),

			transaction.NewOutputs(map[address.Address][]*balance.Balance{
				seed.Address(H): {
					balance.New(balance.ColorIOTA, 1111),
				},
			}),
		)
		transactions["[-C, H+]"].Sign(signaturescheme.ED25519(*seed.KeyPair(C)))
		valueObjects["[-C, H+]"] = payload.New(valueObjects["[-GENESIS, A+, B+, C+]"].ID(), valueObjects["[-A, D+]"].ID(), transactions["[-C, H+]"])

		// check if signatures are valid
		assert.True(t, transactions["[-C, H+]"].SignaturesValid())

		tangle.Expect("PayloadAttached", valueObjects["[-C, H+]"], mock.Anything)
		tangle.Expect("PayloadSolid", valueObjects["[-C, H+]"], mock.Anything)
		tangle.Expect("TransactionReceived", transactions["[-C, H+]"], mock.Anything, mock.Anything)
		tangle.Expect("TransactionSolid", transactions["[-C, H+]"], mock.Anything)
		tangle.Expect("TransactionBooked", transactions["[-C, H+]"], mock.Anything, true)
		tangle.Expect("Fork", transactions["[-B, -C, E+]"], mock.Anything, mock.Anything, []transaction.OutputID{outputC})

		// attach payload
		tangle.AttachPayloadSync(valueObjects["[-C, H+]"])

		// create alias for the branch
		branches["C"] = branchmanager.NewBranchID(transactions["[-C, H+]"].ID())
		branches["AC"] = tangle.BranchManager().GenerateAggregatedBranchID(branches["A"], branches["C"])

		// check if transaction metadata is found in database
		assert.True(t, tangle.TransactionMetadata(transactions["[-C, H+]"].ID()).Consume(func(transactionMetadata *TransactionMetadata) {
			assert.True(t, transactionMetadata.Solid(), "the transaction is not solid")
			assert.Equal(t, branches["C"], transactionMetadata.BranchID(), "the transaction was booked into the wrong branch")
		}))

		// check if payload metadata is found in database
		assert.True(t, tangle.PayloadMetadata(valueObjects["[-C, H+]"].ID()).Consume(func(payloadMetadata *PayloadMetadata) {
			assert.True(t, payloadMetadata.IsSolid(), "the payload is not solid")
			assert.NotEqual(t, branches["C"], payloadMetadata.BranchID(), "the payload was booked into the wrong branch")
			assert.Equal(t, branches["AC"], payloadMetadata.BranchID(), "the payload was booked into the wrong branch")
		}))

		// check if the balance on address C is found in the database
		assert.True(t, tangle.OutputsOnAddress(seed.Address(C)).Consume(func(output *Output) {
			assert.Equal(t, 2, output.ConsumerCount(), "the output should be spent")
			assert.Equal(t, []*balance.Balance{balance.New(balance.ColorIOTA, 1111)}, output.Balances())
			assert.Equal(t, branchmanager.MasterBranchID, output.BranchID(), "the output was booked into the wrong branch")
			assert.True(t, output.Solid(), "the output is not solid")
		}))

		// check if the balance on address H is found in the database
		assert.True(t, tangle.OutputsOnAddress(seed.Address(H)).Consume(func(output *Output) {
			assert.Equal(t, 0, output.ConsumerCount(), "the output should not be spent")
			assert.Equal(t, []*balance.Balance{balance.New(balance.ColorIOTA, 1111)}, output.Balances())
			assert.Equal(t, branches["C"], output.BranchID(), "the output was booked into the wrong branch")
			assert.True(t, output.Solid(), "the output is not solid")
		}))

		// Branch D

		// create alias for the branch
		branches["D"] = branchmanager.NewBranchID(transactions["[-B, -C, E+]"].ID())
		branches["BD"] = tangle.branchManager.GenerateAggregatedBranchID(branches["B"], branches["D"])

		{
			// check if transaction metadata is found in database
			assert.True(t, tangle.PayloadMetadata(valueObjects["[-B, -C, E+]"].ID()).Consume(func(payloadMetadata *PayloadMetadata) {
				assert.True(t, payloadMetadata.IsSolid(), "the payload is not solid")
				assert.Equal(t, branches["D"], payloadMetadata.BranchID(), "the payload was booked into the wrong branch")
			}))

			// check if transaction metadata is found in database
			assert.True(t, tangle.PayloadMetadata(valueObjects["[-B, -C, E+] (Reattachment)"].ID()).Consume(func(payloadMetadata *PayloadMetadata) {
				assert.True(t, payloadMetadata.IsSolid(), "the payload is not solid")
				assert.Equal(t, branches["D"], payloadMetadata.BranchID(), "the payload was booked into the wrong branch")
			}))
		}

		// check if the branches C and D are conflicting
		branchesConflicting, err := tangle.branchManager.BranchesConflicting(branches["C"], branches["D"])
		require.NoError(t, err)
		assert.True(t, branchesConflicting, "the branches should be conflicting")

		// Aggregated Branch [BD]
		{
			// check if transaction metadata is found in database
			assert.True(t, tangle.PayloadMetadata(valueObjects["[-E, -F, G+]"].ID()).Consume(func(payloadMetadata *PayloadMetadata) {
				assert.True(t, payloadMetadata.IsSolid(), "the payload is not solid")
				assert.Equal(t, branches["BD"], payloadMetadata.BranchID(), "the payload was booked into the wrong branch")
			}))

			// check if transaction metadata is found in database
			assert.True(t, tangle.PayloadMetadata(valueObjects["[-E, -F, G+]"].ID()).Consume(func(payloadMetadata *PayloadMetadata) {
				assert.True(t, payloadMetadata.IsSolid(), "the payload is not solid")
				assert.Equal(t, branches["BD"], payloadMetadata.BranchID(), "the payload was booked into the wrong branch")
			}))
		}
	}

	// [-H, -D, I+]
	{
		// create transaction + payload
		transactions["[-H, -D, I+]"] = transaction.New(
			transaction.NewInputs(
				transaction.NewOutputID(seed.Address(H), transactions["[-C, H+]"].ID()),
				transaction.NewOutputID(seed.Address(D), transactions["[-A, D+]"].ID()),
			),

			transaction.NewOutputs(map[address.Address][]*balance.Balance{
				seed.Address(I): {
					balance.New(balance.ColorIOTA, 2222),
				},
			}),
		)
		transactions["[-H, -D, I+]"].Sign(signaturescheme.ED25519(*seed.KeyPair(H)))
		transactions["[-H, -D, I+]"].Sign(signaturescheme.ED25519(*seed.KeyPair(D)))
		valueObjects["[-H, -D, I+]"] = payload.New(valueObjects["[-C, H+]"].ID(), valueObjects["[-A, D+]"].ID(), transactions["[-H, -D, I+]"])

		// check if signatures are valid
		assert.True(t, transactions["[-H, -D, I+]"].SignaturesValid())

		tangle.Expect("PayloadAttached", valueObjects["[-H, -D, I+]"], mock.Anything)
		tangle.Expect("PayloadSolid", valueObjects["[-H, -D, I+]"], mock.Anything)
		tangle.Expect("TransactionReceived", transactions["[-H, -D, I+]"], mock.Anything, mock.Anything)
		tangle.Expect("TransactionSolid", transactions["[-H, -D, I+]"], mock.Anything)
		tangle.Expect("TransactionBooked", transactions["[-H, -D, I+]"], mock.Anything, true)

		// attach payload
		tangle.AttachPayloadSync(valueObjects["[-H, -D, I+]"])

		// create alias for the branch
		branches["AC"] = tangle.branchManager.GenerateAggregatedBranchID(branches["A"], branches["C"])

		// check if transaction metadata is found in database
		assert.True(t, tangle.TransactionMetadata(transactions["[-H, -D, I+]"].ID()).Consume(func(transactionMetadata *TransactionMetadata) {
			assert.True(t, transactionMetadata.Solid(), "the transaction is not solid")
			assert.Equal(t, branches["AC"], transactionMetadata.BranchID(), "the transaction was booked into the wrong branch")
		}))

		// check if payload metadata is found in database
		assert.True(t, tangle.PayloadMetadata(valueObjects["[-H, -D, I+]"].ID()).Consume(func(payloadMetadata *PayloadMetadata) {
			assert.True(t, payloadMetadata.IsSolid(), "the payload is not solid")
			assert.Equal(t, branches["AC"], payloadMetadata.BranchID(), "the payload was booked into the wrong branch")
		}))

		// check if the balance on address H is found in the database
		assert.True(t, tangle.OutputsOnAddress(seed.Address(H)).Consume(func(output *Output) {
			assert.Equal(t, 1, output.ConsumerCount(), "the output should be spent")
			assert.Equal(t, []*balance.Balance{balance.New(balance.ColorIOTA, 1111)}, output.Balances())
			assert.Equal(t, branches["C"], output.BranchID(), "the output was booked into the wrong branch")
			assert.True(t, output.Solid(), "the output is not solid")
		}))

		// check if the balance on address D is found in the database
		assert.True(t, tangle.OutputsOnAddress(seed.Address(D)).Consume(func(output *Output) {
			assert.Equal(t, 1, output.ConsumerCount(), "the output should be spent")
			assert.Equal(t, []*balance.Balance{balance.New(balance.ColorIOTA, 1111)}, output.Balances())
			assert.Equal(t, branches["A"], output.BranchID(), "the output was booked into the wrong branch")
			assert.True(t, output.Solid(), "the output is not solid")
		}))

		// check if the balance on address I is found in the database
		assert.True(t, tangle.OutputsOnAddress(seed.Address(I)).Consume(func(output *Output) {
			assert.Equal(t, 0, output.ConsumerCount(), "the output should not be spent")
			assert.Equal(t, []*balance.Balance{balance.New(balance.ColorIOTA, 2222)}, output.Balances())
			assert.Equal(t, branches["AC"], output.BranchID(), "the output was booked into the wrong branch")
			assert.True(t, output.Solid(), "the output is not solid")
		}))
	}

	// [-B, J+]
	{
		// create transaction + payload
		transactions["[-B, J+]"] = transaction.New(
			transaction.NewInputs(
				transaction.NewOutputID(seed.Address(B), transactions["[-GENESIS, A+, B+, C+]"].ID()),
			),

			transaction.NewOutputs(map[address.Address][]*balance.Balance{
				seed.Address(J): {
					balance.New(balance.ColorIOTA, 1111),
				},
			}),
		)
		transactions["[-B, J+]"].Sign(signaturescheme.ED25519(*seed.KeyPair(B)))
		valueObjects["[-B, J+]"] = payload.New(valueObjects["[-C, H+]"].ID(), valueObjects["[-A, D+]"].ID(), transactions["[-B, J+]"])

		// check if signatures are valid
		assert.True(t, transactions["[-B, J+]"].SignaturesValid())

		tangle.Expect("PayloadAttached", valueObjects["[-B, J+]"], mock.Anything)
		tangle.Expect("PayloadSolid", valueObjects["[-B, J+]"], mock.Anything)
		tangle.Expect("TransactionReceived", transactions["[-B, J+]"], mock.Anything, mock.Anything)
		tangle.Expect("TransactionSolid", transactions["[-B, J+]"], mock.Anything)
		tangle.Expect("TransactionBooked", transactions["[-B, J+]"], mock.Anything, true)

		// attach payload
		tangle.AttachPayloadSync(valueObjects["[-B, J+]"])

		// create alias for the branch
		branches["E"] = branchmanager.NewBranchID(transactions["[-B, J+]"].ID())

		// check if transaction metadata is found in database
		assert.True(t, tangle.TransactionMetadata(transactions["[-B, J+]"].ID()).Consume(func(transactionMetadata *TransactionMetadata) {
			assert.True(t, transactionMetadata.Solid(), "the transaction is not solid")
			assert.Equal(t, branches["E"], transactionMetadata.BranchID(), "the transaction was booked into the wrong branch")
		}))

		// create alias for the branch
		branches["ACE"] = tangle.branchManager.GenerateAggregatedBranchID(branches["A"], branches["C"], branches["E"])

		// check if payload metadata is found in database
		assert.True(t, tangle.PayloadMetadata(valueObjects["[-B, J+]"].ID()).Consume(func(payloadMetadata *PayloadMetadata) {
			assert.True(t, payloadMetadata.IsSolid(), "the payload is not solid")
			assert.Equal(t, branches["ACE"], payloadMetadata.BranchID(), "the payload was booked into the wrong branch")
		}))

		// check if the balance on address B is found in the database
		assert.True(t, tangle.OutputsOnAddress(seed.Address(B)).Consume(func(output *Output) {
			assert.Equal(t, 2, output.ConsumerCount(), "the output should be spent")
			assert.Equal(t, []*balance.Balance{balance.New(balance.ColorIOTA, 1111)}, output.Balances())
			assert.Equal(t, branchmanager.MasterBranchID, output.BranchID(), "the output was booked into the wrong branch")
			assert.True(t, output.Solid(), "the output is not solid")
		}))

		// check if the balance on address J is found in the database
		assert.True(t, tangle.OutputsOnAddress(seed.Address(J)).Consume(func(output *Output) {
			assert.Equal(t, 0, output.ConsumerCount(), "the output should not be spent")
			assert.Equal(t, []*balance.Balance{balance.New(balance.ColorIOTA, 1111)}, output.Balances())
			assert.Equal(t, branches["E"], output.BranchID(), "the output was booked into the wrong branch")
			assert.True(t, output.Solid(), "the output is not solid")
		}))

		// check if the branches D and E are conflicting
		branchesConflicting, err := tangle.branchManager.BranchesConflicting(branches["D"], branches["E"])
		require.NoError(t, err)
		assert.True(t, branchesConflicting, "the branches should be conflicting")

	}

	return tangle, transactions, valueObjects, branches, seed
}

func TestPropagationScenario1(t *testing.T) {
	// img/scenario1.png

	// test past cone monotonicity - all value objects MUST be confirmed
	{
		tangle, transactions, valueObjects, _, _ := preparePropagationScenario1(t)
		defer tangle.DetachAll()

		// initialize debugger for this test
		debugger.ResetAliases()
		for name, valueObject := range valueObjects {
			debugger.RegisterAlias(valueObject.ID(), "ValueObjectID"+name)
		}
		for name, tx := range transactions {
			debugger.RegisterAlias(tx.ID(), "TransactionID"+name)
		}

		// preferring [-GENESIS, A+, B+, C+] will get it liked
		tangle.Expect("TransactionPreferred", transactions["[-GENESIS, A+, B+, C+]"], mock.Anything)
		tangle.Expect("TransactionLiked", transactions["[-GENESIS, A+, B+, C+]"], mock.Anything)
		tangle.Expect("PayloadLiked", valueObjects["[-GENESIS, A+, B+, C+]"], mock.Anything)

		setTransactionPreferredWithCheck(t, tangle, transactions["[-GENESIS, A+, B+, C+]"], true)
		verifyInclusionState(t, tangle, valueObjects["[-GENESIS, A+, B+, C+]"], true, false, true, false, false)

		// finalizing [-B, -C, E+] will not get it confirmed, as [-GENESIS, A+, B+, C+] is not yet confirmed
		tangle.Expect("TransactionPreferred", transactions["[-B, -C, E+]"], mock.Anything)
		tangle.Expect("TransactionLiked", transactions["[-B, -C, E+]"], mock.Anything)
		tangle.Expect("TransactionFinalized", transactions["[-B, -C, E+]"], mock.Anything)
		tangle.Expect("PayloadLiked", valueObjects["[-B, -C, E+]"], mock.Anything)
		tangle.Expect("PayloadLiked", valueObjects["[-B, -C, E+] (Reattachment)"], mock.Anything)

		setTransactionPreferredWithCheck(t, tangle, transactions["[-B, -C, E+]"], true)
		setTransactionFinalizedWithCheck(t, tangle, transactions["[-B, -C, E+]"])
		verifyInclusionState(t, tangle, valueObjects["[-B, -C, E+]"], true, true, true, false, false)

		// finalize [-GENESIS, A+, B+, C+] to also get [-B, -C, E+] as well as [-B, -C, E+] (Reattachment) confirmed
		tangle.Expect("TransactionFinalized", transactions["[-GENESIS, A+, B+, C+]"], mock.Anything)
		tangle.Expect("TransactionConfirmed", transactions["[-GENESIS, A+, B+, C+]"], mock.Anything)
		tangle.Expect("TransactionConfirmed", transactions["[-B, -C, E+]"], mock.Anything)
		tangle.Expect("PayloadConfirmed", valueObjects["[-GENESIS, A+, B+, C+]"], mock.Anything)
		tangle.Expect("PayloadConfirmed", valueObjects["[-B, -C, E+]"], mock.Anything)
		tangle.Expect("PayloadConfirmed", valueObjects["[-B, -C, E+] (Reattachment)"], mock.Anything)

		setTransactionFinalizedWithCheck(t, tangle, transactions["[-GENESIS, A+, B+, C+]"])
		verifyInclusionState(t, tangle, valueObjects["[-GENESIS, A+, B+, C+]"], true, true, true, true, false)
		verifyInclusionState(t, tangle, valueObjects["[-B, -C, E+]"], true, true, true, true, false)
		verifyInclusionState(t, tangle, valueObjects["[-B, -C, E+] (Reattachment)"], true, true, true, true, false)

		tangle.AssertExpectations(t)
	}

	// test future cone monotonicity simple - everything MUST be rejected and finalized if spending funds from rejected tx
	{
		tangle, transactions, valueObjects, _, _ := preparePropagationScenario1(t)
		defer tangle.DetachAll()

		// finalizing [-GENESIS, A+, B+, C+] will get the entire future cone finalized and rejected
		tangle.Expect("TransactionFinalized", transactions["[-GENESIS, A+, B+, C+]"], mock.Anything)
		tangle.Expect("TransactionRejected", transactions["[-GENESIS, A+, B+, C+]"], mock.Anything)
		tangle.Expect("TransactionFinalized", transactions["[-B, -C, E+]"], mock.Anything)
		tangle.Expect("TransactionRejected", transactions["[-B, -C, E+]"], mock.Anything)
		tangle.Expect("TransactionFinalized", transactions["[-A, D+]"], mock.Anything)
		tangle.Expect("TransactionRejected", transactions["[-A, D+]"], mock.Anything)
		tangle.Expect("TransactionFinalized", transactions["[-A, F+]"], mock.Anything)
		tangle.Expect("TransactionRejected", transactions["[-A, F+]"], mock.Anything)
		tangle.Expect("TransactionFinalized", transactions["[-E, -F, G+]"], mock.Anything)
		tangle.Expect("TransactionRejected", transactions["[-E, -F, G+]"], mock.Anything)
		tangle.Expect("PayloadRejected", valueObjects["[-GENESIS, A+, B+, C+]"], mock.Anything)
		tangle.Expect("PayloadRejected", valueObjects["[-B, -C, E+]"], mock.Anything)
		tangle.Expect("PayloadRejected", valueObjects["[-B, -C, E+] (Reattachment)"], mock.Anything)
		tangle.Expect("PayloadRejected", valueObjects["[-A, D+]"], mock.Anything)
		tangle.Expect("PayloadRejected", valueObjects["[-A, F+]"], mock.Anything)
		tangle.Expect("PayloadRejected", valueObjects["[-E, -F, G+]"], mock.Anything)

		setTransactionFinalizedWithCheck(t, tangle, transactions["[-GENESIS, A+, B+, C+]"])
		verifyInclusionState(t, tangle, valueObjects["[-GENESIS, A+, B+, C+]"], false, true, false, false, true)
		verifyInclusionState(t, tangle, valueObjects["[-B, -C, E+]"], false, true, false, false, true)
		verifyInclusionState(t, tangle, valueObjects["[-B, -C, E+] (Reattachment)"], false, true, false, false, true)
		verifyInclusionState(t, tangle, valueObjects["[-A, D+]"], false, true, false, false, true)
		verifyInclusionState(t, tangle, valueObjects["[-A, F+]"], false, true, false, false, true)
		verifyInclusionState(t, tangle, valueObjects["[-E, -F, G+]"], false, true, false, false, true)

		tangle.AssertExpectations(t)
	}

	// test future cone monotonicity more complex - everything MUST be rejected and finalized if spending funds from rejected tx
	{
		tangle, transactions, valueObjects, branches, _ := preparePropagationScenario1(t)
		defer tangle.DetachAll()

		// initialize debugger for this test
		debugger.ResetAliases()
		for name, valueObject := range valueObjects {
			debugger.RegisterAlias(valueObject.ID(), "ValueObjectID"+name)
		}
		for name, tx := range transactions {
			debugger.RegisterAlias(tx.ID(), "TransactionID"+name)
		}

		tangle.Expect("TransactionPreferred", transactions["[-GENESIS, A+, B+, C+]"], mock.Anything)
		tangle.Expect("TransactionLiked", transactions["[-GENESIS, A+, B+, C+]"], mock.Anything)
		tangle.Expect("TransactionFinalized", transactions["[-GENESIS, A+, B+, C+]"], mock.Anything)
		tangle.Expect("TransactionConfirmed", transactions["[-GENESIS, A+, B+, C+]"], mock.Anything)
		tangle.Expect("PayloadLiked", valueObjects["[-GENESIS, A+, B+, C+]"], mock.Anything)
		tangle.Expect("PayloadConfirmed", valueObjects["[-GENESIS, A+, B+, C+]"], mock.Anything)

		setTransactionPreferredWithCheck(t, tangle, transactions["[-GENESIS, A+, B+, C+]"], true)
		setTransactionFinalizedWithCheck(t, tangle, transactions["[-GENESIS, A+, B+, C+]"])
		verifyInclusionState(t, tangle, valueObjects["[-GENESIS, A+, B+, C+]"], true, true, true, true, false)

		tangle.Expect("PayloadRejected", valueObjects["[-B, -C, E+]"], mock.Anything)
		tangle.Expect("TransactionFinalized", transactions["[-B, -C, E+]"], mock.Anything)
		tangle.Expect("TransactionRejected", transactions["[-B, -C, E+]"], mock.Anything)
		tangle.Expect("PayloadRejected", valueObjects["[-B, -C, E+] (Reattachment)"], mock.Anything)
		tangle.Expect("PayloadRejected", valueObjects["[-A, F+]"], mock.Anything)
		tangle.Expect("PayloadRejected", valueObjects["[-E, -F, G+]"], mock.Anything)
		tangle.Expect("TransactionFinalized", transactions["[-E, -F, G+]"], mock.Anything)
		tangle.Expect("TransactionRejected", transactions["[-E, -F, G+]"], mock.Anything)

		// finalize & reject
		//debugger.Enable()
		setTransactionFinalizedWithCheck(t, tangle, transactions["[-B, -C, E+]"])
		verifyInclusionState(t, tangle, valueObjects["[-B, -C, E+]"], false, true, false, false, true)
		//debugger.Disable()

		// check future cone to be rejected
		verifyInclusionState(t, tangle, valueObjects["[-B, -C, E+] (Reattachment)"], false, true, false, false, true)

		// [-A, F+] should be rejected but the tx not finalized since it spends funds from [-GENESIS, A+, B+, C+] which is confirmed
		verifyTransactionInclusionState(t, tangle, valueObjects["[-A, F+]"], false, false, false, false, false)
		verifyValueObjectInclusionState(t, tangle, valueObjects["[-A, F+]"], false, false, true)
		verifyBranchState(t, tangle, branches["B"], false, false, false, false)

		// [-E, -F, G+] should be finalized and rejected since it spends funds from [-B, -C, E+]
		verifyInclusionState(t, tangle, valueObjects["[-E, -F, G+]"], false, true, false, false, true)

		// [-A, D+] should be unchanged
		verifyInclusionState(t, tangle, valueObjects["[-A, D+]"], false, false, false, false, false)
		verifyBranchState(t, tangle, branches["A"], false, false, false, false)

		tangle.AssertExpectations(t)
	}

	// simulate vote on [-A, F+] -> Branch A becomes rejected, Branch B confirmed
	{
		tangle, transactions, valueObjects, branches, _ := preparePropagationScenario1(t)
		defer tangle.DetachAll()

		tangle.Expect("PayloadLiked", valueObjects["[-GENESIS, A+, B+, C+]"], mock.Anything)
		tangle.Expect("PayloadConfirmed", valueObjects["[-GENESIS, A+, B+, C+]"], mock.Anything)
		tangle.Expect("TransactionPreferred", transactions["[-GENESIS, A+, B+, C+]"], mock.Anything)
		tangle.Expect("TransactionLiked", transactions["[-GENESIS, A+, B+, C+]"], mock.Anything)
		tangle.Expect("TransactionFinalized", transactions["[-GENESIS, A+, B+, C+]"], mock.Anything)
		tangle.Expect("TransactionConfirmed", transactions["[-GENESIS, A+, B+, C+]"], mock.Anything)

		setTransactionPreferredWithCheck(t, tangle, transactions["[-GENESIS, A+, B+, C+]"], true)
		setTransactionFinalizedWithCheck(t, tangle, transactions["[-GENESIS, A+, B+, C+]"])
		verifyInclusionState(t, tangle, valueObjects["[-GENESIS, A+, B+, C+]"], true, true, true, true, false)

		// check future cone
		verifyInclusionState(t, tangle, valueObjects["[-B, -C, E+]"], false, false, false, false, false)
		verifyInclusionState(t, tangle, valueObjects["[-B, -C, E+] (Reattachment)"], false, false, false, false, false)
		verifyInclusionState(t, tangle, valueObjects["[-A, D+]"], false, false, false, false, false)
		verifyInclusionState(t, tangle, valueObjects["[-A, F+]"], false, false, false, false, false)
		verifyInclusionState(t, tangle, valueObjects["[-E, -F, G+]"], false, false, false, false, false)

		tangle.Expect("PayloadLiked", valueObjects["[-B, -C, E+]"], mock.Anything)
		tangle.Expect("PayloadConfirmed", valueObjects["[-B, -C, E+]"], mock.Anything)
		tangle.Expect("PayloadLiked", valueObjects["[-B, -C, E+] (Reattachment)"], mock.Anything)
		tangle.Expect("PayloadConfirmed", valueObjects["[-B, -C, E+] (Reattachment)"], mock.Anything)
		tangle.Expect("TransactionPreferred", transactions["[-B, -C, E+]"], mock.Anything)
		tangle.Expect("TransactionLiked", transactions["[-B, -C, E+]"], mock.Anything)
		tangle.Expect("TransactionFinalized", transactions["[-B, -C, E+]"], mock.Anything)
		tangle.Expect("TransactionConfirmed", transactions["[-B, -C, E+]"], mock.Anything)

		// confirm [-B, -C, E+]
		setTransactionPreferredWithCheck(t, tangle, transactions["[-B, -C, E+]"], true)
		setTransactionFinalizedWithCheck(t, tangle, transactions["[-B, -C, E+]"])
		verifyInclusionState(t, tangle, valueObjects["[-B, -C, E+]"], true, true, true, true, false)
		verifyInclusionState(t, tangle, valueObjects["[-B, -C, E+] (Reattachment)"], true, true, true, true, false)

		tangle.Expect("PayloadLiked", valueObjects["[-A, D+]"], mock.Anything)
		tangle.Expect("TransactionPreferred", transactions["[-A, D+]"], mock.Anything)
		tangle.Expect("TransactionLiked", transactions["[-A, D+]"], mock.Anything)

		// prefer [-A, D+]
		setTransactionPreferredWithCheck(t, tangle, transactions["[-A, D+]"], true)
		verifyInclusionState(t, tangle, valueObjects["[-A, D+]"], true, false, true, false, false)
		verifyBranchState(t, tangle, branches["A"], false, true, false, false)

		tangle.Expect("PayloadLiked", valueObjects["[-A, F+]"], mock.Anything)
		tangle.Expect("PayloadConfirmed", valueObjects["[-A, F+]"], mock.Anything)
		tangle.Expect("TransactionPreferred", transactions["[-A, F+]"], mock.Anything)
		tangle.Expect("TransactionLiked", transactions["[-A, F+]"], mock.Anything)
		tangle.Expect("TransactionFinalized", transactions["[-A, F+]"], mock.Anything)
		tangle.Expect("TransactionConfirmed", transactions["[-A, F+]"], mock.Anything)
		tangle.Expect("PayloadRejected", valueObjects["[-A, D+]"], mock.Anything)
		tangle.Expect("PayloadDisliked", valueObjects["[-A, D+]"], mock.Anything)
		tangle.Expect("TransactionUnpreferred", transactions["[-A, D+]"], mock.Anything)
		tangle.Expect("TransactionDisliked", transactions["[-A, D+]"], mock.Anything)
		tangle.Expect("TransactionFinalized", transactions["[-A, D+]"], mock.Anything)
		tangle.Expect("TransactionRejected", transactions["[-A, D+]"], mock.Anything)

		// simulate vote result to like [-A, F+] -> [-A, F+] becomes confirmed and [-A, D+] rejected
		setTransactionPreferredWithCheck(t, tangle, transactions["[-A, F+]"], true)
		setTransactionFinalizedWithCheck(t, tangle, transactions["[-A, F+]"])
		verifyInclusionState(t, tangle, valueObjects["[-A, F+]"], true, true, true, true, false)
		verifyBranchState(t, tangle, branches["B"], true, true, true, false)
		// [-A, D+] should be rejected
		verifyInclusionState(t, tangle, valueObjects["[-A, D+]"], false, true, false, false, true)
		verifyBranchState(t, tangle, branches["A"], true, false, false, true)

		tangle.Expect("PayloadLiked", valueObjects["[-E, -F, G+]"], mock.Anything)
		tangle.Expect("PayloadConfirmed", valueObjects["[-E, -F, G+]"], mock.Anything)
		tangle.Expect("TransactionPreferred", transactions["[-E, -F, G+]"], mock.Anything)
		tangle.Expect("TransactionLiked", transactions["[-E, -F, G+]"], mock.Anything)
		tangle.Expect("TransactionFinalized", transactions["[-E, -F, G+]"], mock.Anything)
		tangle.Expect("TransactionConfirmed", transactions["[-E, -F, G+]"], mock.Anything)

		verifyInclusionState(t, tangle, valueObjects["[-E, -F, G+]"], false, false, false, false, false)
		setTransactionPreferredWithCheck(t, tangle, transactions["[-E, -F, G+]"], true)
		setTransactionFinalizedWithCheck(t, tangle, transactions["[-E, -F, G+]"])
		verifyInclusionState(t, tangle, valueObjects["[-E, -F, G+]"], true, true, true, true, false)

		tangle.AssertExpectations(t)
	}

	// simulate vote on [-A, D+] -> Branch B becomes rejected, Branch A confirmed
	{
		tangle, transactions, valueObjects, branches, _ := preparePropagationScenario1(t)
		defer tangle.DetachAll()

		tangle.Expect("PayloadLiked", valueObjects["[-GENESIS, A+, B+, C+]"], mock.Anything)
		tangle.Expect("PayloadConfirmed", valueObjects["[-GENESIS, A+, B+, C+]"], mock.Anything)
		tangle.Expect("TransactionPreferred", transactions["[-GENESIS, A+, B+, C+]"], mock.Anything)
		tangle.Expect("TransactionLiked", transactions["[-GENESIS, A+, B+, C+]"], mock.Anything)
		tangle.Expect("TransactionFinalized", transactions["[-GENESIS, A+, B+, C+]"], mock.Anything)
		tangle.Expect("TransactionConfirmed", transactions["[-GENESIS, A+, B+, C+]"], mock.Anything)

		// confirm [-GENESIS, A+, B+, C+]
		setTransactionPreferredWithCheck(t, tangle, transactions["[-GENESIS, A+, B+, C+]"], true)
		setTransactionFinalizedWithCheck(t, tangle, transactions["[-GENESIS, A+, B+, C+]"])
		verifyInclusionState(t, tangle, valueObjects["[-GENESIS, A+, B+, C+]"], true, true, true, true, false)

		tangle.Expect("PayloadLiked", valueObjects["[-B, -C, E+]"], mock.Anything)
		tangle.Expect("PayloadConfirmed", valueObjects["[-B, -C, E+]"], mock.Anything)
		tangle.Expect("PayloadLiked", valueObjects["[-B, -C, E+] (Reattachment)"], mock.Anything)
		tangle.Expect("PayloadConfirmed", valueObjects["[-B, -C, E+] (Reattachment)"], mock.Anything)
		tangle.Expect("TransactionPreferred", transactions["[-B, -C, E+]"], mock.Anything)
		tangle.Expect("TransactionLiked", transactions["[-B, -C, E+]"], mock.Anything)
		tangle.Expect("TransactionFinalized", transactions["[-B, -C, E+]"], mock.Anything)
		tangle.Expect("TransactionConfirmed", transactions["[-B, -C, E+]"], mock.Anything)

		// confirm [-B, -C, E+]
		setTransactionPreferredWithCheck(t, tangle, transactions["[-B, -C, E+]"], true)
		setTransactionFinalizedWithCheck(t, tangle, transactions["[-B, -C, E+]"])
		verifyInclusionState(t, tangle, valueObjects["[-B, -C, E+]"], true, true, true, true, false)

		tangle.Expect("PayloadLiked", valueObjects["[-A, F+]"], mock.Anything)
		tangle.Expect("TransactionPreferred", transactions["[-A, F+]"], mock.Anything)
		tangle.Expect("TransactionLiked", transactions["[-A, F+]"], mock.Anything)

		// prefer [-A, F+] and thus Branch B
		setTransactionPreferredWithCheck(t, tangle, transactions["[-A, F+]"], true)
		verifyInclusionState(t, tangle, valueObjects["[-A, F+]"], true, false, true, false, false)
		verifyBranchState(t, tangle, branches["B"], false, true, false, false)

		tangle.Expect("PayloadLiked", valueObjects["[-E, -F, G+]"], mock.Anything)
		tangle.Expect("TransactionPreferred", transactions["[-E, -F, G+]"], mock.Anything)
		tangle.Expect("TransactionLiked", transactions["[-E, -F, G+]"], mock.Anything)

		// prefer [-E, -F, G+]
		setTransactionPreferredWithCheck(t, tangle, transactions["[-E, -F, G+]"], true)
		verifyInclusionState(t, tangle, valueObjects["[-E, -F, G+]"], true, false, true, false, false)

		tangle.Expect("PayloadLiked", valueObjects["[-A, D+]"], mock.Anything)
		tangle.Expect("PayloadConfirmed", valueObjects["[-A, D+]"], mock.Anything)
		tangle.Expect("TransactionPreferred", transactions["[-A, D+]"], mock.Anything)
		tangle.Expect("TransactionLiked", transactions["[-A, D+]"], mock.Anything)
		tangle.Expect("TransactionFinalized", transactions["[-A, D+]"], mock.Anything)
		tangle.Expect("TransactionConfirmed", transactions["[-A, D+]"], mock.Anything)

		tangle.Expect("PayloadRejected", valueObjects["[-A, F+]"], mock.Anything)
		tangle.Expect("PayloadDisliked", valueObjects["[-A, F+]"], mock.Anything)
		tangle.Expect("TransactionUnpreferred", transactions["[-A, F+]"], mock.Anything)
		tangle.Expect("TransactionDisliked", transactions["[-A, F+]"], mock.Anything)
		tangle.Expect("TransactionFinalized", transactions["[-A, F+]"], mock.Anything)
		tangle.Expect("TransactionRejected", transactions["[-A, F+]"], mock.Anything)
		tangle.Expect("PayloadRejected", valueObjects["[-E, -F, G+]"], mock.Anything)
		tangle.Expect("PayloadDisliked", valueObjects["[-E, -F, G+]"], mock.Anything)
		tangle.Expect("TransactionUnpreferred", transactions["[-E, -F, G+]"], mock.Anything)
		tangle.Expect("TransactionDisliked", transactions["[-E, -F, G+]"], mock.Anything)
		tangle.Expect("TransactionFinalized", transactions["[-E, -F, G+]"], mock.Anything)
		tangle.Expect("TransactionRejected", transactions["[-E, -F, G+]"], mock.Anything)

		// simulate vote result to like [-A, D+] -> [-A, D+] becomes confirmed and [-A, F+], [-E, -F, G+] rejected
		setTransactionPreferredWithCheck(t, tangle, transactions["[-A, D+]"], true)
		setTransactionFinalizedWithCheck(t, tangle, transactions["[-A, D+]"])
		verifyInclusionState(t, tangle, valueObjects["[-A, D+]"], true, true, true, true, false)
		verifyBranchState(t, tangle, branches["A"], true, true, true, false)

		// [-A, F+], [-E, -F, G+] should be finalized and rejected
		verifyInclusionState(t, tangle, valueObjects["[-A, F+]"], false, true, false, false, true)
		verifyBranchState(t, tangle, branches["B"], true, false, false, true)
		verifyInclusionState(t, tangle, valueObjects["[-E, -F, G+]"], false, true, false, false, true)

		tangle.AssertExpectations(t)
	}
}

func TestPropagationScenario2(t *testing.T) {
	// img/scenario2.png
	tangle, transactions, valueObjects, branches, _ := preparePropagationScenario2(t)
	defer tangle.DetachAll()

	// initialize debugger for this test
	debugger.ResetAliases()
	for name, valueObject := range valueObjects {
		debugger.RegisterAlias(valueObject.ID(), "ValueObjectID"+name)
	}
	for name, tx := range transactions {
		debugger.RegisterAlias(tx.ID(), "TransactionID"+name)
	}

	tangle.Expect("PayloadLiked", valueObjects["[-GENESIS, A+, B+, C+]"], mock.Anything)
	tangle.Expect("PayloadConfirmed", valueObjects["[-GENESIS, A+, B+, C+]"], mock.Anything)
	tangle.Expect("TransactionPreferred", transactions["[-GENESIS, A+, B+, C+]"], mock.Anything)
	tangle.Expect("TransactionLiked", transactions["[-GENESIS, A+, B+, C+]"], mock.Anything)
	tangle.Expect("TransactionFinalized", transactions["[-GENESIS, A+, B+, C+]"], mock.Anything)
	tangle.Expect("TransactionConfirmed", transactions["[-GENESIS, A+, B+, C+]"], mock.Anything)

	// confirm [-GENESIS, A+, B+, C+]
	setTransactionPreferredWithCheck(t, tangle, transactions["[-GENESIS, A+, B+, C+]"], true)
	setTransactionFinalizedWithCheck(t, tangle, transactions["[-GENESIS, A+, B+, C+]"])
	verifyInclusionState(t, tangle, valueObjects["[-GENESIS, A+, B+, C+]"], true, true, true, true, false)

	tangle.Expect("PayloadLiked", valueObjects["[-B, -C, E+]"], mock.Anything)
	tangle.Expect("PayloadLiked", valueObjects["[-B, -C, E+] (Reattachment)"], mock.Anything)
	tangle.Expect("TransactionPreferred", transactions["[-B, -C, E+]"], mock.Anything)
	tangle.Expect("TransactionLiked", transactions["[-B, -C, E+]"], mock.Anything)

	// prefer [-B, -C, E+] and thus Branch D
	setTransactionPreferredWithCheck(t, tangle, transactions["[-B, -C, E+]"], true)
	verifyInclusionState(t, tangle, valueObjects["[-B, -C, E+]"], true, false, true, false, false)
	verifyBranchState(t, tangle, branches["D"], false, true, false, false)
	verifyInclusionState(t, tangle, valueObjects["[-B, -C, E+] (Reattachment)"], true, false, true, false, false)

	tangle.Expect("PayloadLiked", valueObjects["[-A, F+]"], mock.Anything)
	tangle.Expect("TransactionPreferred", transactions["[-A, F+]"], mock.Anything)
	tangle.Expect("TransactionLiked", transactions["[-A, F+]"], mock.Anything)

	// prefer [-A, F+] and thus Branch B
	setTransactionPreferredWithCheck(t, tangle, transactions["[-A, F+]"], true)
	verifyInclusionState(t, tangle, valueObjects["[-A, F+]"], true, false, true, false, false)
	verifyBranchState(t, tangle, branches["B"], false, true, false, false)

	tangle.Expect("PayloadLiked", valueObjects["[-E, -F, G+]"], mock.Anything)
	tangle.Expect("TransactionPreferred", transactions["[-E, -F, G+]"], mock.Anything)
	tangle.Expect("TransactionLiked", transactions["[-E, -F, G+]"], mock.Anything)

	// prefer [-E, -F, G+]
	setTransactionPreferredWithCheck(t, tangle, transactions["[-E, -F, G+]"], true)
	verifyInclusionState(t, tangle, valueObjects["[-E, -F, G+]"], true, false, true, false, false)
	// check aggregated branch
	verifyBranchState(t, tangle, branches["BD"], false, true, false, false)

	// verify states of other transactions, value objects and branches
	verifyInclusionState(t, tangle, valueObjects["[-A, D+]"], false, false, false, false, false)
	verifyBranchState(t, tangle, branches["A"], false, false, false, false)

	verifyInclusionState(t, tangle, valueObjects["[-C, H+]"], false, false, false, false, false)
	verifyBranchState(t, tangle, branches["C"], false, false, false, false)

	verifyInclusionState(t, tangle, valueObjects["[-H, -D, I+]"], false, false, false, false, false)
	// check aggregated branch
	verifyBranchState(t, tangle, branches["AC"], false, false, false, false)

	verifyInclusionState(t, tangle, valueObjects["[-B, J+]"], false, false, false, false, false)
	verifyBranchState(t, tangle, branches["E"], false, false, false, false)
	verifyBranchState(t, tangle, branches["ACE"], false, false, false, false)

	tangle.Expect("PayloadLiked", valueObjects["[-H, -D, I+]"], mock.Anything)
	tangle.Expect("TransactionPreferred", transactions["[-H, -D, I+]"], mock.Anything)
	tangle.Expect("TransactionLiked", transactions["[-H, -D, I+]"], mock.Anything)

	// prefer [-H, -D, I+] - should be liked after votes on [-A, D+] and [-C, H+]
	setTransactionPreferredWithCheck(t, tangle, transactions["[-H, -D, I+]"], true)
	verifyInclusionState(t, tangle, valueObjects["[-H, -D, I+]"], true, false, false, false, false)

	tangle.Expect("PayloadLiked", valueObjects["[-A, D+]"], mock.Anything)
	tangle.Expect("PayloadConfirmed", valueObjects["[-A, D+]"], mock.Anything)
	tangle.Expect("TransactionPreferred", transactions["[-A, D+]"], mock.Anything)
	tangle.Expect("TransactionLiked", transactions["[-A, D+]"], mock.Anything)
	tangle.Expect("TransactionFinalized", transactions["[-A, D+]"], mock.Anything)
	tangle.Expect("TransactionConfirmed", transactions["[-A, D+]"], mock.Anything)

	tangle.Expect("PayloadRejected", valueObjects["[-A, F+]"], mock.Anything)
	tangle.Expect("PayloadDisliked", valueObjects["[-A, F+]"], mock.Anything)
	tangle.Expect("TransactionUnpreferred", transactions["[-A, F+]"], mock.Anything)
	tangle.Expect("TransactionDisliked", transactions["[-A, F+]"], mock.Anything)
	tangle.Expect("TransactionFinalized", transactions["[-A, F+]"], mock.Anything)
	tangle.Expect("TransactionRejected", transactions["[-A, F+]"], mock.Anything)
	tangle.Expect("PayloadRejected", valueObjects["[-E, -F, G+]"], mock.Anything)
	tangle.Expect("PayloadDisliked", valueObjects["[-E, -F, G+]"], mock.Anything)
	tangle.Expect("TransactionUnpreferred", transactions["[-E, -F, G+]"], mock.Anything)
	tangle.Expect("TransactionDisliked", transactions["[-E, -F, G+]"], mock.Anything)
	tangle.Expect("TransactionFinalized", transactions["[-E, -F, G+]"], mock.Anything)
	tangle.Expect("TransactionRejected", transactions["[-E, -F, G+]"], mock.Anything)

	// simulate vote result to like [-A, D+] -> [-A, D+] becomes confirmed and [-A, F+], [-E, -F, G+] rejected
	setTransactionPreferredWithCheck(t, tangle, transactions["[-A, D+]"], true)
	setTransactionFinalizedWithCheck(t, tangle, transactions["[-A, D+]"])
	verifyInclusionState(t, tangle, valueObjects["[-A, D+]"], true, true, true, true, false)
	verifyBranchState(t, tangle, branches["A"], true, true, true, false)

	verifyInclusionState(t, tangle, valueObjects["[-A, F+]"], false, true, false, false, true)
	verifyBranchState(t, tangle, branches["B"], true, false, false, true)
	verifyInclusionState(t, tangle, valueObjects["[-E, -F, G+]"], false, true, false, false, true)

	tangle.Expect("PayloadLiked", valueObjects["[-C, H+]"], mock.Anything)
	tangle.Expect("PayloadConfirmed", valueObjects["[-C, H+]"], mock.Anything)
	tangle.Expect("TransactionPreferred", transactions["[-C, H+]"], mock.Anything)
	tangle.Expect("TransactionLiked", transactions["[-C, H+]"], mock.Anything)
	tangle.Expect("TransactionFinalized", transactions["[-C, H+]"], mock.Anything)
	tangle.Expect("TransactionConfirmed", transactions["[-C, H+]"], mock.Anything)

	tangle.Expect("PayloadRejected", valueObjects["[-B, -C, E+]"], mock.Anything)
	tangle.Expect("PayloadDisliked", valueObjects["[-B, -C, E+]"], mock.Anything)
	tangle.Expect("TransactionUnpreferred", transactions["[-B, -C, E+]"], mock.Anything)
	tangle.Expect("TransactionDisliked", transactions["[-B, -C, E+]"], mock.Anything)
	tangle.Expect("TransactionFinalized", transactions["[-B, -C, E+]"], mock.Anything)
	tangle.Expect("TransactionRejected", transactions["[-B, -C, E+]"], mock.Anything)
	tangle.Expect("PayloadRejected", valueObjects["[-B, -C, E+] (Reattachment)"], mock.Anything)
	tangle.Expect("PayloadDisliked", valueObjects["[-B, -C, E+] (Reattachment)"], mock.Anything)

	// simulate vote result to like [-C, H+] -> [-C, H+] becomes confirmed and [-B, -C, E+], [-B, -C, E+] (Reattachment) rejected
	setTransactionPreferredWithCheck(t, tangle, transactions["[-C, H+]"], true)
	setTransactionFinalizedWithCheck(t, tangle, transactions["[-C, H+]"])

	verifyInclusionState(t, tangle, valueObjects["[-C, H+]"], true, true, true, true, false)
	verifyBranchState(t, tangle, branches["C"], true, true, true, false)
	verifyBranchState(t, tangle, branches["AC"], true, true, true, false)

	verifyInclusionState(t, tangle, valueObjects["[-B, -C, E+]"], false, true, false, false, true)
	verifyBranchState(t, tangle, branches["D"], true, false, false, true)
	verifyInclusionState(t, tangle, valueObjects["[-B, -C, E+] (Reattachment)"], false, true, false, false, true)
	verifyBranchState(t, tangle, branches["BD"], true, false, false, true)
	// TODO: BD is not finalized

	// [-H, -D, I+] is already preferred
	tangle.Expect("PayloadConfirmed", valueObjects["[-H, -D, I+]"], mock.Anything)
	tangle.Expect("TransactionFinalized", transactions["[-H, -D, I+]"], mock.Anything)
	tangle.Expect("TransactionConfirmed", transactions["[-H, -D, I+]"], mock.Anything)

	// [-H, -D, I+] should now be liked
	verifyInclusionState(t, tangle, valueObjects["[-H, -D, I+]"], true, false, true, false, false)
	setTransactionFinalizedWithCheck(t, tangle, transactions["[-H, -D, I+]"])
	verifyInclusionState(t, tangle, valueObjects["[-H, -D, I+]"], true, true, true, true, false)
	// [-B, J+] should be unchanged
	verifyInclusionState(t, tangle, valueObjects["[-B, J+]"], false, false, false, false, false)

	tangle.Expect("PayloadLiked", valueObjects["[-B, J+]"], mock.Anything)
	tangle.Expect("PayloadConfirmed", valueObjects["[-B, J+]"], mock.Anything)
	tangle.Expect("TransactionPreferred", transactions["[-B, J+]"], mock.Anything)
	tangle.Expect("TransactionLiked", transactions["[-B, J+]"], mock.Anything)
	tangle.Expect("TransactionFinalized", transactions["[-B, J+]"], mock.Anything)
	tangle.Expect("TransactionConfirmed", transactions["[-B, J+]"], mock.Anything)

	// [-B, J+] should become confirmed after preferring and finalizing
	setTransactionPreferredWithCheck(t, tangle, transactions["[-B, J+]"], true)
	setTransactionFinalizedWithCheck(t, tangle, transactions["[-B, J+]"])
	verifyInclusionState(t, tangle, valueObjects["[-B, J+]"], true, true, true, true, false)
	verifyBranchState(t, tangle, branches["E"], true, true, true, false)
	verifyBranchState(t, tangle, branches["ACE"], true, true, true, false)

	tangle.AssertExpectations(t)
}

// verifyBranchState verifies the the branch state according to the given parameters.
func verifyBranchState(t *testing.T, tangle *eventTangle, id branchmanager.BranchID, finalized, liked, confirmed, rejected bool) {
	assert.True(t, tangle.branchManager.Branch(id).Consume(func(branch *branchmanager.Branch) {
		assert.Equalf(t, finalized, branch.Finalized(), "branch finalized state does not match")
		assert.Equalf(t, liked, branch.Liked(), "branch liked state does not match")

		assert.Equalf(t, confirmed, branch.Confirmed(), "branch confirmed state does not match")
		assert.Equalf(t, rejected, branch.Rejected(), "branch rejected state does not match")
	}))
}

// verifyInclusionState verifies the inclusion state of outputs and transaction according to the given parameters.
func verifyTransactionInclusionState(t *testing.T, tangle *eventTangle, valueObject *payload.Payload, preferred, finalized, liked, confirmed, rejected bool) {
	tx := valueObject.Transaction()

	// check outputs
	tx.Outputs().ForEach(func(address address.Address, balances []*balance.Balance) bool {
		assert.True(t, tangle.TransactionOutput(transaction.NewOutputID(address, tx.ID())).Consume(func(output *Output) {
			assert.Equalf(t, liked, output.Liked(), "output liked state does not match")
			assert.Equalf(t, confirmed, output.Confirmed(), "output confirmed state does not match")
			assert.Equalf(t, rejected, output.Rejected(), "output rejected state does not match")
		}))
		return true
	})

	// check transaction
	assert.True(t, tangle.TransactionMetadata(tx.ID()).Consume(func(metadata *TransactionMetadata) {
		assert.Equalf(t, preferred, metadata.Preferred(), "tx preferred state does not match")
		assert.Equalf(t, finalized, metadata.Finalized(), "tx finalized state does not match")

		assert.Equalf(t, liked, metadata.Liked(), "tx liked state does not match")
		assert.Equalf(t, confirmed, metadata.Confirmed(), "tx confirmed state does not match")
		assert.Equalf(t, rejected, metadata.Rejected(), "tx rejected state does not match")
	}))
}

// verifyValueObjectInclusionState verifies the inclusion state of a value object according to the given parameters.
func verifyValueObjectInclusionState(t *testing.T, tangle *eventTangle, valueObject *payload.Payload, liked, confirmed, rejected bool) {
	assert.True(t, tangle.PayloadMetadata(valueObject.ID()).Consume(func(payloadMetadata *PayloadMetadata) {
		assert.Equalf(t, liked, payloadMetadata.Liked(), "value object liked state does not match")
		assert.Equalf(t, confirmed, payloadMetadata.Confirmed(), "value object confirmed state does not match")
		assert.Equalf(t, rejected, payloadMetadata.Rejected(), "value object rejected state does not match")
	}))
}

// verifyInclusionState verifies the inclusion state of outputs, transaction and value object according to the given parameters.
func verifyInclusionState(t *testing.T, tangle *eventTangle, valueObject *payload.Payload, preferred, finalized, liked, confirmed, rejected bool) {
	verifyTransactionInclusionState(t, tangle, valueObject, preferred, finalized, liked, confirmed, rejected)
	verifyValueObjectInclusionState(t, tangle, valueObject, liked, confirmed, rejected)
}

// setTransactionPreferredWithCheck sets the transaction to preferred and makes sure that no error occurred and it's modified.
func setTransactionPreferredWithCheck(t *testing.T, tangle *eventTangle, tx *transaction.Transaction, preferred bool) {
	modified, err := tangle.SetTransactionPreferred(tx.ID(), preferred)
	require.NoError(t, err)
	assert.True(t, modified)
}

// setTransactionFinalizedWithCheck sets the transaction to finalized and makes sure that no error occurred and it's modified.
func setTransactionFinalizedWithCheck(t *testing.T, tangle *eventTangle, tx *transaction.Transaction) {
	modified, err := tangle.SetTransactionFinalized(tx.ID())
	require.NoError(t, err)
	assert.True(t, modified)
}
