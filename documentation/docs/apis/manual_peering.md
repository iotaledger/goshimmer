---
description: The manual peering APIs allows you to add, get and remove the list of known peers of the node.
image: /img/logo/goshimmer_light.png
keywords:
- client library
- HTTP API
- known peer
- peer
- public key
- gossip port
---
# Manual Peering API methods

The manual peering APIs allow managing the list of known peers of the node.

HTTP APIs:

* POST [/manualpeering/peers](#post-manualpeeringpeers)
* GET [/manualpeering/peers](#get-manualpeeringpeers)
* DELETE [/manualpeering/peers](#delete-manualpeeringpeers)

Client lib APIs:

* [AddManualPeers()](#addmanualpeers)
* [GetManualPeers()](#getmanualpeers)
* [RemoveManualPeers()](#removemanualpeers)



## POST `/manualpeering/peers`

Add peers to the list of known peers of the node.

### Request Body

```json
[
  {
    "publicKey": "CHfU1NUf6ZvUKDQHTG2df53GR7CvuMFtyt7YymJ6DwS3",
    "address": "127.0.0.1:14666"
  }
]
```

#### Description

|Field | Description|
|:-----|:------|
| `publicKey` | Public key of the peer. |
| `address`   | IP address of the peer's node and its gossip port. |

### Response

HTTP status code: 204 No Content

### Examples

#### cURL

```shell
curl --location --request POST 'http://localhost:8080/manualpeering/peers' \
--header 'Content-Type: application/json' \
--data-raw '[
    {
        "publicKey": "CHfU1NUf6ZvUKDQHTG2df53GR7CvuMFtyt7YymJ6DwS3",
        "address": "172.19.0.3:14666"
    }
]'
```

### Client library

#### `AddManualPeers`

```go
import "github.com/iotaledger/goshimmer/packages/manualpeering"

peersToAdd := []*manualpeering.KnownPeerToAdd{{PublicKey: publicKey, Address: address}}
err := goshimAPI.AddManualPeers(peersToAdd)
if err != nil {
// return error
}
```



## GET `/manualpeering/peers`

Get the list of all known peers of the node.

### Request Body

```json
{
  "onlyConnected": true
}
```

#### Description

|Field | Description|
|:-----|:------|
| `onlyConnected` | Optional, if set to true only peers with established connection will be returned. |

### Response

HTTP status code: 200 OK

```json
[
  {
    "publicKey": "CHfU1NUf6ZvUKDQHTG2df53GR7CvuMFtyt7YymJ6DwS3",
    "address": "127.0.0.1:14666",
    "connectionDirection": "inbound",
    "connectionStatus": "connected"
  }
]
```

#### Description

|Field | Description|
|:-----|:------|
| `publicKey` | The public key of the peer node. |
| `address` | IP address of the peer's node and its gossip port. |
| `connectionDirection` | Enum, possible values: "inbound", "outbound". Inbound means that the local node accepts the connection. On the other side, the other peer node dials, and it will have "outbound" connectionDirection.  |
| `connectionStatus` | Enum, possible values: "disconnected", "connected". Whether the actual TCP connection has been established between peers. |

### Examples

#### cURL

```shell
curl --location --request GET 'http://localhost:8080/manualpeering/peers' \
--header 'Content-Type: application/json' \
--data-raw '{
    "onlyConnected": true
}'
```

### Client library

#### `GetManualPeers`

```go
import "github.com/iotaledger/goshimmer/packages/manualpeering"

peers, err := goshimAPI.GetManualPeers(manualpeering.WithOnlyConnectedPeers())
if err != nil {
// return error
}
fmt.Println(peers)
```



## DELETE `/manualpeering/peers`

Remove peers from the list of known peers of the node.

### Request Body

```json
[
  {
    "publicKey": "CHfU1NUf6ZvUKDQHTG2df53GR7CvuMFtyt7YymJ6DwS3"
  }
]
```

#### Description

|Field | Description|
|:-----|:------|
| `publicKey` | Public key of the peer to remove from the list. |

### Response

HTTP status code: 204 No Content

### Examples

#### cURL

```shell
curl --location --request DELETE 'http://localhost:8080/manualpeering/peers' \
--header 'Content-Type: application/json' \
--data-raw '[
    {
        "publicKey": "8qN1yD95fhbfDZtKX49RYFEXqej5fvsXJ2NPmF1LCqbd"
    }
]'
```

### Client library

#### `RemoveManualPeers`

```go
import "github.com/iotaledger/hive.go/crypto/ed25519"
import "github.com/iotaledger/goshimmer/packages/manualpeering"

publicKeysToRemove := []ed25519.PublicKey{publicKey1, publicKey2}
err := goshimAPI.RemoveManualPeers(publicKeysToRemove)
if err != nil {
// return error
}
```