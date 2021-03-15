> This guide is meant for developers familiar with the Go programming language.

GoShimmer ships with a client Go library which communicates with the HTTP API. Please refer to the [godoc.org docs](https://godoc.org/github.com/iotaledger/goshimmer/client) for function/struct documentation. There is also a set of APIs which do not directly have anything to do with the different layers. Since they are so simple, simply extract their usage from the GoDocs.

# Use the API

Simply `go get` the lib via:
```
go get github.com/iotaledger/goshimmer/client
```

Init the API by passing in the API URI of your GoShimmer node:
```
goshimAPI := client.NewGoShimmerAPI("http://mynode:8080")
```

Optionally, define your own `http.Client` to use, in order for example to define custom timeouts:
```
goshimAPI := client.NewGoShimmerAPI("http://mynode:8080", http.Client{Timeout: 30 * time.Second})
```

#### A note about errors

The API issues HTTP calls to the defined GoShimmer node. Non 200 HTTP OK status codes will reflect themselves as `error` in the returned arguments. Meaning that for example calling for attachments with a non existing/available transaction on a node, will return an `error` from the respective function. (There might be exceptions to this rule)

# Communication Layer APIs

The communication layer represents the base Tangle layer where so called `Messages` are gossiped around. A `Message` contains payloads and it is up to upper layers to interpret and derive functionality out of them.

The API provides three functions to interact with this primitive layer:
* `Data(data []byte) (string, error)`
* `FindMessageByID(base58EncodedIDs []string) (*webapi_message.Response, error)`
* `SendPayload(payload []byte) (string, error)`

#### Issuing a data message
A data message is simply a `Message` containing some raw data (literally bytes). This type of message has therefore no real functionality other than that it is retrievable via `FindMessageByID`.

Example:
```
messageID, err := goshimAPI.Data([]byte("Hello GoShimmer World"))
```

Note that there is no need to do any additional work, since things like tip-selection, PoW and other tasks are done by the node itself.

#### Retrieve messages

Of course messages can then be retrieved via `FindMessageByID()`
```
foundMsgs, err := goshimAPI.FindMessageByID([]string{base58EncodedMessageID})
if err != nil {
    // return error
}

// this might be nil if the message wasn't available
message := foundMsgs[0]
if message == nil {
    // return error
}

// will print "Hello GoShimmer World"
fmt.Println(string(message.Payload))
```

Note that we're getting actual `Message` objects from this call which represent a vertex in the communication layer Tangle. It does not matter what type of payload the message contains, meaning that `FindMessageByID` will also return messages which contain value objects or DRNG payloads.

#### Send Payload
`SendPayload()` takes a `payload` object of any type (data, value, drng, etc.) as a byte slice, issues a message with the given payload and returns its `messageID`. Note, that the payload must be valid, otherwise an error is returned.

Example:
```go
helloPayload := payload.NewData([]byte{"Hello Goshimmer World!"})
messageID, err := goshimAPI.SendPayload(helloPayload.Bytes())
```


# Value Layer APIs

The value layer builds on top of the communication layer. It encapsulates the functionality of token transfers, ledger representation, conflict state and consensus via FPC.

Note that the value layer operates on `Value Objects` which is a payload type which is transferred via `Messages`. A `Value Object` encapsulates the transaction spending [UTXOs](https://en.wikipedia.org/wiki/Unspent_transaction_output) to new outputs and references two other `Value Objects`. Please refer to [concepts & layers](https://github.com/iotaledger/goshimmer/wiki/Concepts-&-Layers) for a more in depth explanation of the value layer.

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

# Mana APIs

#### Get Own Mana
You can get the access and consensus mana of the node this API client is communicating with.

```
manas, err := goshimAPI.GetOwnMana()
if err != nil {
    // return error
}

// print the node ID
fmt.Println("full ID: ", manas.NodeID, "short ID: ", manas.ShortNodeID)

// get access mana of the node
fmt.Println("access mana: ", manas.Access, "access mana updated time: ", manas.AccessTimestamp)

// get consensus mana of the node
fmt.Println("consensus mana: ", manas.Consensus, "consensus mana updated time: ", manas.ConsensusTimestamp)

```

#### Get Mana of a node with its full nodeID
You can get the access and consensus mana of a node with its full nodeID.
**Note**: Both full ID and short ID are Base58 strings, the short ID only contains the first 8 bytes of the identity.

```
manas, err := goshimAPI.GetManaFullNodeID("2GtxMQD94KvDH1SJPJV7icxofkyV1njuUZKtsqKmtux5")
if err != nil {
    // return error
}

// print the node ID
fmt.Println("full ID: ", manas.NodeID, "short ID: ", manas.ShortNodeID)

// get access mana of the node
fmt.Println("access mana: ", manas.Access, "access mana updated time: ", manas.AccessTimestamp)

// get consensus mana of the node
fmt.Println("consensus mana: ", manas.Consensus, "consensus mana updated time: ", manas.ConsensusTimestamp)

```

#### Get Mana of a node with its short nodeID
You can get the access and consensus mana of a node with its short nodeID.

```
manas, err := goshimAPI.GetMana("4AeXyZ26e4G")
if err != nil {
    // return error
}

// print short node ID
fmt.Println("short ID: ", manas.ShortNodeID)

// get access mana of the node
fmt.Println("access mana: ", manas.Access)

// get consensus mana of the node
fmt.Println("consensus mana: ", manas.Consensus)

```

#### Get Mana of every node
Get the mana perception of the node in the network.
You can retrieve the full/short node ID, consensus mana, access mana of each node, and the mana updated time.

```
manas, err := goshimAPI.GetAllMana()
if err != nil {
    // return error
}

// mana updated time
fmt.Println("access mana updated time: ", manas.AccessTimestamp)
fmt.Println("consensus mana updated time: ", manas.ConsensusTimestamp)

// get access mana of each node
for _, m := range manas.Access {
    fmt.Println("full node ID: ", m.NodeID, "short node ID:", m.ShortNodeID, "access mana: ", m.Mana)
}

// get consensus mana of each node
for _, m := range manas.Consensus {
    fmt.Println("full node ID: ", m.NodeID, "short node ID:", m.ShortNodeID, "consensus mana: ", m.Mana)
}
```

#### Get Mana percentile
To learn the top percentile the node belongs to relative to the network in terms of mana.
The input should be a full node ID.

```
mana, err := goshimAPI.GetManaPercentile("2GtxMQD94KvDH1SJPJV7icxofkyV1njuUZKtsqKmtux5")
if err != nil {
    // return error
}

// mana updated time
fmt.Println("access mana percentile: ", mana.Access, "access mana updated time: ", manas.AccessTimestamp)
fmt.Println("consensus mana percentile: ", mana.Consensus, "consensus mana updated time: ", manas.ConsensusTimestamp)
```

#### Get online access/consensus mana of nodes
You can get a sorted list of online access/consensus mana of nodes, sorted from the highest mana to the lowest.
The highest mana node has OnlineRank 1, and increases 1 by 1 for the following nodes. 

```
// online access mana
accessMana, err := goshimAPI.GetOnlineAccessMana()
if err != nil {
    // return error
}

for _, m := accessMana.Online {
    fmt.Println("full node ID: ", m.ID, "mana rank: ", m.OnlineRank, "access mana: ", m.Mana)
}

// online consensus mana
consensusMana, err := goshimAPI.GetOnlineConsensusMana()
if err != nil {
    // return error
}

for _, m := consensusMana.Online {
    fmt.Println("full node ID: ", m.ID, "mana rank: ", m.OnlineRank, "consensus mana: ", m.Mana)
}

```

#### Get the top N highest access/consensus Mana nodes
You can get the N highest access/consensus mana holders in the network, sorted in descending order.

```
// get the top 5 highest access mana nodes
accessMana, err := goshimAPI.GetNHighestAccessMana(5)
if err != nil {
    // return error
}

for _, m := accessMana.Nodes {
    fmt.Println("full node ID: ", m.NodeID, "access mana: ", m.Mana)
}

// get the top 5 highest consensus mana nodes
consensusMana, err := goshimAPI.GetNHighestConsensusMana(5)
if err != nil {
    // return error
}

for _, m := consensusMana.Nodes {
    fmt.Println("full node ID: ", m.ID, "consensus mana: ", m.Mana)
}

```

#### Get the pending mana of an output
You can get the mana (bm2) that will be pledged by a given output.

```
res, err := goshimAPI.GetPending("4a5KkxVfsdFVbf1NBGeGTCjP8Ppsje4YFQg9bu5YGNMSJK1")
if err != nil {
    // return error
}

// get the amount of mana
fmt.Println("mana be pledged: ", res.Mana)
fmt.Println("the timestamp of the output (decay duration)", res.Timestamp)
```

#### Get the consensus Mana vector
Get the consensus base mana vector of a time (int64) in the past.

```
res, err := goshimAPI.GetPastConsensusManaVector(1614924295)
if err != nil {
    // return error
}

// the mana vector of each node
for _, m := range res.Consensus {
    fmt.Println("node ID:", m.NodeID, "consensus mana: ", m.Mana)
}
```
#### Get consensus event logs
Get the consensus event logs of the given node IDs.

```
res, err := goshimAPI.GetConsensusEventLogs([]string{"2GtxMQD94KvDH1SJPJV7icxofkyV1njuUZKtsqKmtux5"})
if err != nil {
    // return error
}

for nodeID, e := range res.Logs {
    fmt.Println("node ID:", nodeID)
    
    // pledge logs
    for _, p := e.Pledge {
        fmt.Println("mana type: ", p.ManaType)
        fmt.Println("node ID: ", p.NodeID)
        fmt.Println("time: ", p.Time)
        fmt.Println("transaction ID: ", p.TxID)
        fmt.Println("mana amount: ", p.Amount)
    }

    // revoke logs
    for _, r := e.Revoke {
        fmt.Println("mana type: ", r.ManaType)
        fmt.Println("node ID: ", r.NodeID)
        fmt.Println("time: ", r.Time)
        fmt.Println("transaction ID: ", r.TxID)
        fmt.Println("mana amount: ", r.Amount)
        fmt.Println("input ID: ", r.InputID)
    }
}
```

#### Get a list of allowed mana pledge nodes
This returns the list of allowed mana pledge node IDs.

```
res, err := goshimAPI.GetAllowedManaPledgeNodeIDs()
if err != nil {
    // return error
}

// print the list of nodes that access mana is allowed to be pledged to
for _, id := range res.Access.Allowed {
    fmt.Println("node ID:", id)
}

// print the list of nodes that consensus mana is allowed to be pledged to
for _, id := range res.Consensus.Allowed {
    fmt.Println("node ID:", id)
}
```