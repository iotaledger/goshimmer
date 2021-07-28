# How To Send Transaction

The simplest way you can send and receive transactions is to use ready solution, such us GUI wallets: [pollen-wallet](https://github.com/iotaledger/pollen-wallet/tree/master) and [Dr-Electron ElectricShimmer](https://github.com/Dr-Electron/ElectricShimmer), 
or a command line wallet [Command Line Wallet](wallet_library.md). However, you can also create a transaction directly with Go client library, which will be main focus of this tutorial.

## Funds

To create a transaction, you will need to have some tokens. You can receive them from other network users, or request them from the faucet. You can find more details on requesting funds in the [obtain tokens  tutorial](obtain_tokens.md).

## Preparing a Transaction

A transaction consists of two parts: 
- Transaction essence which contains, among other information, the amount, origin and destination where the funds should be sent 
- Unlock blocks which make sure that only the owner of the transferred funds can successfully perform this transaction.

### Seed

In order to send funds you need to have a private key that can be used to prove that you own the funds, and can unlock them. If you want to use an existing seed from one of your wallets, you can use the backup seed showed during wallet creation. With this, you can decode the string with the `base58` library and create the `seed.Seed` instance. That will allow you to retrieve the wallet addresses (`mySeed.Address()`), and the corresponding private and public keys (`mySeed.KeyPair()`) as shown in the following example.

```go
seedBytes, _ := base58.Decode("BoDjAh57RApeqCnoGnHXBHj6wPwmnn5hwxToKX5PfFg7") // ignoring error
mySeed := walletseed.NewSeed(seedBytes)
```

Another option is to generate a completely new seed and addresses as shown in the following snippet.

```go 
mySeed := walletseed.NewSeed()
fmt.Println("My secret seed:", myWallet.Seed.String())
```

You can obtain the addresses from the seed by providing their index, in the last example it is `0`. Later you will use the same index to retrieve the corresponding keys.

```go
myAddr := mySeed.Address(0)
```

Additionally, you should make sure that the unspent outputs you want to use are confirmed.
If we use a wallet, this information will be available along with the wallet balance. You can also use the dashboard and look up your address in the explorer. You can also check the confirmation status with Go using the `PostAddressUnspentOutputs()` API method to get the outputs and check their inclusion state.

```go
resp, _ := goshimAPI.PostAddressUnspentOutputs([]string{myAddr.Base58()}) // ignoring error
for _, output := range resp.UnspentOutputs[0].Outputs {
		fmt.Println("outputID:", output.Output.OutputID.Base58, "confirmed:", output.InclusionState.Confirmed)
}
```


### Transaction Essence

You can create the transaction essence can be created with by running the 
`NewTransactionEssence(version, timestamp, accessPledgeID, consensusPledgeID, inputs, outputs)` function.
You will need to provide the following arguments:

```go
var version TransactionEssenceVersion
var timestamp time.Time
var accessPledgeID identity.ID
var consensusPledgeID identity.ID
var inputs ledgerstate.Inputs
var outputs ledgerstate.Outputs
```

#### Version and timestamp

You can use `0` for a version, and provide the current time as a timestamp of the transaction.

```go
version = 0
timestamp = time.Now()
```

#### Mana pledge IDs

You will also need to specify the nodeID to which you want to pledge the access and consensus mana. You can use two different nodes for each type of mana.

You can retrieve an identity instance by converting base58 encoded node ID as in the following example:

```go
pledgeID, err := mana.IDFromStr(base58encodedNodeID)
accessPledgeID = pledgeID
consensusPledgeID = pledgeID
```

Alternatively, you can discard the mana by pledging it to the empty nodeID:

```go
accessPledgeID = identity.ID{}
consensusPledgeID = identity.ID{}
```

#### Inputs

You need to provide unspent outputs as inputs for the transaction .

You can use the following example to get unspent outputs of the address. 

```go
resp, _ := goshimAPI.GetAddressUnspentOutputs(myAddr.Base58())  // ignoring error
// iterate over unspent outputs of an address
for _, output := range resp2.Outputs {
    var out ledgerstate.Output
    out, _ = output.ToLedgerstateOutput()  // ignoring error
}
```

You can check the available output's balance  and provide the token color using the `Balances()` method. You can use the default IOTA color as shown in the following snippet.

```go
balance, colorExist := out.Balances().Get(ledgerstate.ColorIOTA)
fmt.Println(balance, exist)
```

Alternatively, you can iterate over all colors and balances as shown in the following snippet.

```go
out.Balances().ForEach(func(color ledgerstate.Color, balance uint64) bool {
			fmt.Println("Color:", color.Base58())
			fmt.Println("Balance:", balance)
			return true
		})
```

At the end of the process, you will need wrap the selected output to match the inputs' interface as shown in the following snippet.

```go
inputs = ledgerstate.NewInputs(ledgerstate.NewUTXOInput(out))
```

#### Outputs

You can create the most basic type of output using the `ledgerstate.NewSigLockedColoredOutput()` method. You should send a balance and destination address as arguments. It is important to provide the correct balance value. The total balance with the same color has to be equal for input and output.

```go
balance := ledgerstate.NewColoredBalances(map[ledgerstate.Color]uint64{
							ledgerstate.ColorIOTA: uint64(100),
						})
outputs := ledgerstate.NewOutputs(ledgerstate.NewSigLockedColoredOutput(balance, destAddr.Address()))
```

As in case of inputs,  you will need to adapt it with `ledgerstate.NewOutputs()` before passing to the `NewTransactionEssence` function.

### Signing a Transaction

After preparing the transaction essence, you should sign it and use the signature to create the transaction's unlock block.

You can retrieve private and public key pairs from the seed by providing it with indexes corresponding to the addresses that hold the unspent output that you want to use in your transaction. 

```go
kp := *mySeed.KeyPair(0)
txEssence := NewTransactionEssence(version, timestamp, accessPledgeID, consensusPledgeID, inputs, outputs)
```

You can sign the transaction with `ED25519` or `BLS` signature. The wallet seed library uses the `ed25519` package and keys, so you can use `Sign()` method along with the `ledgerstate.ED25519Signature` constructor to sign the transaction essence bytes.

The next step is to create the unlock block from our signature.

```go
signature := ledgerstate.NewED25519Signature(kp.PublicKey, kp.PrivateKey.Sign(txEssence.Bytes())
unlockBlock := ledgerstate.NewSignatureUnlockBlock(signature)
```

We are now able to create transaction with previously created transaction essence, and the adapted unlock block.

```go
tx := ledgerstate.NewTransaction(txEssence, ledgerstate.UnlockBlocks{unlockBlock})
```

## Sending a Transaction

There are two web API methods you can use to send the transaction:

- `IssuePayload()`
- `PostTransaction()`

`IssuePayload()` is a more general method that sends the attached payload. 

`IssuePayload()` will additionally check the transaction validity before issuing it, and will wait to send the response until the message is booked. The method accepts a byte array, so you will need to call `Bytes()`.

If the transaction is booked without any problems, you should be able to get the transaction ID from the API response.

```go
resp, err := goshimAPI.PostTransaction(tx.Bytes())
if err != nil {
	return
}
fmt.Println("Transaction issued, txID:", resp.TransactionID)
```

## Code Examples

### Creating the Transaction

Constructing a new `ledgerstate.Transaction`. 

```go
import (
	"fmt"
	"net/http"
	"time"

	"github.com/iotaledger/goshimmer/client"
	walletseed "github.com/iotaledger/goshimmer/client/wallet/packages/seed"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/mana"
)

func buildTransaction() (tx *ledgerstate.Transaction, err error) {
	// node to pledge access mana.
	accessManaPledgeIDBase58 := "2GtxMQD94KvDH1SJPJV7icxofkyV1njuUZKtsqKmtux5"
	accessManaPledgeID, err := mana.IDFromStr(accessManaPledgeIDBase58)
	if err != nil {
		return
	}

	// node to pledge consensus mana.
	consensusManaPledgeIDBase58 := "1HzrfXXWhaKbENGadwEnAiEKkQ2Gquo26maDNTMFvLdE3"
	consensusManaPledgeID, err := mana.IDFromStr(consensusManaPledgeIDBase58)
	if err != nil {
		return
	}
     
        /**
        N.B to pledge mana to the node issuing the transaction, use empty pledgeIDs.
        emptyID := identity.ID{}
        accessManaPledgeID, consensusManaPledgeID := emptyID, emptyID
        **/      

	// destination address.
	destAddressBase58 := "your_base58_encoded_address"
	destAddress, err := ledgerstate.AddressFromBase58EncodedString(destAddressBase58)
	if err != nil {
		return
	}

	// output to consume.
	outputIDBase58 := "your_base58_encoded_outputID"
	out, err := ledgerstate.OutputIDFromBase58(outputIDBase58)
	if err != nil {
		return
	}
	inputs := ledgerstate.NewInputs(ledgerstate.NewUTXOInput(out))

	// UTXO output.
	output := ledgerstate.NewSigLockedColoredOutput(ledgerstate.NewColoredBalances(map[ledgerstate.Color]uint64{
		ledgerstate.ColorIOTA: uint64(1337),
	}), destAddress)
	outputs := ledgerstate.NewOutputs(output)

	// build tx essence.
	txEssence := ledgerstate.NewTransactionEssence(0, time.Now(), accessManaPledgeID, consensusManaPledgeID, inputs, outputs)

	// sign.
	seed := walletseed.NewSeed([]byte("your_seed"))
	kp := seed.KeyPair(0)
	sig := ledgerstate.NewED25519Signature(kp.PublicKey, kp.PrivateKey.Sign(txEssence.Bytes()))
	unlockBlock := ledgerstate.NewSignatureUnlockBlock(sig)

	// build tx.
	tx = ledgerstate.NewTransaction(txEssence, ledgerstate.UnlockBlocks{unlockBlock})
	return
}
```

### Post the Transaction

There are 2 available options to post the created transaction.

 - [GoShimmer client lib](#post-via-client-lib)
 - [Web API](#post-via-web-api)
 
#### Post via client lib

You can use the following example to post a transaction using the client library.

```go
func postTransactionViaClientLib() (res string , err error) {
	// connect to goshimmer node
	goshimmerClient := client.NewGoShimmerAPI("http://127.0.0.1:8080", client.WithHTTPClient(http.Client{Timeout: 60 * time.Second}))

	// build tx from previous step
	tx, err := buildTransaction()
	if err != nil {
		return
	}

	// send the tx payload.
	res, err = goshimmerClient.PostTransaction(tx.Bytes())
	if err != nil {
		return
	}
	return
}
```

#### Post via web API

You can use the following example to post a transaction using the web library.

1. Get the transaction bytes.

    ```go
    // build tx from previous step
    tx, err := buildTransaction()
    if err != nil {
        return
    }
    bytes := tx.Bytes()
    
    // print bytes
    fmt.Println(string(bytes))
    ```

2. Post the bytes.

    ```shell script
    curl --location --request POST 'http://localhost:8080/ledgerstate/transactions' \
    --header 'Content-Type: application/json' \
    --data-raw '{
        "tx_bytes": "bytes..."
    }'
    ```
