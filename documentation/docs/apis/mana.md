---
description: The mana APIs provide methods for people to retrieve the amount of access/consensus mana of nodes and outputs, as well as the event logs.
image: /img/logo/goshimmer_light.png
keywords:
- client library
- HTTP API
- mana
- percentile
- online
- consensus
- pledge
---

# Mana API Methods

The mana APIs provide methods for people to retrieve the amount of access/consensus mana of nodes and outputs, as well as the event logs.   

HTTP APIs:
* [/mana](#mana)
* [/mana/all](#manaall)
* [/mana/percentile](#manapercentile)
* [/mana/access/online](#manaaccessonline)
* [/mana/consensus/online](#manaconsensusonline)
* [/mana/access/nhighest](#manaaccessnhighest)
* [/mana/consensus/nhighest](#manaconsensusnhighest)
* [/mana/pending](#manapending)
* [/mana/consensus/past](#manaconsensuspast)
* [/mana/consensus/logs](#manaconsensuslogs)
* [/mana/allowedManaPledge](#manaallowedmanapledge)

Client lib APIs:
* [GetOwnMana()](#getownmana)
* [GetManaFullNodeID()](#getmanafullnodeid)
* [GetMana with short node ID()](#getmana-with-short-node-id)
* [GetAllMana()](#client-lib---getallmana)
* [GetManaPercentile()](#client-lib---getmanapercentile)
* [GetOnlineAccessMana()](#client-lib---getonlineaccessmana)
* [GetOnlineConsensusMana()](#client-lib---getonlineconsensusmana)
* [GetNHighestAccessMana()](#client-lib---getnhighestaccessmana)
* [GetNHighestConsensusMana()](#client-lib---getnhighestconsensusmana)
* [GetPending()](#client-lib---getpending)
* [GetPastConsensusManaVector()](#client-lib---getpastconsensusmanavector)
* [GetConsensusEventLogs()](#client-lib---getconsensuseventlogs)
* [GetAllowedManaPledgeNodeIDs()](#client-lib---getallowedmanapledgenodeids)



## `/mana`

Get the access and consensus mana of the node.

### Parameters

| **Parameter**            | `node ID`      |
|--------------------------|----------------|
| **Required or Optional** | optional       |
| **Description**          | full node ID   |
| **Type**                 | string         |


#### **Note**
If no node ID is given, it returns the access and consensus mana of the node you're communicating with.

### Examples

#### cURL

```shell
curl http://localhost:8080/mana?2GtxMQD94KvDH1SJPJV7icxofkyV1njuUZKtsqKmtux5 \
-X GET \
-H 'Content-Type: application/json'
```

#### client lib
There are 3 APIs to get mana of a node, which is based on the same HTTP API `/mana`.

##### `GetOwnMana`
Get the access and consensus mana of the node this API client is communicating with.

```go
manas, err := goshimAPI.GetOwnMana()
if err != nil {
    // return error
}

// print the node ID
fmt.Println("full ID: ", manas.NodeID, "short ID: ", manas.ShortNodeID)

// get access mana of the node
fmt.Println("access mana: ", manas.Access, "access mana updated time: ", manas.AccessTimestamp)

// get consensus mana of the node
fmt.Println("consensus mana: ", manas.Consensus, "consensus mana updated time: ", manas.ConsensusTimestamp)
```

##### `GetManaFullNodeID`
Get Mana of a node with its full node ID.

```go
manas, err := goshimAPI.GetManaFullNodeID("2GtxMQD94KvDH1SJPJV7icxofkyV1njuUZKtsqKmtux5")
if err != nil {
    // return error
}
```

##### `GetMana` with short node ID
```go
manas, err := goshimAPI.GetMana("4AeXyZ26e4G")
if err != nil {
    // return error
}
```

### Response examples
```json
{
  "shortNodeID": "4AeXyZ26e4G",
  "nodeID": "2GtxMQD94KvDH1SJPJV7icxofkyV1njuUZKtsqKmtux5",
  "access": 26.5,
  "accessTimestamp": 1614924295,
  "consensus": 26.5,
  "consensusTimestamp": 1614924295
}
```

### Results
|Return field | Type | Description|
|:-----|:------|:------|
| `shortNodeID`  | string | The short ID of a node.   |
| `nodeID`   | string | The full ID of a node.     |
| `access`   | float64 | The amount of access mana.     |
| `accessTimestamp` | int64 | The timestamp of access mana updates.     |
| `consensus`   | float64 | The amount of consensus mana.     |
| `consensusTimestamp` | int64 | The timestamp of consensus mana updates.  |



## `/mana/all`

Get the mana perception of the node in the network. You can retrieve the full/short node ID, consensus mana, access mana of each node, and the mana updated time.

### Parameters
None.

### Examples

#### cURL

```shell
curl http://localhost:8080/mana/all \
-X GET \
-H 'Content-Type: application/json'
```

#### Client lib - `GetAllMana()` 

```go
manas, err := goshimAPI.GetAllMana()
if err != nil {
    // return error
}

// mana updated time
fmt.Println("access mana updated time: ", manas.AccessTimestamp)
fmt.Println("consensus mana updated time: ", manas.ConsensusTimestamp)

// get access mana of each node
for _, m := range manas.Access {
    fmt.Println("full node ID: ", m.NodeID, "short node ID:", m.ShortNodeID, "access mana: ", m.Mana)
}

// get consensus mana of each node
for _, m := range manas.Consensus {
    fmt.Println("full node ID: ", m.NodeID, "short node ID:", m.ShortNodeID, "consensus mana: ", m.Mana)
}
```

### Response examples
```json
{
  "access": [
      {
          "shortNodeID": "4AeXyZ26e4G",
          "nodeID": "2GtxMQD94KvDH1SJPJV7icxofkyV1njuUZKtsqKmtux5",
          "mana": 26.5
      }
  ],
  "accessTimestamp": 1614924295,
  "consensus": [
      {
          "shortNodeID": "4AeXyZ26e4G",
          "nodeID": "2GtxMQD94KvDH1SJPJV7icxofkyV1njuUZKtsqKmtux5",
          "mana": 26.5
      }
  ],
  "consensusTimestamp": 1614924295
}
```

### Results
|Return field | Type | Description|
|:-----|:------|:------|
| `access`  | mana.NodeStr | A list of node that has access mana.   |
| `accessTimestamp` | int64 | The timestamp of access mana updates.     |
| `consensus`   | mana.NodeStr | A list of node that has access mana.     |
| `consensusTimestamp` | int64 | The timestamp of consensus mana updates.  |

#### Type `mana.NodeStr`
|field | Type | Description|
|:-----|:------|:------|
| `shortNodeID`  | string | The short ID of a node.   |
| `nodeID`   | string | The full ID of a node.     |
| `mana`   | float64 | The amount of mana.     |



## `/mana/percentile`

To learn the top percentile the node belongs to relative to the network in terms of mana. The input should be a full node ID.

### Parameters
| | |
|-|-|
| **Parameter**  | `node ID`          |
| **Required or Optional**   | Required     |
| **Description**   | full node ID      |
| **Type**      | string      |

### Examples

#### cURL

```shell
curl http://localhost:8080/mana/percentile?2GtxMQD94KvDH1SJPJV7icxofkyV1njuUZKtsqKmtux5 \
-X GET \
-H 'Content-Type: application/json'
```

#### Client lib - `GetManaPercentile()`

```go
mana, err := goshimAPI.GetManaPercentile("2GtxMQD94KvDH1SJPJV7icxofkyV1njuUZKtsqKmtux5")
if err != nil {
    // return error
}

// mana updated time
fmt.Println("access mana percentile: ", mana.Access, "access mana updated time: ", manas.AccessTimestamp)
fmt.Println("consensus mana percentile: ", mana.Consensus, "consensus mana updated time: ", manas.ConsensusTimestamp)
```

### Response examples
```json
{
  "shortNodeID": "4AeXyZ26e4G",
  "nodeID": "2GtxMQD94KvDH1SJPJV7icxofkyV1njuUZKtsqKmtux5",
  "access": 75,
  "accessTimestamp": 1614924295,
  "consensus": 75,
  "consensusTimestamp": 1614924295
}
```

### Results
|Return field | Type | Description|
|:-----|:------|:------|
| `shortNodeID`  | string | The short ID of a node.   |
| `nodeID`   | string | The full ID of a node.     |
| `access`  | float64 | Access mana percentile of a node.    |
| `accessTimestamp` | int64 | The timestamp of access mana updates.     |
| `consensus`   | float64 | Access mana percentile of a node.     |
| `consensusTimestamp` | int64 | The timestamp of consensus mana updates.  |



## `/mana/access/online`

You can get a sorted list of online access mana of nodes, sorted from the highest access mana to the lowest. The highest access mana node has OnlineRank 1, and increases 1 by 1 for the following nodes.

### Parameters
None.

### Examples

#### cURL

```shell
curl http://localhost:8080/mana/access/online \
-X GET \
-H 'Content-Type: application/json'
```

#### Client lib - `GetOnlineAccessMana()`

```go
// online access mana
accessMana, err := goshimAPI.GetOnlineAccessMana()
if err != nil {
    // return error
}

for _, m := accessMana.Online {
    fmt.Println("full node ID: ", m.ID, "mana rank: ", m.OnlineRank, "access mana: ", m.Mana)
}
```

### Response examples
```json
{
  "online": [
    {
      "rank": 1,
      "shortNodeID": "4AeXyZ26e4G",
      "nodeID": "2GtxMQD94KvDH1SJPJV7icxofkyV1njuUZKtsqKmtux5",
      "mana": 75
    }
  ],
  "timestamp": 1614924295
}
```

### Results
|Return field | Type | Description|
|:-----|:------|:------|
| `online`  | OnlineNodeStr | The access mana information of online nodes.   |
| `timestamp` | int64 | The timestamp of mana updates.  |

#### Type `OnlineNodeStr`
|Field | Type | Description|
|:-----|:------|:------|
| `rank`  | int | The rank of a node. |
| `shortNodeID`  | string | The short ID of a node.   |
| `nodeID`   | string | The full ID of a node.     |
| `mana`   | float64 | The amount of access mana.     |



## `/mana/consensus/online`

You can get a sorted list of online consensus mana of nodes, sorted from the highest consensus mana to the lowest. The highest consensus mana node has OnlineRank 1, and increases 1 by 1 for the following nodes.

### Parameters
None.

### Examples

#### cURL

```shell
curl http://localhost:8080/mana/consensus/online \
-X GET \
-H 'Content-Type: application/json'
```

#### Client lib - `GetOnlineConsensusMana()`

```go
// online access mana
accessMana, err := goshimAPI.GetOnlineConsensusMana()
if err != nil {
    // return error
}

for _, m := accessMana.Online {
    fmt.Println("full node ID: ", m.ID, "mana rank: ", m.OnlineRank, "consensus mana: ", m.Mana)
}
```

### Response examples
```json
{
  "online": [
      {
        "rank": 1,
        "shortNodeID": "4AeXyZ26e4G",
        "nodeID": "2GtxMQD94KvDH1SJPJV7icxofkyV1njuUZKtsqKmtux5",
        "mana": 75
      }
  ],  
  "timestamp": 1614924295
}
```

### Results
|Return field | Type | Description|
|:-----|:------|:------|
| `online`  | OnlineNodeStr | The consensus mana information of online nodes.   |
| `timestamp` | int64 | The timestamp of mana updates.  |

#### Type `OnlineNodeStr`
|Field | Type | Description|
|:-----|:------|:------|
| `rank`  | int | The rank of a node. |
| `shortNodeID`  | string | The short ID of a node.   |
| `nodeID`   | string | The full ID of a node.     |
| `mana`   | float64 | The amount of consensus mana.     |



## `/mana/access/nhighest`

You can get the N highest access mana holders in the network, sorted in descending order.
If N=0, all nodes that have access mana are returned sorted.

### Parameters
| | |
|-|-|
| **Parameter**  | `N`          |
| **Required or Optional**   | Required     |
| **Description**   | The number of highest mana nodes.      |
| **Type**      | int      |

### Examples

#### cURL

```shell
curl http://localhost:8080/mana/access/nhighest?number=5 \
-X GET \
-H 'Content-Type: application/json'
```

#### Client lib - `GetNHighestAccessMana()`

```go
// get the top 5 highest access mana nodes
accessMana, err := goshimAPI.GetNHighestAccessMana(5)
if err != nil {
    // return error
}

for _, m := accessMana.Nodes {
    fmt.Println("full node ID: ", m.NodeID, "access mana: ", m.Mana)
}v
```

### Response examples
```json
{
  "nodes": [
      {
        "shortNodeID": "4AeXyZ26e4G",
        "nodeID": "2GtxMQD94KvDH1SJPJV7icxofkyV1njuUZKtsqKmtux5",
        "mana": 26.5
      }
  ],  
  "timestamp": 1614924295
}
```

### Results
|Return field | Type | Description|
|:-----|:------|:------|
| `nodes`  | mana.NodeStr | The N highest access mana nodes.   |
| `timestamp` | int64 | The timestamp of mana updates.  |

#### Type `mana.NodeStr`
|field | Type | Description|
|:-----|:------|:------|
| `shortNodeID`  | string | The short ID of a node.   |
| `nodeID`   | string | The full ID of a node.     |
| `mana`   | float64 | The amount of mana.     |



## `/mana/consensus/nhighest`

You can get the N highest consensus mana holders in the network, sorted in descending order.

### Parameters
| | |
|-|-|
| **Parameter**  | `N`          |
| **Required or Optional**   | Required     |
| **Description**   | The number of highest consensus mana nodes.      |
| **Type**      | int      |

### Examples

#### cURL

```shell
curl http://localhost:8080/mana/consensus/nhighest?number=5 \
-X GET \
-H 'Content-Type: application/json'
```

#### Client lib - `GetNHighestConsensusMana()`

```go
// get the top 5 highest consensus mana nodes
consensusMana, err := goshimAPI.GetNHighestConsensusMana(5)
if err != nil {
    // return error
}

for _, m := consensusMana.Nodes {
    fmt.Println("full node ID: ", m.NodeID, "consensus mana: ", m.Mana)
}v
```

### Response examples
```json
{
  "nodes": [
      {
        "shortNodeID": "4AeXyZ26e4G",
        "nodeID": "2GtxMQD94KvDH1SJPJV7icxofkyV1njuUZKtsqKmtux5",
        "mana": 26.5
      }
  ],  
  "timestamp": 1614924295
}
```

### Results
|Return field | Type | Description|
|:-----|:------|:------|
| `nodes`  | mana.NodeStr | The N highest consensus mana nodes.   |
| `timestamp` | int64 | The timestamp of mana updates.  |

#### Type `mana.NodeStr`
|field | Type | Description|
|:-----|:------|:------|
| `shortNodeID`  | string | The short ID of a node.   |
| `nodeID`   | string | The full ID of a node.     |
| `mana`   | float64 | The amount of mana.     |



## `/mana/pending`

Get the amount of base access mana that would be pledged if the given output was spent.

### Parameters
| | |
|-|-|
| **Parameter**  | `outputID`          |
| **Required or Optional**   | Required     |
| **Description**   | The requesting output ID.      |
| **Type**      | string      |

### Examples

#### cURL

```shell
curl http://localhost:8080/mana/pending?outputid="4a5KkxVfsdFVbf1NBGeGTCjP8Ppsje4YFQg9bu5YGNMSJK1" \
-X GET \
-H 'Content-Type: application/json'
```

#### Client lib - `GetPending()`

```go
res, err := goshimAPI.GetPending("4a5KkxVfsdFVbf1NBGeGTCjP8Ppsje4YFQg9bu5YGNMSJK1")
if err != nil {
    // return error
}

// get the amount of mana
fmt.Println("mana be pledged: ", res.Mana)
fmt.Println("the timestamp of the output (decay duration)", res.Timestamp)
```

### Response examples
```json
{
  "mana": 26.5,
  "outputID": "4a5KkxVfsdFVbf1NBGeGTCjP8Ppsje4YFQg9bu5YGNMSJK1",
  "timestamp": 1614924295
}
```

### Results
|Return field | Type | Description|
|:-----|:------|:------|
| `mana`   | float64 | The amount of access base mana to be pledged.     |
| `outputID`  | string | The output ID of the request.     |
| `timestamp` | int64 | The timestamp of mana updates.  |



## `/mana/consensus/past`

Get the consensus base mana vector of a time (int64) in the past.

### Parameters
| | |
|-|-|
| **Parameter**  | `timestamp`          |
| **Required or Optional**   | Required     |
| **Description**   | The timestamp of the request.      |
| **Type**      | int64      |

### Examples

#### cURL

```shell
curl http://localhost:8080/mana/consensus/past?timestamp=1614924295 \
-X GET \
-H 'Content-Type: application/json'
```

#### Client lib - `GetPastConsensusManaVector()`

```go
res, err := goshimAPI.GetPastConsensusManaVector(1614924295)
if err != nil {
    // return error
}

// the mana vector of each node
for _, m := range res.Consensus {
    fmt.Println("node ID:", m.NodeID, "consensus mana: ", m.Mana)
}
```

### Response examples
```json
{
  "consensus": [
      {
        "shortNodeID": "4AeXyZ26e4G",
        "nodeID": "2GtxMQD94KvDH1SJPJV7icxofkyV1njuUZKtsqKmtux5",
        "mana": 26.5 
      }
  ],  
  "timestamp": 1614924295
}
```

### Results
|Return field | Type | Description|
|:-----|:------|:------|
| `consensus`   | mana.NodeStr | The consensus mana of nodes.     |
| `timestamp` | int64 | The timestamp of mana updates.  |

#### Type `mana.NodeStr`
|field | Type | Description|
|:-----|:------|:------|
| `shortNodeID`  | string | The short ID of a node.   |
| `nodeID`   | string | The full ID of a node.     |
| `mana`   | float64 | The amount of mana.     |



## `/mana/consensus/logs`

Get the consensus event logs of the given node IDs.

### Parameters
| | |
|-|-|
| **Parameter**  | `nodeIDs`          |
| **Required or Optional**   | Required     |
| **Description**   | A list of node ID of the request.      |
| **Type**      | string array      |

### Examples

#### cURL

```shell
curl http://localhost:8080/mana/consensus/logs \
-X GET \
-H 'Content-Type: application/json'
-d '{
  "nodeIDs": [
    "2GtxMQD94KvDH1SJPJV7icxofkyV1njuUZKtsqKmtux5",
    "2GtxMQD94KvDH1SJPJV7icxofkyV1njuUZKtsqKmtux6"
  ]
}'
```

#### Client lib - `GetConsensusEventLogs()`

```go
res, err := goshimAPI.GetConsensusEventLogs([]string{"2GtxMQD94KvDH1SJPJV7icxofkyV1njuUZKtsqKmtux5"})
if err != nil {
    // return error
}

for nodeID, e := range res.Logs {
    fmt.Println("node ID:", nodeID)
    
    // pledge logs
    for _, p := e.Pledge {
        fmt.Println("mana type: ", p.ManaType)
        fmt.Println("node ID: ", p.NodeID)
        fmt.Println("time: ", p.Time)
        fmt.Println("transaction ID: ", p.TxID)
        fmt.Println("mana amount: ", p.Amount)
    }

    // revoke logs
    for _, r := e.Revoke {
        fmt.Println("mana type: ", r.ManaType)
        fmt.Println("node ID: ", r.NodeID)
        fmt.Println("time: ", r.Time)
        fmt.Println("transaction ID: ", r.TxID)
        fmt.Println("mana amount: ", r.Amount)
        fmt.Println("input ID: ", r.InputID)
    }
}
```

### Response examples
```json
{
  "logs": [
      "2GtxMQD94KvDH1SJPJV7icxofkyV1njuUZKtsqKmtux5": {
          "pledge": [
              {
                  "manaType": "Consensus",
                  "nodeID": "2GtxMQD94KvDH1SJPJV7icxofkyV1njuUZKtsqKmtux5",
                  "time": 1614924295,
                  "txID": "7oAfcEhodkfVyGyGrobBpRrjjdsftQknpj5KVBQjyrda",
                  "amount": 28
               }
          ],
          "revoke": [
              {
                  "manaType": "Consensus",
                  "nodeID": "2GtxMQD94KvDH1SJPJV7icxofkyV1njuUZKtsqKmtux5",
                  "time": 1614924295,
                  "txID": "7oAfcEhodkfVyGyGrobBpRrjjdsftQknpj5KVBQjyrda",
                  "amount": 28,
                  "inputID": "35P4cW9QfzHNjXJwZMDMCUxAR7F9mfm6FvPbdpJWudK2nBZ"
              }
          ]
      }
  ],  
  "startTime": 1614924295,
  "endTime": 1614924300
}
```

### Results
|Return field | Type | Description|
|:-----|:------|:------|
| `logs`   | map[string]*EventLogsJSON | The consensus mana of nodes. The key of map is node ID.     |
| `startTime` | int64 | The starting time of collecting logs.  |
| `endTime` | int64 | The ending time of collecting logs.  |

#### Type `EventLogsJSON`
|field | Type | Description|
|:-----|:------|:------|
| `pledge`  | PledgedEventJSON | Pledged event logs.   |
| `revoke`   | RevokedEventJSON | Revoked event logs.     |

#### Type `PledgedEventJSON`
|field | Type | Description|
|:-----|:------|:------|
| `manaType`  | string | Type of mana.   |
| `nodeID`   | string | The full ID of a node.     |
| `time` | int64 | The time of transaction.  |
| `txID`   | string | The transaction ID of pledged mana.     |
| `amount`   | float64 | The amount of pledged mana.    |

#### Type `RevokedEventJSON`
|field | Type | Description|
|:-----|:------|:------|
| `manaType`  | string | Type of mana.   |
| `nodeID`   | string | The full ID of a node.     |
| `time` | int64 | The time of transaction.  |
| `txID`   | string | The transaction ID of revoked mana.     |
| `amount`   | float64 | The amount of revoked mana.    |
| `inputID`   | string | The input ID of revoked mana.     |



## `/mana/allowedManaPledge`

This returns the list of allowed mana pledge node IDs.

### Parameters
None.

### Examples

#### cURL

```shell
curl http://localhost:8080/mana/allowedManaPledge \
-X GET \
-H 'Content-Type: application/json'
```

#### Client lib - `GetAllowedManaPledgeNodeIDs()`

```go
res, err := goshimAPI.GetAllowedManaPledgeNodeIDs()
if err != nil {
    // return error
}

// print the list of nodes that access mana is allowed to be pledged to
for _, id := range res.Access.Allowed {
    fmt.Println("node ID:", id)
}

// print the list of nodes that consensus mana is allowed to be pledged to
for _, id := range res.Consensus.Allowed {
    fmt.Println("node ID:", id)
}
```

### Response examples
```json
{
  "accessMana": {
      "isFilterEnabled": false,
      "allowed": [
          "2GtxMQD94KvDH1SJPJV7icxofkyV1njuUZKtsqKmtux5"
      ] 
  }
  "consensusMana": {
      "isFilterEnabled": false,
      "allowed": [
          "2GtxMQD94KvDH1SJPJV7icxofkyV1njuUZKtsqKmtux5"
      ] 
  }
}
```

### Results
|Return field | Type | Description|
|:-----|:------|:------|
| `accessMana`   | AllowedPledge | A list of nodes that allow to pledge access mana.     |
| `consensusMana` | AllowedPledge | A list of nodes that allow to pledge consensus mana.  |

#### Type `AllowedPledge`
|field | Type | Description|
|:-----|:------|:------|
| `isFilterEnabled`  | bool | A flag shows that if mana pledge filter is enabled.   |
| `allowed`   | []string | A list of node ID that allow to be pledged mana. This list has effect only if `isFilterEnabled` is `true`|



