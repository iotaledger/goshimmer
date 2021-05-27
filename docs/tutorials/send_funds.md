# How to prepare and send transactions

## Creating and sending transactions
The simplest and easiest way for creating transaction is to use ready solution, such us GUI wallets: [pollen-wallet](https://github.com/iotaledger/pollen-wallet/tree/master) and [Dr-Electron ElectricShimmer](https://github.com/Dr-Electron/ElectricShimmer)
or command line wallet [Command Line Wallet](./wallet.md). However, there is also an option to create transaction directly with Go client library, which will be main focus of this tutorial.

## Funds
To create a transaction, firstly we need to be in possession of tokens. We can receive them from other network users or request them from the faucet. For more details on how to request funds, see [tutorial](./request_funds.md).

## Preparing transaction
 A transaction is build from two main parts: a transaction essence and unlock a block. The transaction essence contains all crucial information on how many funds and where should be sent. The unlock block makes sure that only owner of the destination address will be able to spend outputs of this transaction. 

### Seed
In order to send funds we need to have private key that can unlocks our funds. If you want to use existing seed from one of your wallet, just copy the backup seed showed during wallet creation. With this  we can decode the string with `base58` library and create the `seed.Seed` instance that allows us to get addresses (`mySeed.Address()`) and their corresponding private and public keys (`mySeed.KeyPair()`).
```Go
seedBytes, _ := base58.Decode("BoDjAh57RApeqCnoGnHXBHj6wPwmnn5hwxToKX5PfFg7") // ignoring error
mySeed := walletseed.NewSeed(seedBytes)
```
Another option is to generate a completely new seed.
```Go 
mySeed := walletseed.NewSeed()
fmt.Println("My secret seed:", myWallet.Seed.String())
```
We can obtain the addresses from the seed by providing their index, in our example it is `0`. Later we will use the same index to retrieve the corresponding keys.
```Go
myAddr := mySeed.Address(0)
```

Additionally, we should make sure that unspent outputs we want to use are already confirmed.
If we use a wallet this information will be available along with balance, we can also use dashboard and look up our address. To check the confirmation status with Go use `PostAddressUnspentOutputs()` API call to get the outputs and check their inclusion state.
```Go
resp, _ := goshimAPI.PostAddressUnspentOutputs([]string{myAddr.Base58()}) // ignoring error

for _, output := range resp.UnspentOutputs[0].Outputs {
		fmt.Println("outputID:", output.Output.OutputID.Base58, "confirmed:", output.InclusionState.Confirmed)
}
```


### Transaction essence
The transaction essence can be created with: 
`NewTransactionEssence(version, timestamp, accessPledgeID, consensusPledgeID, inputs, outputs)`
 We need to provide the following arguments:
```Go
var version TransactionEssenceVersion
var timestamp time.Time
var accessPledgeID identity.ID
var consensusPledgeID identity.ID
var inputs ledgerstate.Inputs
var outputs ledgerstate.Outputs
```

#### Version and timestamp
We use `0` for a version and provide the current time as a timestamp of the transaction.
```Go
version = 0
timestamp = time.Now()
```

#### Mana pledge IDs
We also need to specify the nodeID to which we want to pledge the access and consensus mana. We can use two different nodes for each type of mana.
We can retrieve an identity instance by converting base58 encoded node ID as in the following example:
```Go
nodeIDBytes, _ := base58.Decode(base58encodedNodeID)  // ignoring error
marshalUtil := marshalutil.New(nodeIDBytes)
pledgeID, err := identity.IDFromMarshalUtil(marshalUtil)

accessPledgeID = pledgeID
consensusPledgeID = pledgeID

```
or discard mana by pledging to the empty nodeID:
```Go
accessPledgeID = identity.ID{}
consensusPledgeID = identity.ID{}
```

#### Inputs
As inputs for our transaction we need to provide unspent outputs. 
To get unspent outputs of the address 
```Go
resp, _ := goshimAPI.GetAddressUnspentOutputs(myAddr.Base58())  // ignoring error
// iterate over unspent outputs of an address
for _, output := range resp2.Outputs {
    var out ledgerstate.Output
    out, _ = output.ToLedgerstateOutput()  // ignoring error
}
```

To check the available output's balance use `Balance()` method by providing the token color:

```Go
balance, colorExist := out.Balances().Get(ledgerstate.ColorIOTA)
fmt.Println(balance, exist)
```
or iterate ofer all colors and balances:
```Go
out.Balances().ForEach(func(color ledgerstate.Color, balance uint64) bool {
			fmt.Println("Color:", color.Base58())
			fmt.Println("Balance:", balance)
			return true
		})
```

At the end we need to wrap the selected output to match the interface of the inputs:
```Go
inputs = ledgerstate.NewInputs(ledgerstate.NewUTXOInput(out))
```

#### Outputs
To create the most basic type of output use 
`ledgerstate.NewSigLockedColoredOutput()` and provide it with a balance and destination address:
```Go
balance := ledgerstate.NewColoredBalances(map[ledgerstate.Color]uint64{
							ledgerstate.ColorIOTA: uint64(1000000),
						})
output := ledgerstate.NewOutputs(ledgerstate.NewSigLockedColoredOutput(balance, destAddr.Address()))
```
The same as in case of inputs we need to adapt it with `ledgerstate.NewOutputs()` before passing to the `NewTransactionEssence` function.

### Signing transaction
After preparing the transaction essence there is a time to sign it and put the signature to the unlock block part of the transaction.
We can retrieve private and public key pairs from the seed by providing it with indexes corresponding to the addresses that holds the unspent output we want to use in our transaction.
```Go
keyPair := *mySeed.KeyPair(0)
txEssence := NewTransactionEssence(version, timestamp, accessPledgeID, consensusPledgeID, inputs, outputs)
```
We can sign the transaction in two ways: with ED25519 or BLS signature. The wallet seed library uses `ed25519` package and keys, so we will use `Sign()` method along with `ledgerstate.ED25519Signature` constructor and sign the transaction essence bytes.
Next step is to create the unlock block from our signature.

```Go
signature := ledgerstate.NewED25519Signature(kp.PublicKey, kp.PrivateKey.Sign(txEssence.Bytes())
unlockBlock := ledgerstate.NewSignatureUnlockBlock(signature)
```
Putting it all together, now we are able to create transaction with previously created transaction essence and adapted unlock block.

```Go
tx := ledgerstate.NewTransaction(txEssence, ledgerstate.UnlockBlocks{unlockBlock})
```

## Sending transaction
There are two web API methods that allows us to send the transaction:
PostTransaction() and IssuePayload(). The second one is more general method that sends the attached payload. We are going to use the first one that will additionally checks the transaction validity before issuing and wait with sending the response until the message is booked.

```Go
resp, err := goshimAPI.PostTransaction(tx.Bytes())
if err != nil {
	return
}
fmt.Println("Transaction issued, txID:", resp.TransactionID)

```
