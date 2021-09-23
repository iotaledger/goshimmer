---
slug: new-release-v0.7.6
title: New Release v0.7.6
author: Luca Moser
author_title: Senior Software Engineer @ IOTA Foundation
author_url: https://github.com/luca-moser
author_image_url: https://images4.bamboohr.com/86711/photos/85-4-4.jpg?Policy=eyJTdGF0ZW1lbnQiOlt7IlJlc291cmNlIjoiaHR0cHM6Ly9pbWFnZXM0LmJhbWJvb2hyLmNvbS84NjcxMS8qIiwiQ29uZGl0aW9uIjp7IkRhdGVHcmVhdGVyVGhhbiI6eyJBV1M6RXBvY2hUaW1lIjoxNjMyMzEzNjUxfSwiRGF0ZUxlc3NUaGFuIjp7IkFXUzpFcG9jaFRpbWUiOjE2MzQ5MDU2NjF9fX1dfQ__&Signature=hFmmIsMq6ixckr-MdEq-GF5sZ1kZHGl0mgpZLd3IpYznswOq9xkiNHeQk56eqZgHteIXrvvF48MOJDw~t2-5W~4gUjdXz638SShuCyUfuqEwAz8Ms68h1dloNwL7wfcN4X4TVb75u-aBcZcVguQOAL-KBj-0UUT9lJrUjfm6njSVH~ir3KdPQmFrH52UXSRBnOvjpYfKJ-2ep-izZpkWvgEDB~nOQ-ztB5WtLRxaV4EgxT8HW5O4rwDlL0N7ZLrjs5OvSJjgwvYvhSwAVIrEaiqfUY8OPVnawzCRDZ1LYSPvWWWsBjJOlNbXy6JUsBRNzY0ncKFxKZkHTPtxty1I3g__&Key-Pair-Id=APKAIZ7QQNDH4DJY7K4Q
tags: [release]
---
# GoShimmer v0.7.6

:::note
This release does **not** include changes to the consensus mechanism and still uses FPC+FCoB.
:::

:::caution
This is a **breaking** maintenance release. We urge community node operators to re-apply their config changes on
top of `config.default.json` as many config keys have changed.

Most importantly, the node's identity seed is now defined via `node.seed` (previously `autopeering.seed`). The node will
now store the private key derived from the seed per default under a separate `peerdb` database which **must not** be shared.
If the node is started with a `node.seed` different to what an existing private key in a `peerdb` contains, the node panics;
if you want to automatically override the already stored private key, use `node.overwriteStoredSeed=true`.

:::

We have also created a more streamlined deployment for our infrastructure which means that IF provided endpoints have changed:

| Service                             | Old                                | New                                                             |
| ----------------------------------- | ---------------------------------- | --------------------------------------------------------------- |
| Analysis Server                     | ressims.iota.cafe:21888            | analysisentry-01.devnet.shimmer.iota.cafe:21888                 |
| IOTA 2.0 DevNet Analyzer            | http://ressims.iota.cafe:28080    | http://analysisentry-01.devnet.shimmer.iota.cafe:28080         |
| Autopeering Entry Node "2PV5487..." | [identity]@ressims.iota.cafe:15626 | [identity]@analysisentry-01.devnet.shimmer.iota.cafe:15626 |
| LogStash Remote Log                 | ressims.iota.cafe:5213             | metrics-01.devnet.shimmer.iota.cafe:5213                        |
| Daily Database                                    | N/A                                   | https://dbfiles-goshimmer.s3.eu-central-1.amazonaws.com/dbs/nectar/automated/latest-db.tgz                                                           |

Other changes:
* Refactors codebase, mainly:
    * Usage of `time.Duration` instead of numeric types for time parameters/config options variables etc.
    * Normalization of all plugin names to camel case and without spaces
    * Extracts some code from the `MessageLayer` plugin into a new `Consensus` plugin
    * Use of dependency injection to populate plugin dependencies via https://github.com/uber-go/dig
* Adds CI workflows to use GitHub environments to deploy `develop` branch changes and releases
* Adds a check to see whether during the GoShimmer docker image build, the snapshot file should be downloaded
* Adds a new plugin `Peer` which stores the node's identity private key in a separate database
* Adds a broadcast plugin by community member @arne-fuchs simply providing a raw TCP stream of txs
* Adds a lock to prevent multiple active instances of the CLI wallet
* Adds improvements to the spammer precision
* Adds timestamp filter
* Fixes a bug where the `TxInfo` object passed to the mana booking could have a wrong `totalAmount`
* Refactors the `./tools/docker-network` to leverage docker profiles
* Updates frontend dependencies
* Updates `config.default.json` to use our new infrastructure

Plugin names have been normalized, so please make sure to adapt your custom config too:
- `Analysis-Dashboard` -> `AnalysisDashboard`
- `Analysis-Server` -> `AnalysisServer`
- `Analysis-Client` -> `AnalysisClient`
- `Autopeering` -> `AutoPeering` (no effect)
- `Graceful Shutdown` -> `GracefulShutdown`
- `Manualpeering` -> `ManualPeering` (no effect)
- `PoW` -> `POW` (no effect)
- `WebAPI autopeering Endpoint` -> `WebAPIAutopeeringEndpoint`
- `WebAPI data Endpoint` -> `WebAPIDataEndpoint`
- `WebAPI DRNG Endpoint` -> `WebAPIDRNGEndpoint`
- `WebAPI faucet Endpoint` -> `WebAPIFaucetEndpoint`
- `WebAPI healthz Endpoint` -> `WebAPIHealthzEndpoint`
- `WebAPI info Endpoint` -> `WebAPIInfoEndpoint`
- `WebAPI ledgerstate Endpoint` -> `WebAPILedgerstateEndpoint`
- `WebAPI Mana Endpoint` -> `WebAPIManaEndpoint`
- `WebAPI message Endpoint`
- `WebAPIMessageEndpoint`
- `snapshot` -> `Snapshot` (no effect)
- `WebAPI tools Endpoint` (deleted) -> `WebAPIToolsDRNGEndpoint` & `WebAPIToolsMessageEndpoint`
- `WebAPI WeightProvider Endpoint` -> `WebAPIWeightProviderEndpoint`

Config key changes `config.default.json`:
```diff
@@ -1,7 +1,7 @@
 {
   "analysis": {
     "client": {
-      "serverAddress": "ressims.iota.cafe:21888"
+      "serverAddress": "analysisentry-01.devnet.shimmer.iota.cafe:21888"
     },
     "server": {
       "bindAddress": "0.0.0.0:16178"
@@ -11,17 +11,17 @@
       "dev": false
     }
   },
-  "autopeering": {
+  "autoPeering": {
     "entryNodes": [
-      "2PV5487xMw5rasGBXXWeqSi4hLz7r19YBt8Y1TGAsQbj@ressims.iota.cafe:15626",
+      "2PV5487xMw5rasGBXXWeqSi4hLz7r19YBt8Y1TGAsQbj@analysisentry-01.devnet.shimmer.iota.cafe:15626",
       "5EDH4uY78EA6wrBkHHAVBWBMDt7EcksRq6pjzipoW15B@entry-devnet.tanglebay.com:14646"
     ],
-    "port": 14626
+    "bindAddress": "0.0.0.0:14626"
   },
   "dashboard": {
     "bindAddress": "127.0.0.1:8081",
     "dev": false,
-    "basic_auth": {
+    "basicAuth": {
       "enabled": false,
       "username": "goshimmer",
       "password": "goshimmer"
@@ -44,7 +44,7 @@
         "64wCsTZpmKjRVHtBKXiFojw7uw3GszumfvC4kHdWsHga"
       ]
     },
-    "xteam": {
+    "xTeam": {
       "instanceId": 1339,
       "threshold": 4,
       "distributedPubKey": "",
@@ -69,10 +69,7 @@
     "bindAddress": "0.0.0.0:10895"
   },
   "gossip": {
-    "port": 14666,
-    "tipsBroadcaster": {
-      "interval": "10s"
-    }
+    "bindAddress": "0.0.0.0:14666"
   },
   "logger": {
     "level": "info",
@@ -85,7 +82,7 @@
     ],
     "disableEvents": true,
     "remotelog": {
-      "serverAddress": "ressims.iota.cafe:5213"
+      "serverAddress": "metrics-01.devnet.shimmer.iota.cafe:5213"
     }
   },
   "metrics": {
@@ -103,6 +100,7 @@
     "externalAddress": "auto"
   },
   "node": {
+    "seed": "",
+    "peerDBDirectory": "peerdb",
     "disablePlugins": [],
     "enablePlugins": []
   },
@@ -110,7 +108,6 @@
     "difficulty": 22,
     "numThreads": 1,
     "timeout": "1m"
-
   },
   "profiling": {
     "bindAddress": "127.0.0.1:6061"
@@ -120,13 +117,16 @@
   },
   "webapi": {
     "bindAddress": "127.0.0.1:8080",
-    "basic_auth": {
+    "basicAuth": {
       "enabled": false,
       "username": "goshimmer",
       "password": "goshimmer"
     }
   },
+  "broadcast": {
+    "bindAddress": "127.0.0.1:5050"
+  },
   "networkdelay": {
     "originPublicKey": "9DB3j9cWYSuEEtkvanrzqkzCQMdH1FGv3TawJdVbDxkd"
   }
```