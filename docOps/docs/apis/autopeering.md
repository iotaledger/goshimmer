# Peering API Methods

The peering API allows you to retrieve basic information about autopeering.

The API provides the following functions and endpoints:

:::info
The HTTP may contain endpoints with no equivalent method in the Client Lib API.  
:::

## /autopeering/neighbors

Returns the chosen and accepted neighbors of the node.

### Parameters

| **Parameter**            | `known`      |
|--------------------------|----------------|
| **Required or Optional** | optional       |
| **Description**          | Return all known peers, set to `1` (default: `0`)   |
| **Type**                 | int         |

### Client lib - GetAutopeeringNeighbors

Messages can be retrieved via `GetAutopeeringNeighbors(knownPeers bool) (*jsonmodels.GetNeighborsResponse, error)`

```go
neighbors, err := goshimAPI.GetAutopeeringNeighbors(false)
if err != nil {
    // return error
}

// will print the response
fmt.Println(string(neighbors))
```

### Examples

#### cURL

```shell
curl --location 'http://localhost:8080/autopeering/neighbors?known=1'
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

##### Returned type

|Return field | Type | Description|
|:-----|:------|:------|
| `known`  | [][Neighbor](#type-neighbor) | List of known peers. Only returned when parameter is set. |
| `chosen`  |[][Neighbor](#type-neighbor) | List of chosen peers. |
| `accepted`  | [][Neighbor](#type-neighbor) | List of accepted peers. |
| `error` | `string` | Error message. Omitted if success.     |

######  Type Neighbor

|field | Type | Description|
|:-----|:------|:------|
| `id`  | `string` | Comparable node identifier.  |
| `publicKey`   | `string` | Public key used to verify signatures.   |
| `services`   | [][PeerService](#type-peerservice) | List of exposed services.     |

###### Type PeerService

|field | Type | Description|
|:-----|:------|:------|
| `id`  | `string` | Type of service.  |
| `address`   | `string` |  Network address of the service.   |
