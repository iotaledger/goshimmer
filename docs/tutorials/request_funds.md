# How to obtain tokens from the faucet

## The faucet dApp
The faucet is a dApp built on top of the [value and communication layer](../concepts/layers.md). It sends IOTA tokens to addresses by listening to faucet request messages. A faucet message is a Message containing a special payload with an address encoded in Base58, the aManaPledgeID, the cManaPledgeID and a nonce as a proof that some Proof Of Work has been computed. The PoW is just a way to rate limit and avoid abuse of the Faucet. The Faucet has an additional protection by means of granting request to a given address only once. That means that, in order to receive funds from the Faucet multuple times, the address must be different.

After sending a faucet request message, you can check your balances via [`GetAddressUnspentOutputs()`](../apis/ledgerstate.md).

## Obtain tokens from the faucet
There are 3 ways to send a faucet request message to obtain IOTA tokens:
1. Via the Go client library
2. Via the HTTP API directly
3. Via the wallet

### Via the Go client library
Follow the instructions in [Use the API](../apis/api.md) to set up the API instance. 

Example:
```go
// provide your Base58 encoded destination address,
// the proof of work difficulty,
// the optional aManaPledgeID (Base58 encoded),
// the optional cManaPledgeID (Base58 encoded)
messageID, err := goshimAPI.SendFaucetRequest("JaMauTaTSVBNc13edCCvBK9fZxZ1KKW5fXegT1B7N9jY", 22, "2GtxMQD94KvDH1SJPJV7icxofkyV1njuUZKtsqKmtux5", "2GtxMQD94KvDH1SJPJV7icxofkyV1njuUZKtsqKmtux5")

---- or

// get the given address from your wallet instance and 
// use String() to get its Base58 representation
// the proof of work difficulty,
// the optional aManaPledgeID (Base58 encoded),
// the optional cManaPledgeID (Base58 encoded)
addr := wallet.Seed().Address(0)
messageID, err := goshimAPI.SendFaucetRequest(addr.String(), 22, "2GtxMQD94KvDH1SJPJV7icxofkyV1njuUZKtsqKmtux5", "2GtxMQD94KvDH1SJPJV7icxofkyV1njuUZKtsqKmtux5")
```

### Via the HTTP API directly
The URI for POSTing faucet request messages is `http://<host>:<web-api-port>/faucet`. Refer to [faucet API methods](../apis/faucet) for more details.


| Parameter | Required | Description | Type    |
| --------- | -------- | ----------- | --- |
| `address`      | Yes     | Destination address to which to send tokens to encoded in Base58        | string     |
| `accessManaPledgeID`      | No     | Node ID to pledge access mana to encoded in Base58        | string     |
| `consensusManaPledgeID`      | No     | Node ID to pledge consensus mana to encoded in Base58        | string     |
| `nonce`      | Yes     | Proof of the PoW being done       | uint64     |

cURL example:
```
curl http://localhost:8080/faucet \
-X POST \
-H 'Content-Type: application/json' \
-d '{
  "address": "JaMauTaTSVBNc13edCCvBK9fZxZ1KKW5fXegT1B7N9jY",
  "accessManaPledgeID": "2GtxMQD94KvDH1SJPJV7icxofkyV1njuUZKtsqKmtux5",
  "consensusManaPledgeID": "2GtxMQD94KvDH1SJPJV7icxofkyV1njuUZKtsqKmtux5",
  "nonce": 24578
}'
```

### Via the wallet
Currently, there is one cli-wallet that you can refer to the tutorial [Command Line Wallet
](./wallet.md) and two GUI wallets to use. One from the community member [Dr-Electron ElectricShimmer](https://github.com/Dr-Electron/ElectricShimmer) and another from the foundation [pollen-wallet](https://github.com/iotaledger/pollen-wallet/tree/master). You can request funds from the faucet with these two implementations.

As for pollen-wallet, follow the instructions in [pollen-wallet](https://github.com/iotaledger/pollen-wallet/tree/master) to build and execute the wallet, or download executable file directly in [pollen wallet release](https://github.com/iotaledger/pollen-wallet/releases).

You can request funds by pressing the `Request Funds` in the wallet.

**Note**: You need to create a wallet first before requesting funds.

<img src="https://user-images.githubusercontent.com/11289354/88524828-70edea00-d02c-11ea-9a01-d7e1a8b7bdfd.png" height="450">


This may take a while to receive funds:

<img src="https://user-images.githubusercontent.com/11289354/88525200-e0fc7000-d02c-11ea-9f7f-a545cf14b318.png" width="450">

When the faucet request is successful, you can check the received balances:

<img src="https://user-images.githubusercontent.com/11289354/88525478-38024500-d02d-11ea-92c7-25c80eb6a947.png" width="450">

