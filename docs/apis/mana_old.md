# Mana APIs (deprecated)

#### Get Own Mana
You can get the access and consensus mana of the node this API client is communicating with.

```
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

#### Get Mana of a node with its full nodeID
You can get the access and consensus mana of a node with its full nodeID.
**Note**: Both full ID and short ID are Base58 strings, the short ID only contains the first 8 bytes of the identity.

```
manas, err := goshimAPI.GetManaFullNodeID("2GtxMQD94KvDH1SJPJV7icxofkyV1njuUZKtsqKmtux5")
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

#### Get Mana of a node with its short nodeID
You can get the access and consensus mana of a node with its short nodeID.

```
manas, err := goshimAPI.GetMana("4AeXyZ26e4G")
if err != nil {
    // return error
}

// print short node ID
fmt.Println("short ID: ", manas.ShortNodeID)

// get access mana of the node
fmt.Println("access mana: ", manas.Access)

// get consensus mana of the node
fmt.Println("consensus mana: ", manas.Consensus)

```

#### Get Mana of every node
Get the mana perception of the node in the network.
You can retrieve the full/short node ID, consensus mana, access mana of each node, and the mana updated time.

```
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

#### Get Mana percentile
To learn the top percentile the node belongs to relative to the network in terms of mana.
The input should be a full node ID.

```
mana, err := goshimAPI.GetManaPercentile("2GtxMQD94KvDH1SJPJV7icxofkyV1njuUZKtsqKmtux5")
if err != nil {
    // return error
}

// mana updated time
fmt.Println("access mana percentile: ", mana.Access, "access mana updated time: ", manas.AccessTimestamp)
fmt.Println("consensus mana percentile: ", mana.Consensus, "consensus mana updated time: ", manas.ConsensusTimestamp)
```

#### Get online access/consensus mana of nodes
You can get a sorted list of online access/consensus mana of nodes, sorted from the highest mana to the lowest.
The highest mana node has OnlineRank 1, and increases 1 by 1 for the following nodes. 

```
// online access mana
accessMana, err := goshimAPI.GetOnlineAccessMana()
if err != nil {
    // return error
}

for _, m := accessMana.Online {
    fmt.Println("full node ID: ", m.ID, "mana rank: ", m.OnlineRank, "access mana: ", m.Mana)
}

// online consensus mana
consensusMana, err := goshimAPI.GetOnlineConsensusMana()
if err != nil {
    // return error
}

for _, m := consensusMana.Online {
    fmt.Println("full node ID: ", m.ID, "mana rank: ", m.OnlineRank, "consensus mana: ", m.Mana)
}

```

#### Get the top N highest access/consensus Mana nodes
You can get the N highest access/consensus mana holders in the network, sorted in descending order.

```
// get the top 5 highest access mana nodes
accessMana, err := goshimAPI.GetNHighestAccessMana(5)
if err != nil {
    // return error
}

for _, m := accessMana.Nodes {
    fmt.Println("full node ID: ", m.NodeID, "access mana: ", m.Mana)
}

// get the top 5 highest consensus mana nodes
consensusMana, err := goshimAPI.GetNHighestConsensusMana(5)
if err != nil {
    // return error
}

for _, m := consensusMana.Nodes {
    fmt.Println("full node ID: ", m.ID, "consensus mana: ", m.Mana)
}

```

#### Get the pending mana of an output
You can get the mana (bm2) that will be pledged by a given output.

```
res, err := goshimAPI.GetPending("4a5KkxVfsdFVbf1NBGeGTCjP8Ppsje4YFQg9bu5YGNMSJK1")
if err != nil {
    // return error
}

// get the amount of mana
fmt.Println("mana be pledged: ", res.Mana)
fmt.Println("the timestamp of the output (decay duration)", res.Timestamp)
```

#### Get the consensus Mana vector
Get the consensus base mana vector of a time (int64) in the past.

```
res, err := goshimAPI.GetPastConsensusManaVector(1614924295)
if err != nil {
    // return error
}

// the mana vector of each node
for _, m := range res.Consensus {
    fmt.Println("node ID:", m.NodeID, "consensus mana: ", m.Mana)
}
```
#### Get consensus event logs
Get the consensus event logs of the given node IDs.

```
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

#### Get a list of allowed mana pledge nodes
This returns the list of allowed mana pledge node IDs.

```
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