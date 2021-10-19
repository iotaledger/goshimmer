---
description: In this section you can find all the information on the most recent GoShimmer releases such as breaking changes, changelogs and snapshot dates.  
image: /img/logo/goshimmer_light.png
keywords:
- release notes
- breaking changes
- consensus mechanism
- config
- FPC
- FCoB
- upgrade
---
# Release Notes

In this section you can find all the information on the most recent GoShimmer releases such as breaking changes, changelogs and snapshot dates.  

## v0.7.7 - 09/10/2021 

### Changes to Consensus Mechanism

This release does **not** include changes to the consensus mechanism and still uses FPC+FCoB.

### Breaking Changes

:::caution
This is a **breaking** maintenance release. You must delete your current database and upgrade your node to further participate in the network.
:::

### Changelog

- Changes the way the faucet plugin manages outputs in order to be able to service more funding requests.
- Fixes a nil pointer caused when a re-org is detected but the `RemoteLog` plugin is not enabled.
- Fixes an issue where the CLI wallet would no longer work under Windows.
- Use Go 1.17.2 docker image

### Snapshot Date

The snapshot has been taken at 2021-10-08 2pm CEST.


## v0.7.6 -  23/09/2021

### Changes to Consensus Mechanism

This release does **not** include changes to the consensus mechanism and still uses FPC+FCoB.

### Breaking Changes

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

### Changelog

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

## v0.7.4 - 08/07/2021


### Changes to Consensus Mechanism

This release does **not** include changes to the consensus mechanism and still uses FPC+FCoB.

### Breaking Changes 

:::warning
**Breaking changes:** You must update your GoShimmer installation if you want to keep participating in the network. Follow the steps in the [official documentation](tutorials/setup.md#managing-the-goshimmer-node-lifecycle) to upgrade your node.
:::

For those compiling from source, make sure to download the latest snapshot that we have updated. You can find more info on how to do that in the README.

We have also updated the domain name for the entry node from the community, make sure to update your config.

### Changelog

- Add ParametersDefinition structs in integration test framework
- Add UTXO-DAG interface
- Fix setting correct properties to newly forked branches
- Fix using weak parents for direct approvers when looking for transactions that are approved by a message
- Update entry node from community
- Update docs
- Update snapshot file with DevNet UTXO at 2021-07-08 07:09 UTC

### Snapshot Date

The snap was taken at 2021-07-08 07:09 UTC.