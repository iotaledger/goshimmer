---
description: The Faucet endpoint allows requesting funds from the Faucet.
image: /img/logo/goshimmer_light.png
keywords:
- client library
- HTTP API
- tokens
- funds
- address
- faucet
- testnet
- node Id
---
# Faucet API Methods

Faucet endpoint allows requesting funds from the Faucet.

The API provides the following functions and endpoints:
* [/faucet](#faucet)


Client lib APIs:
* [SendFaucetRequest()](#client-lib---sendfaucetrequest)


## `/faucet`

Method: `POST`

POST request asking for funds from the faucet to be transferred to address in the request.

### Parameters

| **Parameter**            | `address`      |
|--------------------------|----------------|
| **Required or Optional** | required       |
| **Description**          | address to pledge funds to  |
| **Type**                 | string      |



| **Parameter**            | `accessManaPledgeID`      |
|--------------------------|----------------|
| **Required or Optional** | optional       |
| **Description**          | node ID to pledge access mana to  |
| **Type**                 | string      |



| **Parameter**            | `consensusManaPledgeID`      |
|--------------------------|----------------|
| **Required or Optional** | optional       |
| **Description**          | node ID to pledge consensus mana to  |
| **Type**                 | string      |



| **Parameter**            | `powTarget`      |
|--------------------------|----------------|
| **Required or Optional** | required       |
| **Description**          | proof of the PoW being done, **only used in HTTP api** |
| **Type**                 | uint64      |



| **Parameter**            | `nonce`      |
|--------------------------|----------------|
| **Required or Optional** | required       |
| **Description**          | target Proof of Work difficulty,**only used in client lib** |
| **Type**                 | uint64      |

#### Body

```json
{
  "address": "target address",
  "accessManaPledgeID": "nodeID",
  "consensusManaPledgeID": "nodeID",
  "nonce": 50
}

```

### Examples

#### cURL

```shell
curl --location --request POST 'http://localhost:8080/faucet' \
--header 'Content-Type: application/json' \
--data-raw '{
	"address": "target address",
	"accessManaPledgeID": "nodeID",
	"consensusManaPledgeID": "nodeID",
  "nonce": 50
}'
```

#### Client lib - SendFaucetRequest

##### `SendFaucetRequest(base58EncodedAddr string, powTarget int, pledgeIDs ...string) (*jsonmodels.FaucetResponse, error)`
```go
_, err = webConnector.client.SendFaucetRequest(addr.Address().Base58(), powTarget)
if err != nil {
    // return error
}
```

### Response examples

```json
{
  "id": "4MSkwAPzGwnjCJmTfbpW4z4GRC7HZHZNS33c2JikKXJc" 
}
```

### Results

|Return field | Type | Description|
|:-----|:------|:------|
| `id`  | `string` | Message ID of the faucet request. Omitted if error. |
| `error`   | `string` | Error message. Omitted if success.    |
