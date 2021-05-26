# Manual Peering API Methods

Peering API allows retrieving basic information about autopeering.

The API provides the following functions and endpoints:

* [/manualpeering/peers (POST)](#autopeeringneighbors---add-peers)
* [/manualpeering/peers (POST)](#autopeeringneighbors---remove-peers)
* [/manualpeering/peers (POST)](#autopeeringneighbors---list-peers)

##  `/manualpeering/peers` - Add Peers

Connects to the peers passed in the POST body.


### Parameters

POST body in JSON format with information about peers in the following format.

#### Body

```json
[
  {
    "publicKey": "EYsaGXnUVA9aTYL9FwYEvoQ8d1HCJveQVL7vogu6pqCP",
    "ip": "172.19.0.3",
    "services": {
      "peering":{
        "network":"TCP",
        "port":14626
      },
      "gossip": {
        "network": "TCP",
        "port": 14666
      }
    }
  }
]
```


### Examples

#### cURL

```shell
curl --location --request POST 'http://localhost:8080//manualpeering/peers' \
--header 'Content-Type: application/json' \
--data-raw '[
  {
    "publicKey": "EYsaGXnUVA9aTYL9FwYEvoQ8d1HCJveQVL7vogu6pqCP",
    "ip": "172.19.0.3",
    "services": {
      "peering":{
        "network":"TCP",
        "port":14626
      },
      "gossip": {
        "network": "TCP",
        "port": 14666
      }
    }
  }
]'
```

#### Client lib - `AddManualPeers()`

Peers can be removed via `AddManualPeers(peers []*peer.Peer)`
```
err := goshimAPI.AddManualPeers(false)
if err != nil {
    // return error
}
```

#### Results

Empty response with HTTP 200 success code if peers were added correctly.

##  `/manualpeering/peers` - Remove Peers

Method: `DELETE`

Removes peers passed in the DELETE body.

### Parameters

DELETE body in JSON format with information about peers in the following format.

#### Body

```json
[
  {
    "publicKey": "8qN1yD95fhbfDZtKX49RYFEXqej5fvsXJ2NPmF1LCqbd"
  }
]
```

### Examples

#### cURL

```shell
curl --location --request DELETE 'http://localhost:8080//manualpeering/peers' \
--header 'Content-Type: application/json' \
--data-raw '[
  {
    "publicKey": "8qN1yD95fhbfDZtKX49RYFEXqej5fvsXJ2NPmF1LCqbd"
  }
]'
```

#### Client lib - `RemoveManualPeers()`

Peers can be removed via `RemoveManualPeers(keys []ed25519.PublicKey)`
```
err := goshimAPI.RemoveManualPeers(false)
if err != nil {
    // return error
}
```

#### Results

Empty response with HTTP 200 success code if peers were added correctly.

##  `/manualpeering/peers` - Remove Peers

Method: `DELETE`

Removes peers passed in the DELETE body.

### Parameters

DELETE body in JSON format with information about peers in the following format.

#### Body

```json
[
  {
    "publicKey": "8qN1yD95fhbfDZtKX49RYFEXqej5fvsXJ2NPmF1LCqbd"
  }
]
```

### Examples

#### cURL

```shell
curl --location --request DELETE 'http://localhost:8080//manualpeering/peers' \
--header 'Content-Type: application/json' \
--data-raw '[
  {
    "publicKey": "8qN1yD95fhbfDZtKX49RYFEXqej5fvsXJ2NPmF1LCqbd"
  }
]'
```

#### Client lib - `GetManualKnownPeers()`

Manually added peers can be retrieved via `GetManualKnownPeers(opts ...manualpeering.GetKnownPeersOption)`
```
neighbors, err := goshimAPI.GetManualKnownPeers(false)
if err != nil {
    // return error
}

// will print the response
fmt.Println(string(neighbors))
```

#### Results

Empty response with HTTP 200 success code if peers were added correctly.


##  `/manualpeering/peers`

Method: `GET`

Returns the peers of the node selected using manual peering.


### Parameters

| **Parameter**            | `onlyConnected`      |
|--------------------------|----------------|
| **Required or Optional** | optional       |
| **Description**          | Return only connected peers. Otherwise, return all peers added manually. |
| **Type**                 | bool         |


### Examples

#### cURL

```shell
curl --location 'http://localhost:8080/manualpeering/peers?onlyConnected=true'
```

#### Client lib - `GetManualKnownPeers()`

Manually added peers can be retrieved via `GetManualKnownPeers(opts ...manualpeering.GetKnownPeersOption)`
```
neighbors, err := goshimAPI.GetManualKnownPeers(false)
if err != nil {
    // return error
}

// will print the response
fmt.Println(string(neighbors))
```

#### Response examples
```json
{
  "chosen": [
    {
      "id": "PtBSYhniWR2",
      "publicKey": "BogpestCotcmbB2EYKSsyVMywFYvUt1MwGh6nUot8g5X",
      "services": [
        {
          "id": "peering",
          "address": "178.254.42.235:14626"
        },
        {
          "id": "gossip",
          "address": "178.254.42.235:14666"
        },
        {
          "id": "FPC",
          "address": "178.254.42.235:10895"
        }
      ]
    }
  ],
  "accepted": [
    {
      "id": "CRPFWYijV1T",
      "publicKey": "GUdTwLDb6t6vZ7X5XzEnjFNDEVPteU7tVQ9nzKLfPjdo",
      "services": [
        {
          "id": "peering",
          "address": "35.214.101.88:14626"
        },
        {
          "id": "gossip",
          "address": "35.214.101.88:14666"
        },
        {
          "id": "FPC",
          "address": "35.214.101.88:10895"
        }
      ]
    }
  ]
}
```

#### Results

* Returned type

|Return field | Type | Description|
|:-----|:------|:------|
| `known`  | `[]Neighbor` | List of known peers. Only returned when parameter is set. |
| `chosen`  | `[]Neighbor` | List of chosen peers. |
| `accepted`  | `[]Neighbor` | List of accepted peers. |
| `error` | `string` | Error message. Omitted if success.     |

* Type `Neighbor`

|field | Type | Description|
|:-----|:------|:------|
| `id`  | `string` | Comparable node identifier.  |
| `publicKey`   | `string` | Public key used to verify signatures.   |
| `services`   | `[]PeerService` | List of exposed services.     |

* Type `PeerService`

|field | Type | Description|
|:-----|:------|:------|
| `id`  | `string` | Type of service.  |
| `address`   | `string` |  Network address of the service.   |
