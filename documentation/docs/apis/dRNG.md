---
description: The dRNG APIs provide methods to retrieve basic info about dRNG committees and randomness as well as to broadcast collective randomness beacon.
image: /img/logo/goshimmer_light.png
keywords:
- client library
- HTTP API
- drng
- info
- committee
- randomness
- collective beacon
---
# dRNG API Methods

The dRNG APIs provide methods to retrieve basic info about dRNG committees and randomness as well as to broadcast collective randomness beacon.

HTTP APIs:

* [/drng/collectiveBeacon](#drngcollectivebeacon)
* [/drng/info/committee](#drnginfocommittee)
* [/drng/info/randomness](#drnginforandomness)

Client lib APIs:

* [BroadcastCollectiveBeacon()](#client-lib---broadcastcollectivebeacon)
* [GetRandomness()](#client-lib---getrandomness)
* [GetCommittee()](#client-lib---getcommittee)


## `/drng/collectiveBeacon`

Method: `POST`

Sends the given collective beacon (payload) by creating a message in the backend.

### Parameters

| **Parameter**            | `payload`      |
|--------------------------|----------------|
| **Required or Optional** | required       |
| **Description**          | collective beacon payload   |
| **Type**                 | base64 serialized bytes         |


#### Body

```json
{
  "payload": "collectiveBeaconBytes"
}
```

### Examples

#### cURL

```shell
curl --location --request POST 'http://localhost:8080/drng/collectiveBeacon' \
--header 'Content-Type: application/json' \
--data-raw '{"payload": "collectiveBeaconBytes"}'
```

#### Client lib - `BroadcastCollectiveBeacon`

Collective beacon can be broadcast using `BroadcastCollectiveBeacon(payload []byte) (string, error)`.

```go
msgId, err := goshimAPI.BroadcastCollectiveBeacon(payload)
if err != nil {
    // return error
}
```

### Response example

```json
{
  "id": "messageID"
}
```

### Results

|Return field | Type | Description|
|:-----|:------|:------|
| `id`  | `string` | Message ID of beacon message. Omitted if error. |
| `error`   | `string` | Error message. Omitted if success.    |


## `/drng/info/committee`

Returns the current dRNG committee used.

### Parameters

None.

### Examples

#### cURL

```shell
curl http://localhost:8080/drng/info/committee
```

#### Client lib - `GetCommittee`

Available committees can be retrieved using `GetCommittee() (*jsonmodels.CommitteeResponse, error)`.

```go
committees, err := goshimAPI.GetCommittee()
if err != nil {
    // return error
}

// list committees
for _, m := range committees.Committees {
    fmt.Println("committee ID: ", m.InstanceID, "distributed PK:", m.DistributedPK)
}
```

### Response example

```json
{
    "committees": [
        {
            "instanceID": 1,
            "threshold": 3,
            "identities": [
                "AheLpbhRs1XZsRF8t8VBwuyQh9mqPHXQvthV5rsHytDG",
                "FZ28bSTidszUBn8TTCAT9X1nVMwFNnoYBmZ1xfafez2z",
                "GT3UxryW4rA9RN9ojnMGmZgE2wP7psagQxgVdA4B9L1P",
                "4pB5boPvvk2o5MbMySDhqsmC2CtUdXyotPPEpb7YQPD7",
                "64wCsTZpmKjRVHtBKXiFojw7uw3GszumfvC4kHdWsHga"
            ],
            "distributedPK": "884bc65f1d023d84e2bd2e794320dc29600290ca7c83fefb2455dae2a07f2ae4f969f39de6b67b8005e3a328bb0196de"
        }
    ]
}
```

### Results

|Return field | Type | Description|
|:-----|:------|:------|
| `committees`  | `[]Committee` | A list of DRNG committees.   |
| `error` | `string` | Error message. Omitted if success.     |

* Type `Committee`

|field | Type | Description|
|:-----|:------|:------|
| `instanceID`  | `string` | The identifier of the dRAND instance.  |
| `threshold`   | `string` | The threshold of the secret sharing protocol.    |
| `identities`   | `float64` | The nodes' identities of the committee members.     |
| `distributedPK`   | `string` | Distributed Public Key of the committee     |


## `/drng/info/randomness`

Returns the current DRNG randomness used.

### Parameters

None.

### Examples

#### cURL

```shell
curl http://localhost:8080/drng/info/randomness
```

#### client lib - `GetRandomness`

Current randomness from known committees can be retrieved using `GetRandomness() (*jsonmodels.RandomnessResponse, 
error)`.

```go
randomness, err := goshimAPI.GetRandomness()
if err != nil {
    // return error
}

// list randomness
for _, m := range randomness.Randomness {
    fmt.Println("committee ID: ", m.InstanceID, "Randomness:", m.Randomness)
}
```

### Response example

```json
{
    "randomness": [
        {
            "instanceID": 1,
            "round": 2461530,
            "timestamp": "2021-05-24T18:06:20.394849622+02:00",
            "randomness": "Kr5buSEtgLuPxZrax0HfoiougcOXS/75JOBu2Ld6peO77qdKiNyjDueXQZlPE0UCTKkVhehEvfIXhESK9DF3aQ=="
        }
    ]
}
```

### Results

|Return field | Type | Description|
|:-----|:------|:------|
| `randomness`  | `[]Randomness` | List of randomness  |
| `error`   | `string` | Error message. Omitted if success.     |

* Type `Randomness`

|field | Type | Description|
|:-----|:------|:------|
| `instanceID`  | `uint32` | The identifier of the dRAND instance.  |
| `round`   | `uint64` | The current DRNG round.    |
| `timestamp`   | `time.Time` | The timestamp of the current randomness message     |
| `randomness`   | `[]byte` | The current randomness as a slice of bytes    |
