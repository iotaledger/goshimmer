# Manual Peering

With manual peering, it's possible to connect only with a static list of known peers that the user has provided
and don't depend on the autopeering. There are two ways to give the list of known peers to the node:

1. Using the JSON config file
2. Via the HTTP web API of the node

## How Manual Peering Works

When the user provides the list of known peers, which looks like a list of IP addresses and public keys of peers,
the node remembers it and starts a background process that tries to connect with every peer from the list.
To establish the connection with a peer, the other peer should have our local peer in its list of known peers.
So the condition for peers to connect is that they should have each other in their known peers lists.
In case of network failure the node will keep reconnecting with known peers until it succeeds.

The only thing that users have to do to be connected via manual peering is to exchange their IP address
and public key and set that information to known peers of their nodes and machines will do the rest.

## How to Set Known Peers via Config File

Add the following record to the root of your JSON config file that you use to run the node:
```json
{
 ... 
 "manualPeering": {
   "knownPeers": [
     {
       "publicKey": "<public key of the other peer>",
       "ip": "<IP address of the other peer>",
       "services": {
         "peering":{
           "network":"TCP",
           "port":14626 // autopeering port of the other peer.
         },
         "gossip": {
           "network": "TCP",
           "port": 14666 // gossip port of the other peer.
         }
       }
     }
   ]
 }, 
 ...
}
```


## How to Set Known Peers via HTTP web API

See manual peering API docs [page](./../apis/manual_peering.md)
for information on how to manage the known peers list via web API.
