---
description: The ledgerstate API provides endpoints to retrieve address details, unspent outputs for an address, get conflict details, and list child conflicts amongst others.
image: /img/logo/goshimmer_light.png
keywords:
- client library
- HTTP API
- addresses
- conflicts
- outputs
- transactions
- UTXO
- unspent outputs
---
# Ledgerstate API Methods 

## HTTP APIs:

* [/ledgerstate/addresses/:address](#ledgerstateaddressesaddress)
* [/ledgerstate/addresses/:address/unspentOutputs](#ledgerstateaddressesaddressunspentoutputs)
* [/ledgerstate/conflicts/:conflictID](#ledgerstateconflictsconflictid)
* [/ledgerstate/conflicts/:conflictID/children](#ledgerstateconflictsconflictidchildren)
* [/ledgerstate/conflicts/:conflictID/conflicts](#ledgerstateconflictsconflictidconflicts)
* [/ledgerstate/conflicts/:conflictID/voters](#ledgerstateconflictsconflictidvoters)
* [/ledgerstate/outputs/:outputID](#ledgerstateoutputsoutputid)
* [/ledgerstate/outputs/:outputID/consumers](#ledgerstateoutputsoutputidconsumers)
* [/ledgerstate/outputs/:outputID/metadata](#ledgerstateoutputsoutputidmetadata)
* [/ledgerstate/transactions/:transactionID](#ledgerstatetransactionstransactionid)
* [/ledgerstate/transactions/:transactionID/metadata](#ledgerstatetransactionstransactionidmetadata)
* [/ledgerstate/transactions/:transactionID/attachments](#ledgerstatetransactionstransactionidattachments)
* [/ledgerstate/transactions](#ledgerstatetransactions)
* [/ledgerstate/addresses/unspentOutputs](#ledgerstateaddressesunspentoutputs)


## Client Lib APIs:

* [GetAddressOutputs()](#client-lib---getaddressoutputs)
* [GetAddressUnspentOutputs()](#client-lib---getaddressunspentoutputs)
* [GetConflict()](#client-lib---getconflict)
* [GetConflictChildren()](#client-lib---getconflictchildren)
* [GetConflictConflicts()](#client-lib---getconflictconflicts)
* [GetConflictVoters()](#client-lib---getconflictvoters)
* [GetOutput()](#client-lib---getoutput)
* [GetOutputConsumers()](#client-lib---getoutputconsumers)
* [GetOutputMetadata()](#client-lib---getoutputmetadata)
* [GetTransaction()](#client-lib---gettransaction)
* [GetTransactionMetadata()](#client-lib---gettransactionmetadata)
* [GetTransactionAttachments()](#client-lib---gettransactionattachments)
* [PostTransaction()](#client-lib---posttransaction)
* [PostAddressUnspentOutputs()](#client-lib---postaddressunspentoutputs)

## `/ledgerstate/addresses/:address`

Get address details for a given base58 encoded address ID, such as output types and balances. For the client library API call balances will not be directly available as values because they are stored as a raw block. Balance can be read after retrieving `ledgerstate.Output` instance, as presented in the examples.

### Parameters
| **Parameter**            | `address`      |
|--------------------------|----------------|
| **Required or Optional** | required       |
| **Description**          | The address encoded in base58. |
| **Type**                 | string         |

### Examples

#### cURL

```shell
curl http://localhost:8080/ledgerstate/addresses/:address \
-X GET \
-H 'Content-Type: application/json'
```

where `:address` is the base58 encoded address, e.g. 6PQqFcwarCVbEMxWFeAqj7YswK842dMtf84qGyKqVH7s1kK.

#### Client lib - `GetAddressOutputs()`
```Go
resp, err := goshimAPI.GetAddressOutputs("6PQqFcwarCVbEMxWFeAqj7YswK842dMtf84qGyKqVH7s1kK")
if err != nil {
    // return error
}
fmt.Println("output address: ", resp.Address)

for _, output := range resp.Outputs {
    fmt.Println("outputID: ", output.OutputID)
    fmt.Println("output type: ", output.Type)
    // get output instance
    out, err = output.ToLedgerstateOutput()
}
```

### Response Examples
```json
{
    "address": {
        "type": "AddressTypeED25519",
        "base58": "18LhfKUkWt4M9YR6Q3au4LT8wWCERwzHaqn153K78Eixp"
    },
    "outputs": [
        {
            "outputID": {
                "base58": "gdFXAjwsm5kDeGdcZsJAShJLeunZmaKEMmfHSdoX34ZeSs",
                "transactionID": "32yHjeZpghKNkybd2iHjXj7NsUdR63StbJcBioPGAut3",
                "outputIndex": 0
            },
            "type": "SigLockedColoredOutputType",
            "output": {
                "balances": {
                    "11111111111111111111111111111111": 1000000
                },
                "address": "18LhfKUkWt4M9YR6Q3au4LT8wWCERwzHaqn153K78Eixp"
            }
        }
    ]
}
```

### Results
|Return field | Type | Description|
|:-----|:------|:------|
| `address`  | Address | The address corresponding to provided outputID.   |
| `outputs`   | Output | List of transactions' outputs.     |

#### Type `Address`

|Field | Type | Description|
|:-----|:------|:------|
| `type`  | string | The type of an address.   |
| `base58`| string   | The address encoded with base58.          |

#### Type `Output`

|Field | Type | Description|
|:-----|:------|:------|
| `outputID`  | OutputID | The identifier of an output.   |
| `outputType`   | string | The type of the output.     |
| `output` | string | An output raw block containing balances and corresponding addresses. |

#### Type `OutputID`

|Field | Type | Description|
|:-----|:------|:------|
| `base58`  | string | The output identifier encoded with base58.    |
| `transactionID`   | string | The transaction identifier encoded with base58.     |
| `outputIndex`   | int | The index of an output.     |



## `/ledgerstate/addresses/:address/unspentOutputs`
Gets list of all unspent outputs for the address based on a given base58 encoded address ID.

### Parameters

| **Parameter**            | `address`      |
|--------------------------|----------------|
| **Required or Optional** | required       |
| **Description**          | The address encoded in base58. |
| **Type**                 | string         |
### Examples

#### cURL

```shell
curl http://localhost:8080/ledgerstate/addresses/:address/unspentOutputs \
-X GET \
-H 'Content-Type: application/json'
```

where `:address` is the base58 encoded address, e.g. 6PQqFcwarCVbEMxWFeAqj7YswK842dMtf84qGyKqVH7s1kK.

#### Client lib - `GetAddressUnspentOutputs()`

```Go
address := "6PQqFcwarCVbEMxWFeAqj7YswK842dMtf84qGyKqVH7s1kK"
resp, err := goshimAPI.GetAddressUnspentOutputs(address)
if err != nil {
    // return error
}
fmt.Println("output address: ", resp.Address)

for _, output := range resp.Outputs {
    fmt.Println("outputID: ", output.OutputID)
    fmt.Println("output type: ", output.Type)
    // get output instance
    out, err = output.ToLedgerstateOutput()
}
```
### Response Examples
```json
{
    "address": {
        "type": "AddressTypeED25519",
        "base58": "18LhfKUkWt4M9YR6Q3au4LT8wWCERwzHaqn153K78Eixp"
    },
    "outputs": [
        {
            "outputID": {
                "base58": "gdFXAjwsm5kDeGdcZsJAShJLeunZmaKEMmfHSdoX34ZeSs",
                "transactionID": "32yHjeZpghKNkybd2iHjXj7NsUdR63StbJcBioPGAut3",
                "outputIndex": 0
            },
            "type": "SigLockedColoredOutputType",
            "output": {
                "balances": {
                    "11111111111111111111111111111111": 1000000
                },
                "address": "18LhfKUkWt4M9YR6Q3au4LT8wWCERwzHaqn153K78Eixp"
            }
        }
    ]
}
```

### Results
|Return field | Type | Description|
|:-----|:------|:------|
| `address`  | Address | The address corresponding to provided unspent outputID.   |
| `outputs`   | Output | List of transactions' unspent outputs.     |

#### Type `Address`

|Field | Type | Description|
|:-----|:------|:------|
| `type`  | string | The type of an address.   |
| `base58`| string   | The address encoded with base58.          |

#### Type `Output`

|Field | Type | Description|
|:-----|:------|:------|
| `outputID`  | OutputID | The identifier of an output.   |
| `outputType`   | string | The type of the output.    |
| `output` | string | An output raw block containing balances and corresponding addresses |

#### Type `OutputID`

|Field | Type | Description|
|:-----|:------|:------|
| `base58`  | string | The output identifier encoded with base58.    |
| `transactionID`   | string | The transaction identifier encoded with base58.     |
| `outputIndex`   | int | The index of an output.     |



## `/ledgerstate/conflicts/:conflictID`
Gets a conflict details for a given base58 encoded conflict ID.

### Parameters

| **Parameter**            | `conflictID`      |
|--------------------------|----------------|
| **Required or Optional** | required       |
| **Description**          | The conflict ID encoded in base58. |
| **Type**                 | string         |

### Examples

#### cURL

```shell
curl http://localhost:8080/ledgerstate/conflicts/:conflictID \
-X GET \
-H 'Content-Type: application/json'
```

where `:conflictID` is the ID of the conflict, e.g. 2e2EU6fhxRhrXVnYQ6US4zmUkE5YJip25ecafn8gZeoZ.

#### Client lib - `GetConflict()`
```Go
resp, err := goshimAPI.GetConflict("2e2EU6fhxRhrXVnYQ6US4zmUkE5YJip25ecafn8gZeoZ")
if err != nil {
    // return error
}
fmt.Println("conflict ID: ", resp.ID)
fmt.Println("conflict type: ", resp.Type)
fmt.Println("conflict inclusion state: ", resp.ConfirmationState)
fmt.Println("conflict parents IDs: ", resp.Parents)
fmt.Println("conflict conflicts IDs: ", resp.ConflictIDs)
fmt.Printf("liked: %v, finalized: %v, monotonically liked: %v", resp.Liked, resp.Finalized, resp.MonotonicallyLiked)
```
### Response Examples
```json
{
    "id": "5v6iyxKUSSF73yoZa6YngNN5tqoX8hJQWKGXrgcz3XTg",
    "type": "ConflictConflictType",
    "parents": [
        "4uQeVj5tqViQh7yWWGStvkEG1Zmhx6uasJtWCJziofM"
    ],
    "conflictIDs": [
        "3LrHecDf8kvDGZKTAYaKmvdsqXA18YBc8A9UePu7pCxw5ks"
    ],
    "liked": false,
    "monotonicallyLiked": false,
    "finalized": false,
    "confirmationState": "ConfirmationState(Pending)"
}
```

### Results
|Return field | Type | Description                                             |
|:-----|:------|:--------------------------------------------------------|
| `id`  | string | The conflict identifier encoded with base58.              |
| `type`        | string | The type of the conflict.                                 |
| `parents`     | []string | The list of parent conflicts IDs.                        |
| `conflictIDs` | []string | The list of conflicts identifiers.                      |
| `liked`       | bool | The boolean indicator if conflict is liked.               |
| `monotonicallyLiked`   | bool | The boolean indicator if conflict is monotonically liked. |
| `finalized`   | bool | The boolean indicator if conflict is finalized.           |
| `confirmationState`   | string | Confirmation state of a conflict.                         |



## `/ledgerstate/conflicts/:conflictID/children`
Gets a list of all child conflicts for a conflict with given base58 encoded conflict ID.

### Parameters

| **Parameter**            | `conflictID`      |
|--------------------------|----------------|
| **Required or Optional** | required       |
| **Description**          | The conflict ID encoded in base58. |
| **Type**                 | string         |


### Examples

#### cURL

```shell
curl http://localhost:8080/ledgerstate/conflicts/:conflictID/children \
-X GET \
-H 'Content-Type: application/json'
```

where `:conflictID` is the ID of the conflict, e.g. 2e2EU6fhxRhrXVnYQ6US4zmUkE5YJip25ecafn8gZeoZ.

#### Client lib - `GetConflictChildren()`
```Go
resp, err := goshimAPI.GetConflictChildren("2e2EU6fhxRhrXVnYQ6US4zmUkE5YJip25ecafn8gZeoZ")
if err != nil {
    //return error
}
fmt.Printf("All children conflicts for conflict %s:\n", resp.ConflictID)
for _, conflict := range resp.ChildConflicts {
    fmt.Println("conflictID: ", conflict.ConflictID)
    fmt.Printf("type: %s\n", conflict.ConflictID)
}
```

### Response Examples
```json
{
    "conflictID": "HuYUAwCeexmBePNXx5rNeJX1zUvUdUUs5LvmRmWe7HCV",
    "childConflicts": [
        {
            "conflictID": "4SdXm5NXEcVogiJNEKkecqd5rZzRYeGYBj8oBNsdX91W",
            "type": "AggregatedConflictType"
        }
    ]
}
```

### Results

|Return field | Type | Description|
|:-----|:------|:------|
| `conflictID`  | string | The conflict identifier encoded with base58.   |
| `childConflicts`        | []ChildConflict | The child conflicts data.  |


#### Type `ChildConflict`

|Field | Type | Description|
|:-----|:------|:------|
| `conflictID`  | string | The conflict identifier encoded with base58.   |
| `type`        | string | The type of the conflict.  |



## `/ledgerstate/conflicts/:conflictID/conflicts`
Get all conflicts for a given conflict ID, their outputs and conflicting conflicts.

### Parameters

| **Parameter**            | `conflictID`      |
|--------------------------|----------------|
| **Required or Optional** | required       |
| **Description**          | The conflicting conflict ID encoded in base58. |
| **Type**                 | string         |


### Examples

#### cURL

```shell
curl http://localhost:8080/ledgerstate/conflicts/:conflictID/conflicts \
-X GET \
-H 'Content-Type: application/json'
```

where `:conflictID` is the ID of the conflict, e.g. 2e2EU6fhxRhrXVnYQ6US4zmUkE5YJip25ecafn8gZeoZ.

#### Client lib - `GetConflictConflicts()`
```Go
resp, err := goshimAPI.GetConflictConflicts("2e2EU6fhxRhrXVnYQ6US4zmUkE5YJip25ecafn8gZeoZ")
if err != nil {
    // return error
}
fmt.Printf("All conflicts for conflict %s:\n", resp.ConflictID)
// iterate over all conflicts
for _, conflict := range resp.Conflicts {
    fmt.Println("output ID: ", conflict.OutputID.Base58)
    fmt.Println("conflicting transaction ID: ", conflict.OutputID.TransactionID)
    fmt.Printf("related conflicts: %v\n", conflict.ConflictIDs)
}
```
### Response Examples
```json
{
    "conflictID": "HuYUAwCeexmBePNXx5rNeJX1zUvUdUUs5LvmRmWe7HCV",
    "conflicts": [
        {
            "outputID": {
                "base58": "41GvDSQnd12e4nWnd2WzmdLmffruXqsE46jgeUbnB8s1QnK",
                "transactionID": "9wr21zza46Y5QonKEHNQ6x8puA7Rbq5LAbsQZJCK1g1g",
                "outputIndex": 0
            },
            "conflictIDs": [
                "b8QRhHerfg14cYQ4VFD7Fyh1HYTCbjt9aK1XJmdoXwq",
                "HuYUAwCeexmBePNXx5rNeJX1zUvUdUUs5LvmRmWe7HCV"
            ]
        }
    ]
}
```

### Results

|Return field | Type | Description|
|:-----|:------|:------|
| `conflictID`  | string | The conflict identifier encoded with base58.   |
| `conflicts` | []Conflict | The conflict data.  |

#### Type `Conflict`
|Field | Type | Description|
|:-----|:------|:------|
| `outputID`  | OutputID | The conflict identifier encoded with base58.   |
| `conflictIDs` | []string | The identifiers of all related conflicts encoded in base58.  |

#### Type `OutputID`

|Field | Type | Description|
|:-----|:------|:------|
| `base58`  | string | The output identifier encoded with base58.    |
| `transactionID`   | string | The transaction identifier encoded with base58.     |
| `outputIndex`   | int | The index of an output.     |


## `/ledgerstate/conflicts/:conflictID/voters`
Get a list of voters of a given conflictID.

| **Parameter**            | `conflictID`     |
|--------------------------|----------------|
| **Required or Optional** | required       |
| **Description**          | The conflict ID encoded in base58. |
| **Type**                 | string         |

### Examples

### cURL

```shell
curl http://localhost:8080/ledgerstate/conflicts/:conflictID/voters \
-X GET \
-H 'Content-Type: application/json'
```
where `:conflictID` is the ID of the conflict, e.g. 2e2EU6fhxRhrXVnYQ6US4zmUkE5YJip25ecafn8gZeoZ.

#### Client lib - `GetConflictVoters()`
```Go
resp, err := goshimAPI.GetConflictVoters("2e2EU6fhxRhrXVnYQ6US4zmUkE5YJip25ecafn8gZeoZ")
if err != nil {
    // return error
}
fmt.Printf("All voters for conflict %s:\n", resp.ConflictID)
// iterate over all voters
for _, voter := range resp.Voters {
    fmt.Println("ID: ", voter)
}
```

### Response examples
```json
{
  "conflictID": "HuYUAwCeexmBePNXx5rNeJX1zUvUdUUs5LvmRmWe7HCV",
  "voters": ["b8QRhHerfg14cYQ4VFD7Fyh1HYTCbjt9aK1XJmdoXwq","41GvDSQnd12e4nWnd2WzmdLmffruXqsE46jgeUbnB8s1QnK"]
}
```

### Results
|Return field | Type | Description|
|:-----|:------|:------|
| `conflictID`   | string    | The conflict identifier encoded with base58.   |
| `voters` | [] string | The list of conflict voter IDs  |


## `/ledgerstate/outputs/:outputID`
Get an output details for a given base58 encoded output ID, such as output types, addresses, and their corresponding balances.
For the client library API call balances will not be directly available as values because they are stored as a raw block. 

### Parameters

| **Parameter**            | `outputID`      |
|--------------------------|----------------|
| **Required or Optional** | required       |
| **Description**          | The output ID encoded in base58. |
| **Type**                 | string         |


### Examples

#### cURL

```shell
curl http://localhost:8080/ledgerstate/outputs/:outputID \
-X GET \
-H 'Content-Type: application/json'
```

where `:outputID` is the ID of the output, e.g. 41GvDSQnd12e4nWnd2WzmdLmffruXqsE46jgeUbnB8s1QnK.

#### Client lib - `GetOutput()`
```Go
resp, err := goshimAPI.GetOutput("41GvDSQnd12e4nWnd2WzmdLmffruXqsE46jgeUbnB8s1QnK")
if err != nil {
    // return error
}
fmt.Println("outputID: ", resp.OutputID.Base58)
fmt.Println("output type: ", resp.Type)
fmt.Println("transactionID: ", resp.OutputID.TransactionID)
```
### Response Examples
```json
{
    "outputID": {
        "base58": "41GvDSQnd12e4nWnd2WzmdLmffruXqsE46jgeUbnB8s1QnK",
        "transactionID": "9wr21zza46Y5QonKEHNQ6x8puA7Rbq5LAbsQZJCK1g1g",
        "outputIndex": 0
    },
    "type": "SigLockedColoredOutputType",
    "output": {
        "balances": {
            "11111111111111111111111111111111": 1000000
        },
        "address": "1F95a2yceDicNLvqod6P3GLFZDAFdwizcTTYow4Y1G3tt"
    }
}
```

### Results
|Return field | Type | Description|
|:-----|:------|:------|
| `outputID`  | OutputID | The identifier of an output.   |
| `outputType`   | string | The type of the output.     |
| `output` | string | An output raw block containing balances and corresponding addresses |

#### Type `OutputID`

|Field | Type | Description|
|:-----|:------|:------|
| `base58`  | string | The output identifier encoded with base58.    |
| `transactionID`   | string | The transaction identifier encoded with base58.     |
| `outputIndex`   | int | The index of an output.     |



## `/ledgerstate/outputs/:outputID/consumers`
Get a list of consumers based on a provided base58 encoded output ID. Transactions that contains the output and information about its validity.

### Parameters

| **Parameter**            | `outputID`      |
|--------------------------|----------------|
| **Required or Optional** | required       |
| **Description**          | The output ID encoded in base58. |
| **Type**                 | string         |

### Examples

#### cURL

```shell
curl http://localhost:8080/ledgerstate/outputs/:outputID/consumers \
-X GET \
-H 'Content-Type: application/json'
```

where `:outputID` is the ID of the output, e.g. 41GvDSQnd12e4nWnd2WzmdLmffruXqsE46jgeUbnB8s1QnK.

#### Client lib - `GetOutputConsumers()`
```Go
resp, err := goshimAPI.GetOutputConsumers("41GvDSQnd12e4nWnd2WzmdLmffruXqsE46jgeUbnB8s1QnK")
if err != nil {
    // return error
}
fmt.Println("outputID: ", resp.OutputID.Base58)
// iterate over output consumers
for _, consumer := range resp.Consumers {
    fmt.Println("transactionID: ", consumer.TransactionID)
    fmt.Println("valid: ", consumer.Valid)
}
```
### Response Examples
```json
{
    "outputID": {
        "base58": "41GvDSQnd12e4nWnd2WzmdLmffruXqsE46jgeUbnB8s1QnK",
        "transactionID": "9wr21zza46Y5QonKEHNQ6x8puA7Rbq5LAbsQZJCK1g1g",
        "outputIndex": 0
    },
    "consumers": [
        {
            "transactionID": "b8QRhHerfg14cYQ4VFD7Fyh1HYTCbjt9aK1XJmdoXwq",
            "valid": "true"
        },
        {
            "transactionID": "HuYUAwCeexmBePNXx5rNeJX1zUvUdUUs5LvmRmWe7HCV",
            "valid": "true"
        }
    ]
}
```

### Results
|Return field | Type | Description|
|:-----|:------|:------|
| `outputID`  | OutputID | The output identifier encoded with base58.   |
| `consumers`        | []Consumer | Consumers of the requested output. |


#### Type `OutputID`

|Field | Type | Description|
|:-----|:------|:------|
| `base58`  | string | The output identifier encoded with base58.    |
| `transactionID`   | string | The transaction identifier encoded with base58.     |
| `outputIndex`   | int | The index of an output.     |

#### Type `Consumers`

|Field | Type | Description|
|:-----|:------|:------|
| `transactionID`  | string | The transaction identifier encoded with base58.    |
| `valid`   | string | The boolean indicator if the transaction is valid.   |



## `/ledgerstate/outputs/:outputID/metadata`
Gets an output metadata for a given base58 encoded output ID.

### Parameters

| **Parameter**            | `outputID`      |
|--------------------------|----------------|
| **Required or Optional** | required       |
| **Description**          | The output ID encoded in base58. |
| **Type**                 | string         |

### Examples

#### cURL

```shell
curl http://localhost:8080/ledgerstate/outputs/:outputID/metadata \
-X GET \
-H 'Content-Type: application/json'

```

where `:outputID` is the ID of the output, e.g. 41GvDSQnd12e4nWnd2WzmdLmffruXqsE46jgeUbnB8s1QnK.

#### Client lib - `GetOutputMetadata()`
```Go
resp, err := goshimAPI.GetOutputMetadata("41GvDSQnd12e4nWnd2WzmdLmffruXqsE46jgeUbnB8s1QnK")
if err != nil {
    // return error
}
fmt.Printf("Metadata of an output %s:\n", resp.OutputID.Base58)
fmt.Println("conflictID: ", resp.ConflictID)
fmt.Println("first consumer: ", resp.FirstConsumer)
fmt.Println("number of consumers: ", resp.ConsumerCount)
fmt.Printf("finalized: %v, solid: %v\n", resp.Finalized, resp.Solid)
fmt.Println("solidification time: ",  time.Unix(resp.SolidificationTime, 0))
```
### Response Examples
```json
{
    "outputID": {
        "base58": "41GvDSQnd12e4nWnd2WzmdLmffruXqsE46jgeUbnB8s1QnK",
        "transactionID": "9wr21zza46Y5QonKEHNQ6x8puA7Rbq5LAbsQZJCK1g1g",
        "outputIndex": 0
    },
    "conflictID": "4uQeVj5tqViQh7yWWGStvkEG1Zmhx6uasJtWCJziofM",
    "solid": true,
    "solidificationTime": 1621889327,
    "consumerCount": 2,
    "firstConsumer": "HuYUAwCeexmBePNXx5rNeJX1zUvUdUUs5LvmRmWe7HCV",
    "finalized": true
}
```

### Results

|Return field | Type | Description|
|:-----|:------|:------|
| `outputID`            | OutputID  | The output identifier encoded with base58.   |
| `conflictID`            | string    | The identifier of the conflict encoded with base58. |
| `solid`               | bool      | The boolean indicator if the block is solid. |
| `solidificationTime`  | int64     | The time of solidification of a block. |
| `consumerCount`       | int       | The number of consumers. |
| `firstConsumer`       | string    | The first consumer of the output. |
| `finalized`           | bool      | The boolean indicator if the transaction is finalized. |


#### Type `OutputID`

|Field | Type | Description|
|:-----|:------|:------|
| `base58`  | string | The output identifier encoded with base58.    |
| `transactionID`   | string | The transaction identifier encoded with base58.     |
| `outputIndex`   | int | The index of an output.     |



## `/ledgerstate/transactions/:transactionID`
Gets a transaction details for a given base58 encoded transaction ID.

### Parameters
| **Parameter**            | `transactionID`      |
|--------------------------|----------------|
| **Required or Optional** | required       |
| **Description**          | The transaction ID encoded in base58. |
| **Type**                 | string         |

### Examples

#### cURL

```shell
curl http://localhost:8080/ledgerstate/transactions/:transactionID \
-X GET \
-H 'Content-Type: application/json'
```

where `:transactionID` is the ID of the conflict, e.g. HuYUAwCeexmBePNXx5rNeJX1zUvUdUUs5LvmRmWe7HCV.

#### Client lib - `GetTransaction()`
```Go
resp, err := goshimAPI.GetTransaction("41GvDSQnd12e4nWnd2WzmdLmffruXqsE46jgeUbnB8s1QnK")
if err != nil {
    // return error
}
fmt.Println("transaction inputs:")
for _, input := range resp.Inputs {
    fmt.Println("inputID:", input.ReferencedOutputID.Base58)
}
fmt.Println("transaction outputs:")
for _, output := range resp.Outputs{
    fmt.Println("outputID:", output.OutputID.Base58)
    fmt.Println("output type:", output.Type)
}
fmt.Println("access mana pledgeID:", resp.AccessPledgeID)
fmt.Println("consensus mana pledgeID:", resp.ConsensusPledgeID)
```
### Response Examples
```json
{
    "version": 0,
    "timestamp": 1621889348,
    "accessPledgeID": "DsHT39ZmwAGrKQe7F2rAjwHseUnJeY89gDPEH1FJxYdH",
    "consensusPledgeID": "DsHT39ZmwAGrKQe7F2rAjwHseUnJeY89gDPEH1FJxYdH",
    "inputs": [
        {
            "type": "UTXOInputType",
            "referencedOutputID": {
                "base58": "41GvDSQnd12e4nWnd2WzmdLmffruXqsE46jgeUbnB8s1QnK",
                "transactionID": "9wr21zza46Y5QonKEHNQ6x8puA7Rbq5LAbsQZJCK1g1g",
                "outputIndex": 0
            }
        }
    ],
    "outputs": [
        {
            "outputID": {
                "base58": "6gMWUCgJDozmyLeGzW3ibGFicEq2wbhsxgAw8rUVPvn9bj5",
                "transactionID": "HuYUAwCeexmBePNXx5rNeJX1zUvUdUUs5LvmRmWe7HCV",
                "outputIndex": 0
            },
            "type": "SigLockedColoredOutputType",
            "output": {
                "balances": {
                    "11111111111111111111111111111111": 1000000
                },
                "address": "1HrUn1jWAjrMU58LLdFhfnWBwUKVdWjP5ojp7oCL9mVWs"
            }
        }
    ],
    "unlockBlocks": [
        {
            "type": "SignatureUnlockBlockType",
            "publicKey": "12vNcfgRHLSsobeqZFrjFRcVAmFQbDVniguPnEoxmkbG",
            "signature": "4isq3qzhY4MwbSeYM2NgRn5noWAyh5rqD12ruiTQ7P89TfXNecwHZ5nbpDc4UB7md1bkfM1xYtSh18FwLqK8HAC6"
        }
    ],
    "dataPayload": ""
}
```

### Results
|Return field | Type | Description|
|:-----|:------|:------|
| `version`         | uint8  | The version of the transaction essence.   |
| `timestamp`       | int64    | The issuing time of the transaction. |
| `accessPledgeID`  | string      | The node ID indicating to which node pledge the access mana. |
| `consensusPledgeID`| string     |  The node ID indicating to which node pledge the consensus mana. |
| `inputs`          | []Input       | The inputs of the transaction. |
| `outputs`         | []Output    | The outputs of the transaction. |
| `unlockBlocks`    | []UnlockBlock      | The unlock block containing signatures unlocking the inputs or references to previous unlock blocks. |
| `dataPayload` | []byte      | The raw data payload that can be attached to the transaction. |

#### Type `Input `
|Field | Type | Description|
|:-----|:------|:------|
| `Type`  | string | The type of input.   |
| `ReferencedOutputID`   | ReferencedOutputID | The output ID that is used as an input for the transaction.     |

#### Type `ReferencedOutputID`

|Field | Type | Description|
|:-----|:------|:------|
| `base58`  | string | The referenced output identifier encoded with base58.    |
| `transactionID`   | string | The transaction identifier encoded with base58.     |
| `outputIndex`   | int | The index of a referenced output.     |

#### Type `Output`

|Field | Type | Description|
|:-----|:------|:------|
| `outputID`  | OutputID | The identifier of an output.   |
| `outputType`   | string | The type of the output.  |
| `output` | string | An output raw block containing balances and corresponding addresses. |

#### Type `OutputID`

|Field | Type | Description|
|:-----|:------|:------|
| `base58`  | string | The output identifier encoded with base58.    |
| `transactionID`   | string | The transaction identifier encoded with base58.     |
| `outputIndex`   | int | The index of an output.     |

#### Type `UnlockBlock`
|Field | Type | Description|
|:-----|:------|:------|
| `type`  | string |  The unlock block type: signature or reference. |
| `referencedIndex`  | uint16 |  The reference index of an unlock block. |
| `signatureType`  | uint8 |  The unlock block signature type: ED25519 or BLS. |
| `publicKey`  | string |  The public key of a transaction owner. |
| `signature`  | string | The string representation of a signature encoded with base58 signed over a transaction essence. |



## `/ledgerstate/transactions/:transactionID/metadata`
Gets a transaction metadata for a given base58 encoded transaction ID.

### Parameters
| **Parameter**            | `transactionID`      |
|--------------------------|----------------|
| **Required or Optional** | required       |
| **Description**          | The transaction ID encoded in base58. |
| **Type**                 | string         |

### Examples

#### cURL

```shell
curl http://localhost:8080/ledgerstate/transactions/:transactionID/metadata \
-X GET \
-H 'Content-Type: application/json'
```

where `:transactionID` is the ID of the conflict, e.g. HuYUAwCeexmBePNXx5rNeJX1zUvUdUUs5LvmRmWe7HCV.

#### Client lib - `GetTransactionMetadata()`
```Go
resp, err := goshimAPI.GetTransactionMetadata("41GvDSQnd12e4nWnd2WzmdLmffruXqsE46jgeUbnB8s1QnK")
if err != nil {
    // return error
}
fmt.Println("transactionID:", resp.TransactionID)
fmt.Println("conflictID:", resp.ConflictID)
fmt.Printf("conflict lazy booked: %v, solid: %v, finalized: %v\n", resp.LazyBooked, resp.Solid, resp.Finalized)
fmt.Println("solidification time:",  time.Unix(resp.SolidificationTime, 0))
```
### Response Examples
```json
{
    "transactionID": "HuYUAwCeexmBePNXx5rNeJX1zUvUdUUs5LvmRmWe7HCV",
    "conflictID": "HuYUAwCeexmBePNXx5rNeJX1zUvUdUUs5LvmRmWe7HCV",
    "solid": true,
    "solidificationTime": 1621889358,
    "finalized": true,
    "lazyBooked": false
}
```
### Results
|Return field | Type | Description|
|:-----|:------|:------|
| `transactionID`         | string  | The transaction identifier encoded with base58.    |
| `conflictID`       | string    |  The conflict identifier of the transaction. |
| `solid`  | bool      | The boolean indicator if the transaction is solid. |
| `solidificationTime`          | uint64      | The time of solidification of the transaction. |
| `finalized`         | bool    | The boolean indicator if the transaction is finalized. |
| `lazyBooked`    | bool      | The boolean indicator if the transaction is lazily booked.|


## `/ledgerstate/transactions/:transactionID/attachments`
Gets the list of blocks IDs with attachments of the base58 encoded transaction ID.

### Parameters
| **Parameter**            | `transactionID`      |
|--------------------------|----------------|
| **Required or Optional** | required       |
| **Description**          | The transaction ID encoded in base58. |
| **Type**                 | string         |

### Examples

#### cURL

```shell
curl http://localhost:8080/ledgerstate/transactions/:transactionID/attachments \
-X GET \
-H 'Content-Type: application/json'
```

where `:transactionID` is the ID of the conflict, e.g. HuYUAwCeexmBePNXx5rNeJX1zUvUdUUs5LvmRmWe7HCV.

#### Client lib - `GetTransactionAttachments()`
```Go
resp, err := goshimAPI.GetTransactionAttachments("DNSN8GaCeep6CVuUV6KXAabXkL3bv4PUP4NkTNKoZMqS")
if err != nil {
    // return error
}
fmt.Printf("Blocks IDs containing transaction %s:\n", resp.TransactionID)
for _, blkID := range resp.BlockIDs {
    fmt.Println(blkID)
}
```
### Response Examples
```json
{
    "transactionID": "HuYUAwCeexmBePNXx5rNeJX1zUvUdUUs5LvmRmWe7HCV",
    "blockIDs": [
        "J1FQdMcticXiiuKMbjobq4zrYGHagk2mtTzkVwbqPgSq"
    ]
}
```

### Results
|Return field | Type | Description|
|:-----|:------|:------|
| `transactionID`   | string  | The transaction identifier encoded with base58.  |
| `blockIDs`       | []string    | The blocks IDs that contains the requested transaction. |



## `/ledgerstate/transactions`
Sends transaction provided in form of a binary data, validates transaction before issuing the block payload. For more detail on how to prepare transaction bytes see the [tutorial](../tutorials/send_transaction.md).

### Examples

#### Client lib - `PostTransaction()`
```GO
// prepare tx essence and signatures
...
// create transaction
tx := ledgerstate.NewTransaction(txEssence, ledgerstate.UnlockBlocks{unlockBlock})
resp, err := goshimAPI.PostTransaction(tx.Bytes())
if err != nil {
    // return error
}
fmt.Println("Transaction sent, txID: ", resp.TransactionID)
```

### Results
|Return field | Type | Description|
|:-----|:------|:------|
| `transactionID`   | string  | The transaction identifier encoded with base58.  |
| `Error`   | error  | The error returned if transaction was not processed correctly, otherwise is nil.  |



## `/ledgerstate/addresses/unspentOutputs`
Gets all unspent outputs for a list of addresses that were sent in the body block.  Returns the unspent outputs along with inclusion state and metadata for the wallet. 

### Request Body
```json
{
    "addresses": [
        "18LhfKUkWt4M9YR6Q3au4LT8wWCERwzHaqn153K78Eixp"
    ]
}
```

### Examples

#### cURL

```shell
curl http://localhost:8080/ledgerstate/addresses/unspentOutputs \
-X POST \
-H 'Content-Type: application/json'
--data-raw '{"addresses": ["18LhfKUkWt4M9YR6Q3au4LT8wWCERwzHaqn153K78Eixp"]}'
```

#### Client lib - `PostAddressUnspentOutputs()`
```Go
resp, err := goshimAPI.PostAddressUnspentOutputs([]string{"H36sZQkopfoEzP3WCMThSjUv5v9MLVYuaQ73tsKgVzXo"})
if err != nil {
    return
}
for _, outputs := range resp.UnspentOutputs {
    fmt.Println("address ID:", outputs.Address.Base58)
    fmt.Println("address type:", outputs.Address.Type)

    for _, output := range outputs.Outputs {
        fmt.Println("output ID:", output.Output.OutputID.Base58)
        fmt.Println("output type:", output.Output.Type)
    }
}
```

### Response Examples

```json
{
    "unspentOutputs": [
        {
            "address": {
                "type": "AddressTypeED25519",
                "base58": "1Z4t5KEKU65fbeQCbNdztYTB1B4Cdxys1XRzTFrmvAf3"
            },
            "outputs": [
                {
                    "output": {
                        "outputID": {
                            "base58": "4eGoQWG7UDtBGK89vENQ5Ea1N1b8xF26VD2F8nigFqgyx5m",
                            "transactionID": "BqzgVk4yY9PDZuDro2mvT36U52ZYbJDfM41Xng3yWoQK",
                            "outputIndex": 0
                        },
                        "type": "SigLockedColoredOutputType",
                        "output": {
                            "balances": {
                                "11111111111111111111111111111111": 1000000
                            },
                            "address": "1Z4t5KEKU65fbeQCbNdztYTB1B4Cdxys1XRzTFrmvAf3"
                        }
                    },
                    "confirmationState": {
                        "confirmed": true,
                        "rejected": false,
                        "conflicting": false
                    },
                    "metadata": {
                        "timestamp": "2021-05-25T15:47:04.50470213+02:00"
                    }
                }
            ]
        }
    ]
}
```
### Results

|Return field | Type | Description|
|:-----|:------|:------|
| `unspentOutputs`   | WalletOutputsOnAddress  | Unspent outputs representation for wallet.  |

#### Type `WalletOutputsOnAddress`

|Return field | Type | Description|
|:-----|:------|:------|
| `Address`   | Address  | The address corresponding to the unspent output.  |
| `Outputs`   | []WalletOutput  | Unspent outputs representation for wallet.  |

#### Type `Address`

|Field | Type | Description|
|:-----|:------|:------|
| `type`  | string | The type of an address.   |
| `base58`| string   | The address encoded with base58.          |

#### Type `WalletOutput`

|Field | Type | Description|
|:-----|:------|:------|
| `output`  | Output | The unspent output.   |
| `confirmationState`| ConfirmationState   | The inclusion state of the transaction containing the output.  |
| `metadata`| WalletOutputMetadata   | The metadata of the output for the wallet lib.  |

#### Type `Output`

|Field | Type | Description|
|:-----|:------|:------|
| `outputID`  | OutputID | The identifier of an output.   |
| `outputType`   | string |  The type of the output.     |
| `output` | string | An outputs raw block containing balances and corresponding addresses |

#### Type `OutputID`

|Field | Type | Description|
|:-----|:------|:------|
| `base58`  | string | The output identifier encoded with base58.    |
| `transactionID`   | string | The transaction identifier encoded with base58.     |
| `outputIndex`   | int | The index of an output.     |

#### Type `ConfirmationState`

|Field | Type | Description|
|:-----|:------|:------|
| `confirmed`  | bool |  The boolean indicating if the transaction containing the output is confirmed. |
| `rejected`   | bool | The boolean indicating if the transaction that contains the output was rejected and is booked to the rejected conflict.   |
| `conflicting`   | bool | The boolean indicating if the output is in conflicting transaction.|

#### Type `WalletOutputMetadata`

|Field | Type | Description|
|:-----|:------|:------|
| `timestamp`  | time.Time | The timestamp of the transaction containing the output.    |
