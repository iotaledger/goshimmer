# v0.1.3 - 2020-03-16

* Update SPA plugin's JS dependencies
* Upgrad `github.com/gobuffalo/packr` to v2.8.0
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