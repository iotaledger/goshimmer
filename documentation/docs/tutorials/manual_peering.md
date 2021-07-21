# Manual Peering

Manual peering enables node operators to exchange their nodes' information and let them peer with each other, orthogonal to autopeering. It can be an additional protection against eclipse attacks, as the manual peering is completely in the hands of the node operator based on real world trust. Furthermore, it allows to operate nodes without exposing their IP address to the network.

There are two ways to configure the list of known peers of the node:

1. [Add known peers using the JSON config file](#how-to-set-known-peers-via-config-file)
2. [Add/View/Delete via the web API of the node](#how-to-manage-known-peers-via-web-api)

## How Manual Peering Works

A list of known peers looks like a list of IP addresses, with ports and public keys of peers. When a user provides a node with a list of known peers, the node remembers it and starts a background process that will try to connect with every peer from the list.

To establish the connection with a peer, the other peer should have our local node as a peer in its list of known peers. So the condition for peers to connect is that they should have each other in their known peers lists. In case of network failure the node will keep reconnecting with known peers until it succeeds.

In other words, the only thing that users have to do to be connected via manual peering is to exchange their IP address with port and public key.  Once they have added that information to the known peers of their nodes,  machines will do the rest.

## How to Set Known Peers via Config File

You can add the following record to the root of your JSON config file that you are using to run the node.

### Config Record Example

```json
{
  "manualPeering": {
    "knownPeers": "[{\"publicKey\": \"CHfU1NUf6ZvUKDQHTG2df53GR7CvuMFtyt7YymJ6DwS3\", \"address\": \"127.0.0.1:14666\"}]"
  }
}
```

### Config Record Description

| Field                                | Description                                        |
|:-----|:------|
| `publicKey` | Public key of the peer. |
| `address`   | IP address of the peer's node and its gossip port. |

## How to manage Known Peers via web API

For information on how to manage the known peers list via web API please see [manual peering API section](../apis/manual_peering.md) in this documentation.