# Value Layer APIs

The value layer builds on top of the communication layer. It encapsulates the functionality of token transfers, ledger representation, conflict state and consensus via FPC.

Note that the value layer operates on `Value Objects` which is a payload type which is transferred via `Messages`. A `Value Object` encapsulates the transaction spending [UTXOs](https://en.wikipedia.org/wiki/Unspent_transaction_output) to new outputs and references two other `Value Objects`. Please refer to [concepts & layers](../concepts/layers.md) for a more in depth explanation of the value layer.

The API provides multiple functions to interact with the value layer and retrieve/issue the corresponding objects described above.

#### Retrieve attachments/value objects

Attachments are `Value Objects` which reflect a vertex in the value layer Tangle. Note that a transaction can live in multiple `Value Objects` but one `Value Object` contains only one transaction.

You can retrieve all the `Value Objects` which attached a given transaction via `GetAttachments()`:
```
attachments, err := goshimAPI.GetAttachments(base58EncodedTxID)
if err != nil {
    // return error
}

// print out the parents of the value object/attachment location
fmt.Println(attachments[0].ParentID0, attachments[0].ParentID1)
```

#### Retrieve transactions

A transaction encapsulates the transfer from unspent transaction outputs to new outputs. 

You can use `GetTransactionByID` to get a given transaction:
```
tx, err := goshimAPI.GetAttachments(base58EncodedTxID)
if err != nil {
    // return error
}

// print transaction's confirmed state
inclState := tx.InclusionState
fmt.Printf("confirmation state: %v", inclState.Confirmed)

// print number of used UTXOs and created outputs
fmt.Println("inputs:", len(tx.Transaction.Inputs))
fmt.Println("outputs:", len(tx.Transaction.Outputs))
```

#### Send transactions

Sending a transaction involves first creating a transaction object defining the UTXOs to consume and the definition of the outputs. In this example, we create a transaction spending the genesis output onto a new address:

```
// create the output ID to reference as input in our transaction
genesisSeedBytes, _ := base58.Decode("7R1itJx5hVuo9w9hjg5cwKFmek4HMSoBDgJZN8hKGxih")
genesisWallet := wallet.New(genesisSeedBytes)
genesisAddr := genesisWallet.Seed().Address(0)
genesisOutputID := transaction.NewOutputID(genesisAddr, transaction.GenesisID)

// receiver
receiverWallet := wallet.New()
destAddr := receiverWallet.Seed().Address(0)

// create the transaction
tx := transaction.New(
    transaction.NewInputs(genesisOutputID),
    transaction.NewOutputs(map[address.Address][]*balance.Balance{
        destAddr: {
            {Value: genesisBalance, Color: balance.ColorIOTA},
        },
}))

// sign it
tx = tx.Sign(signaturescheme.ED25519(*genesisWallet.Seed().KeyPair(0)))

// issue it
txID, err := goshimAPI.SendTransaction(tx.Bytes())
if err != nil {
    // return error
}

// base58 transaction hash: 9TtYfSP7Y3ipQBQVWVKeunKVMyxgVfGRXLkEZ6kLR1mP
fmt.Println(txID)
```

Note that the node will perform tip-selection and therefore decide the attachment location of the `Value Object` encapsulating the transaction.

#### Retrieve UTXOs/balances

The balances of an address are made up from the UTXOs residing on that address. A balance is an amount of tokens and their corresponding color. A UTXO may contain multiple balances with different colors.

Lets retrieve the UTXO of the transaction we issued above:
```
utxos, err := goshimAPI.GetUnspentOutputs([]string{destAddr.String()})
if err != nil {
    // return error
}

// get the balance of the first UTXO
outputs := utxos.UnspentOutputs[0]
output := outputs.OutputIDs[0]
fmt.Printf("output %s has a balance of %d\n", output.ID, output.Balances[0].Value)

// UTXOs also have inclusion states
if output.InclusionState.Confirmed {
    fmt.Printf("the balance of output %s is confirmed\n", output.ID)
}

```

Note that normally you should interact with UTXOs through a higher level of abstraction since most use cases evolve around things like getting the total confirmed balance, spendable inputs etc., which can be cumbersome when  operating directly on UTXOs.

#### Coloring tokens

As an issuer of a transaction, you're free to color all the tokens of the inputs you spend to a new color of tokens. Note again, that a balance is an amount of tokens and a color. 

In order to color tokens, we declare in our output the color of the balance to be of type `balance.ColorNew`. When we declare `balance.ColorNew`, the node is instructed to set the color of the balance to be equal to the hash of the transaction in which the output is created.

Taking the example from above where we spend the genesis output, we just need to make a minor adjustment to create new colored tokens:
```
tx := transaction.New(
    transaction.NewInputs(genesisOutputID),
    transaction.NewOutputs(map[address.Address][]*balance.Balance{
        destAddr: {
            {Value: genesisBalance, Color: balance.ColorNew},
        },
}))
```

The color will then be equivalent to the transaction ID.

Note that coloring tokens doesn't increase/decrease the supply of tokens by any means. It merely colors the tokens and it is up to applications to interpret their intend.

