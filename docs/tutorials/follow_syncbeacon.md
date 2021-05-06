# How to configure followed sync beacon nodes

## Why do you need to follow sync beacons?
[Sync beacons](./syncbeacon.md) help your node to determine its synced status. To conclude that your node has the same view on the Tangle in a fully decentralized network, you have to rely on information coming from your peers. Sync beacons are part of the mechanism that lets you do exactly this.

You listen to sync beacons in the message Tangle from your choice of nodes, and try to solidify them. If successfully done within a predefined time window, you conclude that your node is `synced` compared to the node that issued the sync beacon. Since you can follow an arbitrary number of sync beacon nodes, your `own synced` status is determined as a quorum of your `synced` status compared to others. The quorum size (in percentage) is configurable via config file or cli flags, but has to be within the [0.5, 1.0] interval. `0.5` means that you have to be in sync at least to half of the nodes you are following, `1.0` means that you have to be synced to all of them.
## Follow a set of nodes
To configure which nodes to follow, you have to specify their public key in your node's `config.json`:
```
  "syncbeaconfollower": {
    "followNodes": [
      "98u5J7Xz4CS6efttPGYLgetZCAmpXpNiUbhAP6mmUHfW",
      "EpPTtBhkr1fAQJGTVqkE6XoSDeS5k5awEo7s4UeLSR2Y"
    ]
  },
```
If you don't specify any nodes in your config, your node will only follow two predefined sync beacon nodes.
## Configure sync quorum size
To configure the quorum size, use the `syncPercentage` config parameter:
```
  "syncbeaconfollower": {
    "followNodes": [
      "98u5J7Xz4CS6efttPGYLgetZCAmpXpNiUbhAP6mmUHfW",
      "EpPTtBhkr1fAQJGTVqkE6XoSDeS5k5awEo7s4UeLSR2Y"
    ],
    "syncPercentage": 0.8
  },
```
If you don't specify `syncPercentage` in your config, or it is outside the [0.5, 1.0] interval, the quorum size will default to `0.5` (50%).

## Observe synced status on your local dashboard
Your node dashboard will display the `synced` information related to the nodes that you follow. You can see the latest beacons which were used to derive the synced status, and the time of their issuance.

![Sync status on local dashboard](https://i.imgur.com/4QXwhyJ.png)