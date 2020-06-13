package tangle

import (
	"sync"
	"testing"

	"github.com/iotaledger/hive.go/kvstore/mapdb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/address"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/balance"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/branchmanager"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/payload"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/tipmanager"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/transaction"
)

func TestConcurrency(t *testing.T) {
	// img/concurrency.png
	// Builds a simple UTXO-DAG where each transaction spends exactly 1 output from genesis.
	// Tips are concurrently selected (via TipManager) resulting in a moderately wide tangle depending on `threads`.
	tangle := New(mapdb.NewMapDB())
	defer tangle.Shutdown()

	tipManager := tipmanager.New()

	count := 1000
	threads := 10
	countTotal := threads * count

	// initialize tangle with genesis block
	outputs := make(map[address.Address][]*balance.Balance)
	for i := 0; i < countTotal; i++ {
		outputs[address.Random()] = []*balance.Balance{
			balance.New(balance.ColorIOTA, 1),
		}
	}
	inputIDs := loadSnapshotFromOutputs(tangle, outputs)

	transactions := make([]*transaction.Transaction, countTotal)
	valueObjects := make([]*payload.Payload, countTotal)

	// start threads, each working on its chunk of transaction and valueObjects
	var wg sync.WaitGroup
	for thread := 0; thread < threads; thread++ {
		wg.Add(1)
		go func(threadNo int) {
			defer wg.Done()

			start := threadNo * count
			end := start + count

			for i := start; i < end; i++ {
				// issue transaction moving funds from genesis
				tx := transaction.New(
					transaction.NewInputs(inputIDs[i]),
					transaction.NewOutputs(
						map[address.Address][]*balance.Balance{
							address.Random(): {
								balance.New(balance.ColorIOTA, 1),
							},
						}),
				)
				// use random value objects as tips (possibly created in other threads)
				parent1, parent2 := tipManager.Tips()
				valueObject := payload.New(parent1, parent2, tx)

				tangle.AttachPayloadSync(valueObject)

				tipManager.AddTip(valueObject)
				transactions[i] = tx
				valueObjects[i] = valueObject
			}
		}(thread)
	}

	wg.Wait()

	// verify correctness
	for i := 0; i < countTotal; i++ {
		// check if transaction metadata is found in database
		assert.True(t, tangle.TransactionMetadata(transactions[i].ID()).Consume(func(transactionMetadata *TransactionMetadata) {
			assert.Truef(t, transactionMetadata.Solid(), "the transaction is not solid")
			assert.Equalf(t, branchmanager.MasterBranchID, transactionMetadata.BranchID(), "the transaction was booked into the wrong branch")
		}))

		// check if payload metadata is found in database
		assert.True(t, tangle.PayloadMetadata(valueObjects[i].ID()).Consume(func(payloadMetadata *PayloadMetadata) {
			assert.Truef(t, payloadMetadata.IsSolid(), "the payload is not solid")
			assert.Equalf(t, branchmanager.MasterBranchID, payloadMetadata.BranchID(), "the payload was booked into the wrong branch")
		}))

		// check if outputs are found in database
		transactions[i].Outputs().ForEach(func(address address.Address, balances []*balance.Balance) bool {
			cachedOutput := tangle.TransactionOutput(transaction.NewOutputID(address, transactions[i].ID()))
			assert.True(t, cachedOutput.Consume(func(output *Output) {
				assert.Equalf(t, 0, output.ConsumerCount(), "the output should not be spent")
				assert.Equal(t, []*balance.Balance{balance.New(balance.ColorIOTA, 1)}, output.Balances())
				assert.Equalf(t, branchmanager.MasterBranchID, output.BranchID(), "the output was booked into the wrong branch")
				assert.Truef(t, output.Solid(), "the output is not solid")
			}))
			return true
		})

		// check that all inputs are consumed exactly once
		cachedInput := tangle.TransactionOutput(inputIDs[i])
		assert.True(t, cachedInput.Consume(func(output *Output) {
			assert.Equalf(t, 1, output.ConsumerCount(), "the output should be spent")
			assert.Equal(t, []*balance.Balance{balance.New(balance.ColorIOTA, 1)}, output.Balances())
			assert.Equalf(t, branchmanager.MasterBranchID, output.BranchID(), "the output was booked into the wrong branch")
			assert.Truef(t, output.Solid(), "the output is not solid")
		}))
	}
}

func TestReverseValueObjectSolidification(t *testing.T) {
	// img/reverse-valueobject-solidification.png
	// Builds a simple UTXO-DAG where each transaction spends exactly 1 output from genesis.
	// All value objects reference the previous value object, effectively creating a chain.
	// The test attaches the prepared value objects concurrently in reverse order.
	tangle := New(mapdb.NewMapDB())
	defer tangle.Shutdown()

	tipManager := tipmanager.New()

	count := 1000
	threads := 5
	countTotal := threads * count

	// initialize tangle with genesis block
	outputs := make(map[address.Address][]*balance.Balance)
	for i := 0; i < countTotal; i++ {
		outputs[address.Random()] = []*balance.Balance{
			balance.New(balance.ColorIOTA, 1),
		}
	}
	inputIDs := loadSnapshotFromOutputs(tangle, outputs)

	transactions := make([]*transaction.Transaction, countTotal)
	valueObjects := make([]*payload.Payload, countTotal)

	// prepare value objects
	for i := 0; i < countTotal; i++ {
		tx := transaction.New(
			// issue transaction moving funds from genesis
			transaction.NewInputs(inputIDs[i]),
			transaction.NewOutputs(
				map[address.Address][]*balance.Balance{
					address.Random(): {
						balance.New(balance.ColorIOTA, 1),
					},
				}),
		)
		parent1, parent2 := tipManager.Tips()
		valueObject := payload.New(parent1, parent2, tx)

		tipManager.AddTip(valueObject)
		transactions[i] = tx
		valueObjects[i] = valueObject
	}

	// attach value objects in reverse order
	var wg sync.WaitGroup
	for thread := 0; thread < threads; thread++ {
		wg.Add(1)
		go func(threadNo int) {
			defer wg.Done()

			for i := countTotal - 1 - threadNo; i >= 0; i -= threads {
				valueObject := valueObjects[i]
				tangle.AttachPayloadSync(valueObject)
			}
		}(thread)
	}
	wg.Wait()

	// verify correctness
	for i := 0; i < countTotal; i++ {
		// check if transaction metadata is found in database
		assert.True(t, tangle.TransactionMetadata(transactions[i].ID()).Consume(func(transactionMetadata *TransactionMetadata) {
			assert.Truef(t, transactionMetadata.Solid(), "the transaction is not solid")
			assert.Equalf(t, branchmanager.MasterBranchID, transactionMetadata.BranchID(), "the transaction was booked into the wrong branch")
		}))

		// check if payload metadata is found in database
		assert.True(t, tangle.PayloadMetadata(valueObjects[i].ID()).Consume(func(payloadMetadata *PayloadMetadata) {
			assert.Truef(t, payloadMetadata.IsSolid(), "the payload is not solid")
			assert.Equalf(t, branchmanager.MasterBranchID, payloadMetadata.BranchID(), "the payload was booked into the wrong branch")
		}))

		// check if outputs are found in database
		transactions[i].Outputs().ForEach(func(address address.Address, balances []*balance.Balance) bool {
			cachedOutput := tangle.TransactionOutput(transaction.NewOutputID(address, transactions[i].ID()))
			assert.True(t, cachedOutput.Consume(func(output *Output) {
				assert.Equalf(t, 0, output.ConsumerCount(), "the output should not be spent")
				assert.Equal(t, []*balance.Balance{balance.New(balance.ColorIOTA, 1)}, output.Balances())
				assert.Equalf(t, branchmanager.MasterBranchID, output.BranchID(), "the output was booked into the wrong branch")
				assert.Truef(t, output.Solid(), "the output is not solid")
			}))
			return true
		})

		// check that all inputs are consumed exactly once
		cachedInput := tangle.TransactionOutput(inputIDs[i])
		assert.True(t, cachedInput.Consume(func(output *Output) {
			assert.Equalf(t, 1, output.ConsumerCount(), "the output should be spent")
			assert.Equal(t, []*balance.Balance{balance.New(balance.ColorIOTA, 1)}, output.Balances())
			assert.Equalf(t, branchmanager.MasterBranchID, output.BranchID(), "the output was booked into the wrong branch")
			assert.Truef(t, output.Solid(), "the output is not solid")
		}))
	}
}

func TestReverseTransactionSolidification(t *testing.T) {
	testIterations := 500

	// repeat the test a few times
	for k := 0; k < testIterations; k++ {
		// img/reverse-transaction-solidification.png
		// Builds a UTXO-DAG with `txChains` spending outputs from the corresponding chain.
		// All value objects reference the previous value object, effectively creating a chain.
		// The test attaches the prepared value objects concurrently in reverse order.

		tangle := New(mapdb.NewMapDB())

		tipManager := tipmanager.New()

		txChains := 2
		count := 10
		threads := 5
		countTotal := txChains * threads * count

		// initialize tangle with genesis block
		outputs := make(map[address.Address][]*balance.Balance)
		for i := 0; i < txChains; i++ {
			outputs[address.Random()] = []*balance.Balance{
				balance.New(balance.ColorIOTA, 1),
			}
		}
		inputIDs := loadSnapshotFromOutputs(tangle, outputs)

		transactions := make([]*transaction.Transaction, countTotal)
		valueObjects := make([]*payload.Payload, countTotal)

		// create chains of transactions
		for i := 0; i < count*threads; i++ {
			for j := 0; j < txChains; j++ {
				var tx *transaction.Transaction

				// transferring from genesis
				if i == 0 {
					tx = transaction.New(
						transaction.NewInputs(inputIDs[j]),
						transaction.NewOutputs(
							map[address.Address][]*balance.Balance{
								address.Random(): {
									balance.New(balance.ColorIOTA, 1),
								},
							}),
					)
				} else {
					// create chains in UTXO dag
					tx = transaction.New(
						getTxOutputsAsInputs(transactions[i*txChains-txChains+j]),
						transaction.NewOutputs(
							map[address.Address][]*balance.Balance{
								address.Random(): {
									balance.New(balance.ColorIOTA, 1),
								},
							}),
					)
				}

				transactions[i*txChains+j] = tx
			}
		}

		// prepare value objects (simple chain)
		for i := 0; i < countTotal; i++ {
			parent1, parent2 := tipManager.Tips()
			valueObject := payload.New(parent1, parent2, transactions[i])

			tipManager.AddTip(valueObject)
			valueObjects[i] = valueObject
		}

		// attach value objects in reverse order
		var wg sync.WaitGroup
		for thread := 0; thread < threads; thread++ {
			wg.Add(1)
			go func(threadNo int) {
				defer wg.Done()

				for i := countTotal - 1 - threadNo; i >= 0; i -= threads {
					valueObject := valueObjects[i]
					tangle.AttachPayloadSync(valueObject)
				}
			}(thread)
		}
		wg.Wait()

		// verify correctness
		for i := 0; i < countTotal; i++ {
			// check if transaction metadata is found in database
			require.Truef(t, tangle.TransactionMetadata(transactions[i].ID()).Consume(func(transactionMetadata *TransactionMetadata) {
				require.Truef(t, transactionMetadata.Solid(), "the transaction %s is not solid", transactions[i].ID().String())
				require.Equalf(t, branchmanager.MasterBranchID, transactionMetadata.BranchID(), "the transaction was booked into the wrong branch")
			}), "transaction metadata %s not found in database", transactions[i].ID())

			// check if value object metadata is found in database
			require.Truef(t, tangle.PayloadMetadata(valueObjects[i].ID()).Consume(func(payloadMetadata *PayloadMetadata) {
				require.Truef(t, payloadMetadata.IsSolid(), "the payload %s is not solid", valueObjects[i].ID())
				require.Equalf(t, branchmanager.MasterBranchID, payloadMetadata.BranchID(), "the payload was booked into the wrong branch")
			}), "value object metadata %s not found in database", valueObjects[i].ID())

			// check if outputs are found in database
			transactions[i].Outputs().ForEach(func(address address.Address, balances []*balance.Balance) bool {
				cachedOutput := tangle.TransactionOutput(transaction.NewOutputID(address, transactions[i].ID()))
				require.Truef(t, cachedOutput.Consume(func(output *Output) {
					// only the last outputs in chain should not be spent
					if i+txChains >= countTotal {
						require.Equalf(t, 0, output.ConsumerCount(), "the output should not be spent")
					} else {
						require.Equalf(t, 1, output.ConsumerCount(), "the output should be spent")
					}
					require.Equal(t, []*balance.Balance{balance.New(balance.ColorIOTA, 1)}, output.Balances())
					require.Equalf(t, branchmanager.MasterBranchID, output.BranchID(), "the output was booked into the wrong branch")
					require.Truef(t, output.Solid(), "the output is not solid")
				}), "output not found in database for tx %s", transactions[i])
				return true
			})
		}
	}
}

func getTxOutputsAsInputs(tx *transaction.Transaction) *transaction.Inputs {
	outputIDs := make([]transaction.OutputID, 0)
	tx.Outputs().ForEach(func(address address.Address, balances []*balance.Balance) bool {
		outputIDs = append(outputIDs, transaction.NewOutputID(address, tx.ID()))
		return true
	})

	return transaction.NewInputs(outputIDs...)
}
