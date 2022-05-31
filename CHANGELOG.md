# v0.8.14 - 2022-05-16

> Minor revision introducing small docker changes.

The snapshot has been taken at 2022-05-07 16:30 UTC.
- Fix several Docker discrepancies (#2201, #2206)

# v0.8.13 - 2022-05-06

> This release introduces serix, a reflection-based serialization library that enables us to automatically serialize all models.

The snapshot has been taken at 2022-05-04 15:05 UTC.
- Avoid failing on duplicated unlock blocks with serix (#2197)
- Use serix library  (#2187)
- Buffer size calculation update for congestion control (#2166)
- Feat/evil spammer improvements and fixes (#2191)
- Fix bugs in evil spammer and tutorial (#2185)


# v0.8.12 - 2022-04-25

> This release introduces the evil wallet and a fix where nodes could not express their opinion on conflicts properly. 

The snapshot has been taken at 2022-03-21 15:40 UTC.
- Decrease activity message interval (#2182)
- Fix issue where liked branch is not returned when LikedConflictMember is called with the liked branch of the conflict set (#2174)
- Evil Wallet and Evil Spammer tool (#2172)
- Build(deps): bump ansi-regex from 3.0.0 to 3.0.1 in /documentation (#2173)
- Build(deps): bump async from 2.6.3 to 2.6.4 for all frontends (#2171)
- Build(deps): bump minimist from 1.2.5 to 1.2.6 in /plugins/dashboard/frontend (#2152)
- Build(deps): bump moment from 2.29.1 to 2.29.2 in dagsvisualizer and dashboard (#2150)

# v0.8.11 - 2022-03-23

> This release upgrades to Go 1.18 and fixes a bug that rendered tips unreferenceable.

The snapshot has been taken at 2022-03-21 03:30 UTC.
- Drop tips referencing unreferenceable branches #2132
- Build(deps): bump lodash-es from 4.17.15 to 4.17.21 in /plugins/dashboard/frontend (#2125)
- Switch to Go1.18 (#2131)
- Use goreleaser supporting Go 1.18rc1 (#2122)

# v0.8.10 - 2022-03-14

> This release introduces generics, the time since confirmation check as well as several fixes to merge to master functionality.

The snapshot has been taken at 2022-03-11 08:30 UTC.
- Fix Docker network defaults and increase TSC threshold (#2117)
- Feat: Validate TimeSinceCofirmation of a parent  (#1767)
- Improve Integration tests experience (#2109)
- Use generic data structures and object storage. (#2051)
- TSA: prevent Branch censoring with dislike references (#2083)
- Fix concurrent map read/write (#2098)
- Remove empty section from wiki menu (#2089)
- Remove MessageIDsSlice (#2088)
- Remove aggregated branches (#2070)
- Introduce specificialized Shallow Approvers (#2071)
- Re-enable feature network debugger (#2072)

# v0.8.9 - 2022-03-04

> This release introduces a critical bug fix that could prevent nodes from running out of sync.

The snapshot has been taken at 2022-03-02 20:30 UTC.
- CRITICAL Fix: HandleMarker Confirmation (#2090)

# v0.8.8 - 2022-03-01

> This release introduces a major bug fix that could prevent nodes from running out of sync.

The snapshot has been taken at 2022-02-26 19:30 CET.
- Remove booker & issuance locks (#2084)
- Build(deps): bump url-parse from 1.5.7 to 1.5.10 in /plugins/dashboard/frontend (#2081)
- Add issuerID to remote metrics (#2042)

# v0.8.7 - 2022-02-24

> This release introduces several small bug fixes and improvements.

The snapshot has been taken at 2022-02-21 17:30 CET.
- Try to adjust rate limit better (#2053)
- /healthz to reflect Synced status (#2075)
- clean test cache (#2045)
- expose port ES to outside (#2052)
- Build(deps): bump url-parse from 1.5.3 to 1.5.7 in dashboards and DAGs visualizer (#2059)
- DAGs Vis: Add legend (#2056)
- Check vertex data before accessing it in graphs (#2054)
- Build(deps): bump follow-redirects from 1.14.7 to 1.14.8 in /plugins/analysis/dashboard/frontend (#2041)
- Build(deps): bump follow-redirects from 1.14.7 to 1.14.8 in /plugins/dashboard/frontend (#2040)
- DAGs Vis: color dags on confirmation and hide aggr branches (#2037)
- Fix output link to dashboard (#2044)

# v0.8.6 - 2022-02-11

> This release introduces the "Merge to Master" functionality to avoid propagation of Confirmed branches to the future cone and introduces an amazing new DAGs Visualizer tool! The new visualizer is exposed on port 8061 by default, check it out!

The snapshot has been taken at 2022-02-11 10:30 CET.
- Feat: Merge confirmed Branches with the MasterBranch (#1770)
- Fix: disable REMOTE_DEBUGGING on feature network, hangs the entrynode (#2034)
- Mark markers in DAGs visualizer (#2032)
- Update go.mod to point to hive.go in master branch (#2028)
- Remove node-sass package from analysis dashboard (#2027)
- Build(deps): bump follow-redirects from 1.14.2 to 1.14.8 in /plugins/dagsvisualizer/frontend (#2026)
- Build(deps-dev): bump node-sass in /plugins/analysis/dashboard/frontend (#2017)
- Implement DAGs Visualizer (#2014)
- Docs - Add Test Build Action (#1994)
- Build(deps): bump simple-get from 3.1.0 to 3.1.1 in /plugins/dashboard/frontend (#2005)
- Skip snapshot download when rebuilding images of old goshimmer version (#1959)
- Spam protection  (#1990)
- Do not hardcode parameters in Docker ENTRYPOINT (#1992)
- Add json and shell annotations for better readability (#1983)
- Fix browser not popping up the basic auth login modal  (#1986)
- Build(deps): bump nanoid from 3.1.23 to 3.2.0 in /plugins/dashboard/frontend (#1980)
- Do not increase the scheduling rate when node is out of sync. (#1961)
- Update webAPI.md (#1981)
- Added section on how to install goshimmer from source (#1935)
- fix: lock scheduler metrics map when writing (#1974)
- Documentation release.md: merge into master without squashing (#1972)

# v0.8.5 - 2022-01-19

> This release introduces minor bug fixes and improvements.

The snapshot has been taken at 2022-01-13 22:30 CET.

- Improve grafana dashboard (#1967)
- Add recieved time remote metric (#1965)
- Build(deps): bump follow-redirects from 1.13.0 to 1.14.7 in node and analysis dashboard (#1962)
- Fix booker time metric (#1963)
- Revert "Default to automated snapshot on Docker image creation (#1953)" (#1956)
- Build(deps-dev): bump postcss from 8.2.10 to 8.2.13 in /plugins/dashboard/frontend (#1951)

# v0.8.4 - 2022-01-12

> This release introduces a complete Congestion Control overhaul and several bug fixes.

The snapshot has been taken at 2022-01-12 17:31 CET.

Changelog:
- Build and push images tagged with version number (#1950)
- New Scheduler buffer management & data flow (#1856)
- Remove spam debug log about duplicate bytes (#1949)
- Snapshot nil pointer dereference fix (#1943)
- Devnet AnalysisServer should use LBed dashboards WS (#1942)
- changes to spammer tool (#1854)
- Fix/feature network port forwarding problem (#1940)
- Fix node identity problem after restart (#1939)
- Update aMana update formula to fix NaN problem (#1930)
- Customize PoW difficulty on feature network (#1937)
- Fix prometheus scrape ports (#1928)
- Fix feature network deployment. (#1925)
- Don't log errors on write to closed connection (#1915)
- Fix mana integration test (#1909)
- Improve integration tests (#1906)
- Add missing prometheus.yml file and improve volumes in docker-compose files. (#1899)
- Internal feature network support (#1895)

# v0.8.3 - 2021-11-30

> This release introduces a critical bug fix on the network read loop

The snapshot has been taken at 2021-11-28 11:31am CET.

Changelog:
- Critical fix on the lib2p's stream readloop.

# v0.8.2 - 2021-11-26

> This release introduces a revamp of the network stack using libp2p, and some minor changes to the deployment logic

The snapshot has been taken at 2021-11-19 11:37pm CET.

Changelog:
- Use libp2p as network stack.
- Fix deployment logic to expose DRNGs' API ports.
- Add community entry nodes.

# v0.8.1 - 2021-11-11

> This release introduces some minor changes to the message solidification and requesting mechanisms.

The snapshot has been taken at 2021-11-05 12:18pm CET.

Changelog:
- There is now a 1% chance that an inbound request for a missing message gets relayed to neighbors to get resolved.
- Messages requests are enqueued for retry with a random jitter.
- Use of TimedExecutor for the message requester.
- Better logging for message requester and filters.
- Messages that could not be retrieved and get removed from the requester can be requested another time later.
- Fixed analysis dashboard.

# v0.8.0 - 2021-11-05

> This release introduces changes to the consensus mechanism. Specifically, a first implementation of pure On Tangle Voting (OTV), like switch, and the Grades of Finality (GoF) is included. This release does not entail algorithmic optimizations of these components. Therefore, it is to be expected, that performance degrades over time.

This is a **breaking** feature release. You must delete your current database and upgrade your node to further participate in the network. Applications that rely on confirmation need to adjust to be compatible with the newly introduced Grades of Finality.

The snapshot has been taken at 2021-11-05 12:18pm CET.

Changelog:
- Add caching to CI operations
- Upgrade hive.go and adjust codebase to use context for cancellation instead of channels
- Add LRU cache to BranchDAG
- Add new consensus mechanism
  - Implement pure On Tangle Voting (OTV)
  - Remove FCOB, FPC and timestamp voting with FPC from the dataflow
  - Implement like switch
    - Make message layout more flexible and add parent blocks to accommodate like switch
    - Message syntactical validation
    - Adjust tip selection and message creation for like switch
    - Adjust booking of messages to execute like switch logic when inheriting branches
  - Grades of Finality (GoF)
    - Marker confirmation with like switch
    - Remove Liked / Finalized / InclusionState references
    - Remove deprecated consensus manager and events
    - Inherit markers from strong and like parents
  - Add conflicts page for node dashboard
  - Adjust all unit and integration tests to use GoF
  - Add new consensus integration tests
  - Fix cMana weight provider with activity log for each node
  - Add OTV metrics collection
  - Optimize tracking of AW for branches and markers

# v0.7.7 - 2021-10-09

> This release does **not** include changes to the consensus mechanism and still uses FPC+FCoB.

This is a **breaking** maintenance release. You must delete your current database and upgrade your node to further participate in the network.

The snapshot has been taken at 2021-10-08 2pm CEST.

Changelog:
- Changes the way the faucet plugin manages outputs in order to be able to service more funding requests. 
- Fixes a nil pointer caused when a re-org is detected but the `RemoteLog` plugin is not enabled.
- Fixes an issue where the CLI wallet would no longer work under Windows.
- Use Go 1.17.2 docker image

# v0.7.6 - 2021-09-23

> This release does **not** include changes to the consensus mechanism and still uses FPC+FCoB.

This is a **breaking** maintenance release. We urge community node operators to re-apply their config changes on
top of `config.default.json` as many config keys have changed.

Most importantly, the node's identity seed is now defined via `node.seed` (previously `autopeering.seed`). The node will
now store the private key derived from the seed per default under a separate `peerdb` database which **must not** be shared.
If the node is started with a `node.seed` different to what an existing private key in a `peerdb` contains, the node panics;
if you want to automatically override the already stored private key, use `node.overwriteStoredSeed=true`.

We have also created a more streamlined deployment for our infrastructure which means that IF provided endpoints have changed:

| Service                             | Old                                | New                                                        |
| ----------------------------------- | ---------------------------------- | ---------------------------------------------------------- |
| Analysis Server                     | ressims.iota.cafe:21888            | analysisentry-01.devnet.shimmer.iota.cafe:21888            |
| IOTA 2.0 DevNet Analyzer            | http://ressims.iota.cafe:28080     | http://analysisentry-01.devnet.shimmer.iota.cafe:28080     |
| Autopeering Entry Node "2PV5487..." | [identity]@ressims.iota.cafe:15626 | [identity]@analysisentry-01.devnet.shimmer.iota.cafe:15626 |
| LogStash Remote Log                 | ressims.iota.cafe:5213             | metrics-01.devnet.shimmer.iota.cafe:5213                   |
| Daily Database                      | N/A                                | https://dbfiles-goshimmer.s3.eu-central-1.amazonaws.com/dbs/nectar/automated/latest-db.tgz |

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

# v0.7.5 - 2021-08-09
* Move the scheduler of the congestion control at the end of data flow
* Add new metrics collection
* Add alias initial state validation
* Add creation of new marker sequence after maxVerticesWithoutFutureMarker threshold
* Fix booking of transactions in multiple conflict sets
* Fix manual peering parsing from config.json
* Update JS dependencies
* Update Dockerfile to explicitly expose used ports
* Update docs to docusaurus
* Update snapshot file with DevNet UTXO at 2021-08-09 13:33 UTC
* **Breaking**: bumps network and database versions

# v0.7.4 - 2021-07-08
* Add ParametersDefinition structs in integration test framework
* Add UTXO-DAG interface
* Fix setting correct properties to newly forked branches
* Fix using weak parents for direct approvers when looking for transactions that are approved by a message
* Update entry node from community
* Update docs
* Update snapshot file with DevNet UTXO at 2021-07-08 07:09 UTC
* **Breaking**: bumps network and database versions

# v0.7.3 - 2021-07-01
* Add latest snapshot download from S3
* Add weak tips on local dashboard
* Improve integration test framework
* Add FCoB metrics to remote logger
* Remove tips broadcaster
* Improve APIs
* Fix Sync status
* Fix gossip and requester issues
* Fix Inclusion state inconsistencies
* Fix Prometheus issues when autopeering is not enabled
* Increase maxVerticesWithoutFutureMarker to 3000000
* Update snapshot file with DevNet UTXO at 2021-06-30 21:16 UTC
* Update JS dependencies
* Update hive.go
* **Breaking**: bumps network and database versions

# v0.7.2 - 2021-06-17
* Add local double spend filter in webAPI
* Add SetBranchFinalized to FCoB
* Set message of the weak parents finalized rather than only their payload
* Update snapshot file with DevNet UTXO at 2021-06-17 08:52 UTC
* Update JS dependencies
* **Breaking**: bumps network and database versions

# v0.7.1 - 2021-06-15
* Improve Faucet
* Improve docs
* Improve message requester
* Improve common integration test
* Improve collected Elasticsearch metrics
* Add run callback to database plugin
* Add Client Library Import Check
* Add time range check in wallet lib
* Replace builtin hash map with a sorted set
* Enable Github Actions Caching
* Fix wrong account mention
* Fix gossip neighbor disconnect
* Fix Genesis Loaded Transaction View
* Fix memory leak in pow plugin/pkg
* Fix overlocking and sleeping issues 
* Fix snapshot bug
* Fix typos
* Update Graphana debugging dashboard
* Update snapshot file with DevNet UTXO at 2021-06-15 09:09 UTC
* Update JS dependencies
* **Breaking**: bumps network and database versions

# v0.7.0 - 2021-06-02
* Add client diagnostic API
* Improve docs and tutorials
* Improve congestion control
* Improve dashboard
* Improve Grafana dashboard
* Limit chat fields length
* Fix several bugs
* Reset snapshot file
* Update JS dependencies
* **Breaking**: bumps network and database versions

# v0.6.4 - 2021-05-29
* Add simple chat dApp
* Improve client lib
* Improve manual peering and add tutorial
* Improve docs and tutorials
* Improve congestion control
* Fix mana dashboard deadlock
* Fix several bugs
* Update snapshot file with Pollen UTXO at 2021-05-29 10:22 UTC
* Update JS dependencies
* Update lo latest hive.go
* **Breaking**: bumps network and database versions

# v0.6.3 - 2021-05-25
* Improve congestion control
* Fix builds
* Fix dRNG shutdown deadlock
* Fix panic on booking
* Fix locking management on message factory
* Fix several visualizer bugs
* Update Grafana pie charts
* Update snapshot file with Pollen UTXO at 2021-05-25 16:54 UTC
* **Breaking**: bumps network and database versions

# v0.6.2 - 2021-05-24
* Add Congestion Control
* Add global snapshot
* Add Central Asset Registry to cli-wallet
* Switch to RocksDB instead of BadgerDB
* Enhance manual peering
* Improve gossip
* Improve APIs
* Fix several dashboard bugs
* Update lo latest hive.go
* Update snapshot file 
* **Breaking**: bumps network and database versions

# v0.6.1 - 2021-05-19
* Change Faucet default parameters
* **Breaking**: bumps network and database versions

# v0.6.0 - 2021-05-19
* Add first iteration of the tokenization framework
* Add aMana refresher
* Add CORS to middleware Dashboard server
* Allow specifying parents count when issuing messages 
* Move PoW for Faucet requests to client
* Remove Faucet requests from Node's Dashboard
* Improve cli-wallet
* Improve APIs
* Improve integration tests
* Refactor statement remote logging  
* Display full messageID in explorer feed
* Update JS dependencies
* Update documentation
* **Breaking**: bumps network and database versions

# v0.5.9 - 2021-05-11
* Replace sync beacons with Tangle Time
* Fix approval weight manager persistence
* Fix non positive ticker
* Fix marker issues
* Fix solidification issues
* Fix concurrency-related issues
* Improve FPC metrics logging
* Improve clock synchronization handling
* Improve dRNG plugin
* Improve integration tests
* Update JS dependencies
* Update to latest hive.go
* Update documentation
* **Breaking**: bumps network and database versions

# v0.5.8 - 2021-05-07
* Integrate FPC with the X-Team committee
* Enable finality via approval weight
* Add Tangle Time
* Add sync status monitoring
* Add activity plugin
* Add manual peering support
* Add markers info to message view
* Remove moving average for cMana
* Replace epochs with weight provider using Tangle Time
* Fix marker issues
* Fix sync issues
* Disable past-cone check when booking transactions
* Improve integration tests
* Refactor error handling
* Update snapshot
* Update JS dependencies
* Update to latest hive.go
* Update documentation
* **Breaking**: bumps network and database versions

# v0.5.7 - 2021-04-23
* Add approval weight manager (soft launch)
* Add epochs
* Add debug APIs for epochs
* Update local dashboard to show finalization based on approval weight
* Improve FPC
* Improve markers manager
* Improve integration tests
* Improve payload unmarshaling
* Remove unless-stopped option from Docker default config
* Increase CfgGossipAgeThreshold parameter
* Fix several bugs on hive.go
* Fix mana event storage pruning
* Fix mana leaderboard and explorer live feed scroll view
* Update snapshot with initial mana state
* Update to latest hive.go
* **Breaking**: bumps network and database versions

# v0.5.6 - 2021-04-03
* Fix childBranchType
* Fix FPC empty round increase
* Make reading of FPC statements less strict
* Fix aggregated branch diagnostic API and dashboard page  
* **Breaking**: bumps network and database versions

# v0.5.5 - 2021-04-01
* Integrate Mana with FPC
* Integrate Mana with the Autopeering
* Add several FPC optimizations
* Add dRNG diagnostic API
* Simplify memory usage of dashboard and align to Grafana
* Add a chart for stored, solidifier, scheduler and booker MPS
* Update to latest hive.go
* **Breaking**: bumps network and database versions

# v0.5.4 - 2021-03-29
* Add new diagnostic APIs
* Add new docs sections
* Add branch inclusion state check before issuing new transactions
* Refactor the Faucet plugin
* Optimize transaction's past cone check
* Make issued messages pass through the parser filters
* Fix Faucet time usage
* Fix markers issue
* Fix max inputs count check
* Fix nil pointer in diagnostic API
* Update to latest hive.go 
* Enhance golangci-lint
* **Breaking**: bumps network and database versions

# v0.5.3 - 2021-03-25
* Added new API endpoints
* Added models navigation through the Dashboard Explorer
* Added new diagnostic APIs
* Added new docs sections
* Fix dashboard mana event feed
* Fix markers issue
* Fix UnlockBlocks check
* Fix loading of config parameters
* Fix bug in the Get method of ColoredBalances
* Enhance golangci-lint
* **Breaking**: bumps network and database versions

# v0.5.2 - 2021-03-17
* Fix markers past cone check
* Add more information to explorer and message API
* Restrict Genesis attachment
* Fix parsing of GenesisNode public key
* Display mana histogram in log scale on dashboards
* **Breaking**: bumps network and database versions

# v0.5.1 - 2021-03-15
* Implement FCoB*
* Fix markers persistence bug
* Fix Docker shutdown too early
* Make FCoB fire the MessageOpinionFormed event only if the event for its parents was also fired
* Update JS dependencies
* Refactor parameters in MessageLayer
* Upgrade go to 1.16.2
* Upgrade to latest hive.go
* **Breaking**: bumps network and database versions

# v0.5.0 - 2021-03-11
* Add Mana (currently not used by any of the modules)
* Add Mana APIs
* Add Mana section to the local dashboard
* Add Mana section to the Pollen Analyzer dashboard
* Add Mana section to the Grafana dashboard
* Refactor the Consensus Manager to be independent from the concrete consensus mechanism implemented  
* Improve Tangle visualizer
* Improve documentation
* **Breaking**: bumps network and database versions

# v0.4.1 - 2021-03-02
* Add orphanage analysis tool
* Add documentation web-api
* Simplify past-cone checks
* Improve message findByID API
* Improve value transactionByID API
* Fix saving markers in message metadata
* Fix issue with ReferenceUnlockBlocks
* Fix Faucet address blacklist
* Fix some visualizer glitches
* **Breaking**: bumps network and database versions

# v0.4.0 - 2021-02-26
* Remove the value Tangle
* Add approval switch
* Add Markers integration
* Add new message booker
* Add new ledger state
* Add new opinion former
* Add new Tip manager
* Add new flow unit tests
* Add remote spammer tool
* Add sendTransaction timestamp validity check
* Add clock Since method
* Add docs actions
* Add Tangle width debug tool option
* Update client-lib
* Update wallet
* Update web-API
* Update Grafana dashboard to show MPS for entire data-flow
* Update to latest hive.go
* Refactor integration tests
* Refactor transaction validity check
* Refactor cli-wallet
* Refactor sendTransactionByJson API
* Refactor inclusion state
* Refactor TransactionConfirmed event
* Refactor transaction visualization in the local dashboard
* Refactor scheduler
* Refactor snapshot script
* Fix clock time usage
* Fix wrong handler in the MessageInvalidEvent
* Decrease cache time
* **Breaking**: bumps network and database versions

# v0.3.6 - 2021-02-12
* Finalize Payload layout
* Update dRNG with finalized payload layout
* Add simple scheduler
* Add message approval analysis API
* Add core logic for timestamp voting (still disabled)
* Refactor message parser
* Refactor solidifier
* Refactor Tangle events
* Update entry node URL
* Update gossip to not gossip requested messages
* Introduce invalid message flag
* Merge the new data flow
* Fix visualizer bug
* Update hive.go
* Update JS dependencies
* **Breaking**: bumps network and database versions

# v0.3.5 - 2021-01-13
* Fix consensus statement bug
* Fix deadlock in RandomMap
* Fix several shutdown related issues
* Load old messages in visualizer
* Fix wrong Tips count in visualizer
* Fix dashboard typos
* Improve integration tests
* Improve network delay analysis
* Update hive.go
* Update JS dependencies
* **Breaking**: bumps network and database versions

# v0.3.4 - 2020-12-11
* Revert Pebble to Badger.
* **Breaking**: bumps network and database versions

# v0.3.3 - 2020-12-10
* Fix sync issue.
* Fix pkger issue.
* **Breaking**: bumps network and database versions

# v0.3.2 - 2020-12-09
* Switch from BadgerDB to Pebble.
* Add FPC statements.
* Add clock based time to message metadata.
* Improve dashboard message live feed.
* Improve spammer to evenly distribute issued messages within a minute.
* Fix panic when writing on a closed channel in the dashboard web socket.
* Upgrade Go to 1.15.5
* Upgrade to latest hive.go
* **Breaking**: bumps network and database versions

# v0.3.1 - 2020-11-13
* Refactor message structure according to the new Tangle RFC: 
    * add support for multiple parents
    * update local dashboard
    * new unit tests
    * max payload size changed to 65157 bytes
* Add community-based entry node.
* Add commit tag to version.
* Add package for common sentinel errors.
* Improve dashboard websocket management.
* Integrate NTP-based clock to the network delay app.
* Switch from packer to pkger to pack dashboard.
* Switch from Viper to koanf as core library for configuration.
* Fix Value Tangle tip selection management.
* Fix mps query label in grafana.
* Fix potential race condition within the clock package.
* Upgrade to latest hive.go
* Upgrade NodeJS dependencies of the dashboard.
* **Breaking**: bumps network and database versions

# v0.3.0 - 2020-10-12
* Added multiple dRNG committees support: Pollen, X-Team and Custom.
* Added clock synchronization plugin via NTP.
* Added basic codeQL analysis pipeline for common vulnerability scanning.
* Added basic HTTP authentication.
* Changed payload layout to be more similar to the one specified for Chrysalis phase 2.
* Improved rand-seed tool by writing its output to a file.
* Improved the Docker network by making MongoDB, Grafana and Prometheus optional so that startup/shutdown times are low when not needed.
* Upgraded to the latest hive.go.
* Upgraded NodeJS dependencies of the dashboard.
* Fixed several security issues.
* Refactored the entire code base to make its package structure flat and more consistent with Hornet.
* Moved data structures to hive.go 
* Removed JWT authentication due to security issues of the package dgrijalva/jwt-go
* **Breaking**: bumps network and database versions

# v0.2.4 - 2020-09-03
* Fixes race condition that was preventing the deletion of some entries from missing messages.
* Improves the Tangle-BadgerDB interaction.
* Improved APIs for debug with the addition of the value-tips endpoint.
* Improved autopeering management by adding the ability to specify a given network version.
* Integrates initial support for the dRNG module.
* **Breaking**: bumps network and database versions

# v0.2.3 - 2020-08-11
* Fixes synchronization issue where missing messages were not requested more than once
* Fixes node's dashboard explorer crashing or not properly visualizing the payload of a given message
* Improves Grafana local dashboard:
    * Adds support for the sync-beacon payload type
    * Displaying uptime and nodeID
* Fixed all linter issues to improve code quality
* **Breaking**: bumps network and database versions

# v0.2.2 - 2020-07-27
* Improves message and transaction validation: 
    * Adjust max transaction inputs count;
    * Adds signature validation before issuance; 
    * Enforce max message size in message factory.
* Improves API:
    * Changes granularity of spammer API to accept messages per minute;
    * Adds API middleware and set CORS to allow from every origin;
    * Adds sendTransactionByJSON to allow a client to issue transactions by providing them in a JSON format;
    * Adds tool API endpoint to facilitate debugging of the solidification status;
    * Removes old API documentation;
* Improves synchronization process:
    * Refactors message requester to be more reliable;
    * Improves solidification process;
    * Refactors worker pool management;
    * Replaces bootstrap plugin with the more secure and reliable beacon plugin.
* Improves analysis-server dashboard:
    * Adds the ability to distinguish between different GoShimmer node versions;
    * Refactors the interaction between server side and dashboard;
    * Improves consensus visualization;
    * Improves dashboard tooling.
* Adds a new electron-based wallet.
* Increases max gossip packet size.
* Adds command to the CLI to override database dirty flag.
* Grafana local dashboard
    * Adds messages in database chart (solid, not solid, total)
    * Adds average solidification time chart
    * Adds Message Request Queue Size chart
* **Breaking**: bumps network and database versions

# v0.2.1 - 2020-07-01
* Adds PoW requirement to faucet payloads
* Adds tips broadcaster to ensure that all chains are getting solidified
* Fixes being able to send a double-spend via one node
* **Breaking**: bumps network and database versions

# v0.2.0 - 2020-06-30
* Adds the value transfer dApp:
    * New binary transaction layout
    * UTXO model 
    * Support for transactions with Ed25519 and BLS signatures
    * Parallel reality based ledger state
    * Support for colored tokens
    * Conflict resolution via FPC
    * Applied FCoB rules
* Adds the network delay dApp which is used to gather the avg. network delay occurring in the network
* Adds the faucet dApp giving the ability to request funds for testing via the dashboard or web API
* Adds the DRNG dApp which is used to propagate random numbers produced by a dRand committee (this dApp is inactive)
* Adds the base communication layer
* Adds improved analysis server:
    * Splits it into 3 separate plugins `analysis-client`/`analysis-dashboard`/`analysis-server`
    * Applies heartbeat pattern for robustness on both client and server side
    * Uses TLV denoted messages for communication
    * Complete new dashboard with live visualisations of the network graph and ongoing conflicts
    * Use short node IDs throughout the analysis dashboard
    * Prometheus exporter to expose global network metrics
    * Storage for conflicts inside a MongoDB for further processing
    * Complete rewritten autopeering data retention
* Adds additional HTTP API routes:
    * "healtz" route to query the health of the node (for LBs)
    * Query transaction attachment locations
    * Query transactions by IDs
    * Send transactions
    * Get UTXOs on addresses
    * Query info about the node
    * Issue a faucet funding request
    * Query dRNG committee
    * Query dRNG random number
    * Issue data payloads
* Adds dashboard improvements:
    * Tips chart
    * Communication layer visualizer
    * Address and UTXOs view
    * Message payload view
    * DRNG live feed
    * Faucet page to request funds
    * Support different payload views (data, value, faucet, unknown)
* Adds integration test framework and corresponding tests:
    * Autopeering/Network Split
    * Message propagation 
    * FPC 50/50 network split voting
    * Faucet funding
    * Value transfers
    * Synchronization
* Adds refactored communication layer code
* Adds BLAKE2-based PoW for rate control
* Adds rewritten FPC package
* Adds possibility to change config options with environment variables
* Adds sample Grafana dashboard for local node instances
* Adds snapshot-file import
* Adds "dirty-flag" to the `Database` plugin to check for corrupted databases
* Adds BadgerDB gargbage collection on startup/shutdown and removes periodic GC
* Adds review-dog linter and automatic integration tests execution to continuous integration pipeline
* Adds `Prometheus` exporter plugin with exposure for [following metrics](https://github.com/iotaledger/goshimmer/issues/465)
* Adds `Sync` plugin keeping track of the node's synchronization state
* Adds `Issuer` plugin which takes care of issuing messages (and blocking any issuance when the node is desynced)
* Adds `Profiling` plugin which exposes the `pprof` endpoints and is now configurable
* Adds `Bootstrap` plugin which continuously issues messages to keep the comm. layer graph growing
* Adds proper metrics collection in the `Metrics` plugin
* Adds support for the new HTTP API routes to the Go client library
* Adds `tools/docker-network` to run an isolated GoShimmer network with a chosen amount of nodes. Predefined local and global grafana dashboards are spinned up as well for the network
* Upgrades `hive.go` with various improvements and fixes
* Fixes bind address prints to not be normalized
* Fixes usage of WebSocket message rate limiter
* Fixes disabled/enabled plugins list in info response
* Fixes the `Graceful Shutdown` plugin not showing the actual background workers during shutdown. The node operator is now
able to see pending background workers in the order in which they are supposed to be terminated
* Refactors display of IDs to use Base58 throughout the entire codebase
* Increases network and database versions
* Removes usage of non std `errors` package
* Removes `Graph` plugin
* Renames `SPA` plugin to `Dashboard`
* Makes running GoShimmer with a config file mandatory if not explicitly bypassed

# v0.1.3 - 2020-03-16

* Update SPA plugin's JS dependencies
* Upgrade `github.com/gobuffalo/packr` to v2.8.0
* Resolves a security issue in the packr dependency, which would allow an unauthenticated attacker to read any
 file from the application's filesystem 

# v0.1.2 - 2020-02-24

* Adds `--version` flag to retrieve the GoShimmer version
* Adds the version and commit hash to the remote log logging
* Replaces the autopeering module with the one from hive.go 
* Changed the pprof listen port to `6061` to avoid conflict with Hornet
* Fixes `invalid stored peer` messages
* Fixes masternodes getting removed if they were offline
* Fixes `-c` and `-d` to define config file/dir
* Fixes drop messages about full queues appearing too many times
* Fixes crash due to incopatible transaction size
* Changed the salt lifetime to 2 hours from 30 minutes

# v0.1.1 - 2020-02-07

This release contains a series of fixes:
* Adds logging of the underlying error when a neighbor connection couldn't be established
* Adds the `RemoteLog` plugin (disabled per default) which sends log messages to a centralized logging service
* Removes the status screen plugin in favor of the SPA/dashboard
* Fixes the neighbor send queue being too small causing spam in the log
* Fixes a deadlock which occurred when a neighbor disconnected while at the same time a `request transaction` 
packet was getting processed which was received by the disconnected neighbor
* Fixes memory consumption by disabling BadgerDB's compression
* Fixes analysis server WebSocket replays freezing the backend code
* Fixes sending on the neighbor send queue if the neighbor is disconnected
* Fixes `BufferedConnection`'s read and written bytes to be atomic

# v0.1.0 - 2020-01-31

> Note that this release is a complete breaking change, therefore node operators are instructed to upgrade.

This release mainly integrates the shared codebase between Hornet and GoShimmer called [hive.go](https://github.com/iotaledger/hive.go), 
to foster improvements in both projects simultaneously. Additionally, the autopeering code from [autopeering-sim](https://github.com/iotaledger/autopeering-sim)
has been integrated into the codebase. For developers a Go client library is now available to communicate
with a GoShimmer node in order to issue, get and search zero value transactions and retrieve neighbor information.

A detailed list about the changes in v0.1.0 can be seen under the given [milestone](https://github.com/iotaledger/goshimmer/milestone/1?closed=1).

* Adds config keys to make bind addresses configurable
* Adds a `relay-checker` tool to check transaction propagation
* Adds database versioning
* Adds concrete shutdown order for plugins
* Adds BadgerDB garbage collection routine
* Adds SPA dashboard with TPS and memory chart, neighbors and Tangle explorer (`spa` plugin)
* Adds option to manually define autopeering seed/private keys
* Adds Golang CI and GitHub workflows as part of the continuous integration pipeline
* Adds back off policies when doing network operations
* Adds an open port check which checks whether NATs are configured properly
* Adds `netutil` package
* Adds the glumb visualizer backend as a plugin called `graph` (has to be manually enabled, check the readme)
* Adds rudimentary PoW as a rate control mechanism
* Adds the autopeering code from autopeering-sim
* Adds association between transactions and their address
* Adds the possibility to allow a node to function/start with gossiping disabled
* Adds buffered connections for gossip messages
* Adds an OAS/Swagger specification file for the web API
* Adds web API and refactors endpoints: `broadcastData`, `findTransactionHashes`,
`getNeighbors`, `getTransactionObjectsByHash`, `getTransactionTrytesByHash`, `getTransactionsToApprove`, `spammer`
* Adds a Go client library over the web API
* Adds a complete rewrite of the gossiping layer
* Fixes the autopeering visualizer to conform to the newest autopeering changes.  
  The visualizer can be accessed [here](http://ressims.iota.cafe/).
* Fixes parallel connections by improving the peer selection mechanism
* Fixes that LRU caches are not flushed correctly up on shutdown
* Fixes consistent application naming (removes instances of sole "Shimmer" in favor of "GoShimmer")
* Fixes several analysis server related issues
* Fixes race condition in the PoW code
* Fixes race condition in the `daemon` package
* Fixes several race conditions in the `analysis` package
* Fixes the database not being closed when the node shuts down
* Fixes `webauth` plugin to function properly with the web API
* Fixes solidification related issues
* Fixes several instances of overused goroutine spawning
* Upgrades to [BadgerDB](https://github.com/dgraph-io/badger) v2.0.1
* Upgrades to latest [iota.go](https://github.com/iotaledger/iota.go) library
* Removes sent count from spammed transactions
* Removes usage of `errors.Identifiable` and `github.com/pkg/errors` in favor of standard lib `errors` package
* Use `network`, `parameter`, `events`, `database`, `logger`, `daemon`, `workerpool` and `node` packages from hive.go
* Removes unused plugins (`zmq`, `dashboard`, `ui`)
