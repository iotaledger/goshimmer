# How to run a sync beacon

## What is a sync beacon?
A sync beacon is a special type of payload in the message tangle. It contains the unix timestamp of issuance of the beacon. Node owners may configure their nodes to distribute such sync beacons periodically to the network. Followers can listen to sync beacons issued by a particular set of nodes, and derive their `synced` status based on if they are able to solidify the messages containing those sync beacons.

## Why should I run a sync beacon?
Simply put: the more sync beacons there are in the network, the easier it is for followers to accurately determine their synced status.

Followers are free to choose which nodes to follow, and to what percentage of the nodes they need to be synced to, in order to consider themselves synced. The correct determination of the `synced` state is not only important for knowing the current status of your node, but also to build a healthy Tangle. You want to attach messages to recent tips, tips that are also known to others in the network, which you only know by being in sync with them.

## How to enable the sync beacon plugin?
All you need to do is enable the `syncbeacon` plugin in your node config. To do it via `config.json`, add it to the list of enabled plugins:
```
  "node": {
    "disablePlugins": [],
    "enablePlugins": ["syncbeacon"]
  },
```
You may also configure how often your node should send a sync beacon with `broadcastInterval` (seconds):
```
...
  },
  "syncbeacon": {
    "broadcastInterval": 30
  },
...
```
