# How to obtain tokens from the faucet

## The faucet dApp
The faucet is a dApp built on top of the [value and communication layer](../concepts/layers.md). It sends IOTA tokens to addresses by listening to faucet request messages. A faucet message is a Message containing an address encoded in Base58, and it is retrievable via [`FindMessageByID()`](../apis/communication.md).
After sending a faucet request message, you can check your balances via [`GetUnspentOutputs()`](../apis/value.md).

## Obtain tokens from the faucet
There are 3 ways to send a faucet request message to obtain IOTA tokens:
1. Via the Go client library
2. Via the HTTP API directly
3. Requesting tokens via the GoShimmer web dashboard
4. Via the wallet

### Via the Go client library
Follow the instructions in [Use the API](../apis/api.md) to set up the API instance. 

Example:
```
// provide your Base58 encoded destination address
messageID, err := goshimAPI.SendFaucetRequest("JaMauTaTSVBNc13edCCvBK9fZxZ1KKW5fXegT1B7N9jY")

---- or

// get the given address from your wallet instance and 
// use String() to get its Base58 representation
addr := wallet.Seed().Address(0)
messageID, err := goshimAPI.SendFaucetRequest(addr.String())
```

### Via the HTTP API directly
The URI for POSTing faucet request messages is `http://<host>:<web-api-port>/faucet`


| Parameter | Required | Description | Type    |
| --------- | -------- | ----------- | --- |
| `address`      | Yes     | Destination address to which to send tokens to encoded in Base58        | string     |

cURL example:
```
curl http://localhost:8080 \
-X POST \
-H 'Content-Type: application/json' \
-d '{
  "address": "JaMauTaTSVBNc13edCCvBK9fZxZ1KKW5fXegT1B7N9jY"
}'
```

### Requesting tokens via the GoShimmer web dashboard
You can send the faucet request message via the faucet tab on the dashboard by filling in a Base58 encoded address to receive tokens.

You can then use the link provided below to check the funds on your supplied address.

<img src="https://user-images.githubusercontent.com/11289354/85510396-d9127000-b629-11ea-8d5c-d5bb10d7c34a.png" width="650">

### Via the wallet
Currently, there are two GUI wallets to use, one from the community member [Dr-Electron ElectricShimmer](https://github.com/Dr-Electron/ElectricShimmer) and another from the foundation [pollen-wallet](https://github.com/iotaledger/pollen-wallet/tree/master). You can request funds from the faucet with these two implementations.

As for pollen-wallet, follow the instructions in [pollen-wallet](https://github.com/iotaledger/pollen-wallet/tree/master) to build and execute the wallet.

You can request funds by pressing the `Request Funds` in the wallet.

**Note**: You need to create a wallet first before requesting funds.

<img src="https://user-images.githubusercontent.com/11289354/88524828-70edea00-d02c-11ea-9a01-d7e1a8b7bdfd.png" height="450">


This may take a while to receive funds:

<img src="https://user-images.githubusercontent.com/11289354/88525200-e0fc7000-d02c-11ea-9f7f-a545cf14b318.png" width="450">

When the faucet request is successful, you can check the received balances:

<img src="https://user-images.githubusercontent.com/11289354/88525478-38024500-d02d-11ea-92c7-25c80eb6a947.png" width="450">


