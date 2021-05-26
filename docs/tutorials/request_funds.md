# How to obtain tokens from the faucet

## The faucet dApp
The faucet is a dApp built on top of the [value and communication layer](../concepts/layers.md). It sends IOTA tokens to addresses by listening to faucet request messages. A faucet message is a Message containing an address encoded in Base58, you can retieve the faucet message and check your balances via API methods.

## Obtain tokens from the faucet
There are 3 ways to send a faucet request message to obtain IOTA tokens:
1. Via the Go client library
2. Via the HTTP API directly
3. Via the wallet

### Via the Go client library
Follow the instructions in [Use the API](../apis/api.md) to set up the API instance. 

Example:
```go
// provide your Base58 encoded destination address
messageID, err := goshimAPI.SendFaucetRequest("JaMauTaTSVBNc13edCCvBK9fZxZ1KKW5fXegT1B7N9jY")

---- or

// get the given address from your wallet instance and 
// use String() to get its Base58 representation
connector := wallet.GenericConnector(wallet.NewWebConnector("http://localhost:8080"))
//TODO add options and explain what they are
addr := wallet.New(connector).Seed().Address(0)
messageID, err := goshimAPI.SendFaucetRequest(addr.String(), 22)
```

### Via the HTTP API directly
The URI for POSTing faucet request messages is `http://<host>:<web-api-port>/faucet`. Refer to [faucet API methods](../apis/faucet) for more details.


| Parameter | Required | Description | Type    |
| --------- | -------- | ----------- | --- |
| `address`      | Yes     | Destination address to which to send tokens to encoded in Base58        | string     |

cURL example:
```
curl http://localhost:8080/faucet \
-X POST \
-H 'Content-Type: application/json' \
-d '{
  "address": "JaMauTaTSVBNc13edCCvBK9fZxZ1KKW5fXegT1B7N9jY"
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


