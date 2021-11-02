---
description: The communication layer represents the base Tangle layer where so called `Messages` are gossiped around. A `Message` contains payloads, and it is up to upper layers to interpret and derive functionality out of them.
image: /img/logo/goshimmer_light.png
keywords:
- client library
- HTTP API
- message
- encoded message id
- consensus
- payload
---
# Communication Layer APIs

The communication layer represents the base Tangle layer where so called `Messages` are gossiped around. A `Message` contains payloads and it is up to upper layers to interpret and derive functionality out of them.


The API provides the following functions to interact with this primitive layer:
* [/messages/:messageID](#messagesmessageid)
* [/messages/:messageID/metadata](#messagesmessageidmetadata)
* [/data](#data)
* [/messages/payload](#messagespayload)

Client lib APIs:
* [GetMessage()](#client-lib---getmessage)
* [GetMessageMetadata()](#client-lib---getmessagemetadata)
* [Data()](#client-lib---data)
* [SendPayload()](#client-lib---sendpayload)

##  `/messages/:messageID`

Return message from the tangle.

### Parameters

| **Parameter**            | `messageID`      |
|--------------------------|----------------|
| **Required or Optional** | required       |
| **Description**          | ID of a message to retrieve   |
| **Type**                 | string         |

### Examples

#### cURL

```shell
curl --location --request GET 'http://localhost:8080/messages/:messageID'
```
where `:messageID` is the base58 encoded message ID, e.g. 4MSkwAPzGwnjCJmTfbpW4z4GRC7HZHZNS33c2JikKXJc.

#### Client lib - `GetMessage`

Messages can be retrieved via `GetMessage(base58EncodedID string) (*jsonmodels.Message, error) `

```go
message, err := goshimAPI.GetMessage(base58EncodedMessageID)
if err != nil {
    // return error
}

// will print "Hello GoShimmer World"
fmt.Println(string(message.Payload))
```

Note that we're getting actual `Message` objects from this call which represent a vertex in the communication layer Tangle. It does not matter what type of payload the message contains, meaning that this will also return messages which contain a transactions or DRNG payloads.

### Response Examples

```json
{
    "id": "4MSkwAPzGwnjCJmTfbpW4z4GRC7HZHZNS33c2JikKXJc",
    "strongParents": [
        "6LrXyDCorw8bTWKFaEmm3CZG6Nb6Ga8Bmosi1GPypGc1",
        "B89koPthm9zDx1p1fbkHwoyC1Buq896Spu3Mx1SmSete"
    ],
    "weakParents": [],
    "strongApprovers": [
        "4E4ucAA9UTTd1UC6ri4GYaS4dpzEnHPjs5gMEYhpUK8p",
        "669BRH69afQ7VfZGmNTMTeh2wnwXGKdBxtUCcRQ9CPzq"
    ],
    "weakApprovers": [],
    "issuerPublicKey": "9DB3j9cWYSuEEtkvanrzqkzCQMdH1FGv3TawJdVbDxkd",
    "issuingTime": 1621873309,
    "sequenceNumber": 4354,
    "payloadType": "GenericDataPayloadType(0)",
    "payload": "BAAAAAAAAAA=",
    "signature": "2J5XuVnmaHo54WipirWo7drJeXG3iRsnLYfzaPPuy6TXKiVBqv6ZYg2NjYP75xvgvut1SKNm8oYTchGi5t2SjyWJ"
}
```

### Results

|Return field | Type | Description|
|:-----|:------|:------|
| `id`  | `string` | Message ID. |
| `strongParents`  | `[]string` | List of strong parents' message IDs. |
| `weakParents`  | `[]string` | List of weak parents' message IDs. |
| `strongApprovers`  | `[]string` | List of strong approvers' message IDs. |
| `weakApprovers`  | `[]string` | List of weak approvers' message IDs. |
| `issuerPublicKey`  | `[]string` | Public key of issuing node. |
| `issuingTime`  | `int64` | Time this message was issued |
| `sequenceNumber`  | `uint64` | Message sequence number. |
| `payloadType`  | `string` | Payload type. |
| `payload`  | `[]byte` | The contents of the message. |
| `signature`  | `string` | Message signature. |
| `error`   | `string` | Error message. Omitted if success.    |

##  `/messages/:messageID/metadata`

Return message metadata.

### Parameters

| **Parameter**            | `messageID`      |
|--------------------------|----------------|
| **Required or Optional** | required       |
| **Description**          | ID of a message to retrieve   |
| **Type**                 | string         |


### Examples

#### cURL

```shell
curl --location --request GET 'http://localhost:8080/messages/:messageID/metadata'
```
where `:messageID` is the base58 encoded message ID, e.g. 4MSkwAPzGwnjCJmTfbpW4z4GRC7HZHZNS33c2JikKXJc.

#### Client lib - `GetMessageMetadata`

Message metadata can be retrieved via `GetMessageMetadata(base58EncodedID string) (*jsonmodels.MessageMetadata, error)`
```go
message, err := goshimAPI.GetMessageMetadata(base58EncodedMessageID)
if err != nil {
    // return error
}

// will print whether message is finalized
fmt.Println(string(message.Finalized))
```

### Response Examples

```json
{
    "id": "4MSkwAPzGwnjCJmTfbpW4z4GRC7HZHZNS33c2JikKXJc",
    "receivedTime": 1621873309,
    "solid": true,
    "solidificationTime": 1621873309,
    "structureDetails": {
        "rank": 23323,
        "pastMarkerGap": 0,
        "isPastMarker": true,
        "pastMarkers": {
            "markers": {
                "1": 21904
            },
            "highestIndex": 21904,
            "lowestIndex": 21904
        },
        "futureMarkers": {
            "markers": {
                "1": 21905
            },
            "highestIndex": 21905,
            "lowestIndex": 21905
        }
    },
    "branchID": "BranchID(MasterBranchID)",
    "scheduled": false,
    "booked": true,
    "invalid": false,
    "gradeOfFinality": 3,
    "gradeOfFinalityTime": 1621873310
}
```

### Results

|Return field | Type | Description|
|:-----|:------|:------|
| `id`  | `string` | Message ID. |
| `receivedTime`  | `int64` | Time when message was received by the node. |
| `solid`  | `bool` | Flag indicating whether the message is solid. |
| `solidificationTime`  | `int64` | Time when message was solidified by the node. |
| `structureDetails`  | `StructureDetails` | List of weak approvers' message IDs. |
| `branchID`  | `string` | Name of branch that the message is part of. |
| `scheduled`  | `bool` | Flag indicating whether the message is scheduled. |
| `booked`  | `bool` | Flag indicating whether the message is booked. |
| `eligible`  | `bool` | Flag indicating whether the message is eligible. |
| `invalid`  | `bool` | Flag indicating whether the message is invalid. |
| `finalized`  | `bool` | Flag indicating whether the message is finalized. |
| `finalizedTime`   | `string` | Time when message was finalized.    |
| `error`   | `string` | Error message. Omitted if success.    |


## `/data`

Method: `POST`

A data message is simply a `Message` containing some raw data (literally bytes). This type of message has therefore no real functionality other than that it is retrievable via `GetMessage`.

### Parameters

| **Parameter**            | `data`      |
|--------------------------|----------------|
| **Required or Optional** | required       |
| **Description**          | data bytes   |
| **Type**                 | base64 serialized bytes         |


#### Body

```json
{
  "data": "dataBytes"
}
```

### Examples

#### cURL

```shell
curl --location --request POST 'http://localhost:8080/data' \
--header 'Content-Type: application/json' \
--data-raw '{"data": "dataBytes"}'
```

#### Client lib - `Data`

##### `Data(data []byte) (string, error)`

```go
messageID, err := goshimAPI.Data([]byte("Hello GoShimmer World"))
if err != nil {
    // return error
}
```
Note that there is no need to do any additional work, since things like tip-selection, PoW and other tasks are done by the node itself.

### Response Examples

```json
{
  "id": "messageID" 
}
```

### Results

|Return field | Type | Description|
|:-----|:------|:------|
| `id`  | `string` | Message ID of the message. Omitted if error. |
| `error`   | `string` | Error message. Omitted if success.    |


## `/messages/payload`

Method: `POST`

`SendPayload()` takes a `payload` object of any type (data, transaction, drng, etc.) as a byte slice, issues a message with the given payload and returns its `messageID`. Note that the payload must be valid, otherwise an error is returned.

### Parameters

| **Parameter**            | `payload`      |
|--------------------------|----------------|
| **Required or Optional** | required       |
| **Description**          | payload bytes  |
| **Type**                 | base64 serialized bytes         |


#### Body

```json
{
  "payload": "payloadBytes"
}
```

### Examples

#### cURL

```shell
curl --location --request POST 'http://localhost:8080/messages/payload' \
--header 'Content-Type: application/json' \
--data-raw '{"payload": "payloadBytes"}'
```

#### Client lib - `SendPayload`

##### `SendPayload(payload []byte) (string, error)`

```go
helloPayload := payload.NewData([]byte{"Hello GoShimmer World!"})
messageID, err := goshimAPI.SendPayload(helloPayload.Bytes())
```

### Response Examples

```shell
{
  "id": "messageID" 
}
```

### Results

|Return field | Type | Description|
|:-----|:------|:------|
| `id`  | `string` | Message ID of the message. Omitted if error. |
| `error`   | `string` | Error message. Omitted if success.    |

Note that there is no need to do any additional work, since things like tip-selection, PoW and other tasks are done by the node itself.
