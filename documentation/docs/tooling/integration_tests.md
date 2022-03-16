---
description: Running the integration tests spins up a `tester` container within which every test can specify its own GoShimmer network with Docker.
image: /img/tooling/integration-testing.png
keywords:
- integration test
- tester
- network
- docker
- peer
- docker compose
- linux
- macOS
---
# Integration Tests with Docker

[![Integration testing](/img/tooling/integration-testing.png "Integration testing")](/img/tooling/integration-testing.png)

Running the integration tests spins up a `tester` container within which every test can specify its own GoShimmer
network with Docker as schematically shown in the figure above.

Peers can communicate freely within their Docker network and this is exactly how the tests are run using the `tester`
container. Test can be written in regular Go style while the framework provides convenience functions to create a new
network, access a specific peer's web API or logs.

## How to Run

Prerequisites:

- Docker 17.12.0+
- Docker compose: file format 3.5

```shell
# Mac & Linux
cd tools/integration-tests
./runTests.sh
```

To run only selected tests provide their names as a parameter.

```shell
./runTests.sh 'value mana'
```

The tests produce `*.log` files for every networks' peer in the `logs` folder after every run.

On GitHub logs of every peer are stored as artifacts and can be downloaded for closer inspection once the job finishes.

## Creating Tests

Tests can be written in regular Go style. Each tested component should reside in its own test file
in `tools/integration-tests/tester/tests`.
`main_test` with its `TestMain` function is executed before any test in the package and initializes the integration test
framework.

Each test has to specify its network where the tests are run. This can be done via the framework at the beginning of a
test.
```go
// create a network with name 'testnetwork' with 6 peers and wait until every peer has at least 3 neighbors
n := f.CreateNetwork("testnetwork", 6, 3)
// must be called to create log files and properly clean up
defer n.Shutdown() 
```

### Using Custom Snapshots

When creating a test's network, you can specify a set of `Snapshots` in the `CreateNetworkConfig` struct. The framework will proceed to create and render the snapshot available to the peers.
An example of a snaphot used in the code is as such:

```
var ConsensusSnapshotDetails = framework.SnapshotInfo{
	FilePath: "/assets/dynamic_snapshots/consensus_snapshot.bin",
	// node ID: 4AeXyZ26e4G
	MasterSeed:         "EYsaGXnUVA9aTYL9FwYEvoQ8d1HCJveQVL7vogu6pqCP",
	GenesisTokenAmount: 800_000, // pledged to peer master
	// peer IDs: jnaC6ZyWuw, iNvPFvkfSDp
	PeersSeedBase58: []string{
		"Bk69VaYsRuiAaKn8hK6KxUj45X5dED3ueRtxfYnsh4Q8",
		"HUH4rmxUxMZBBtHJ4QM5Ts6s8DP3HnFpChejntnCxto2",
	},
	PeersAmountsPledged: []uint64{1_600_000, 800_000},
}
```

The last parameter to the `CreateNetwork` function can be used to alter peers' configuration to use a generated snapshot file (e.g. `conf.MessageLayer.Snapshot.File = snaphotInfo.FilePath`).

The `CommonSnapshotConfigFunc` function can be used for the average scenario: it will use the same `SnapshotInfo` for all peers. 

## Nodes' Debug Tools

Every node in the test's network has their ports exposed on the host as follows: `service_port + 100*n` where `n` is the index of the peer you want to connect to.

Service ports:

* API `8080`
* Dashboard `8081`
* DAGs Visualizer `8061`
* Delve Debugger `40000`

For example for `peer_replica_2` the following ports are exposed:

* API [http://localhost:8280](http://localhost:8280)
* Dashboard  [http://localhost:8261](http://localhost:8261)
* DAGs Visualizer  [http://localhost:8281](http://localhost:8281)
* Delve Debugger  [http://localhost:40200](http://localhost:40200)

## Debugging tests

Tests can be run defining a `DEBUG=1` (e.g. `DEBUG=1 ./runTests.sh`) environment variable. The main container driving the tests will be run under a Delve Go debugger listening
on `localhost:50000`.
The following launch configuration can be used from the VSCode IDE to attach to the debugger and step through the code:

```
{
	"version": "0.2.0",
	"configurations": [
		{
			"name": "Connect to Integration tester",
			"type": "go",
			"request": "attach",
			"mode": "remote",
			"port": 50000,
			"host": "127.0.0.1"
		}
	]
}
```

> When the tester container gets connected to the test network the debugger will suffer a sudden disconnection: it is a caveat of Docker's way of doing networking. Just attach the debugger again and you are ready to go again.

### Preventing Network shutdown

When the test completes for either a PASS or a FAIL, the underlying test network is destroyed. To prevent this and give you a chance to do your thing you will have to place the breakpoint on the `tests.ShutdownNetwork` method.

## Other Tips

Useful for development is to only execute the test you're currently building. For that matter, simply modify the `docker-compose.yml` file as follows:
```yaml
entrypoint: go test ./tests -run <YOUR_TEST_NAME> -v -mod=readonly
```
