# Integration Tests with Docker

![Integration testing](../../static/img/tooling/integration-testing.png "Integration testing")

Running the integration tests spins up a _tester_ container within which every test can specify its own GoShimmer network with Docker, as schematically shown in the figure above.

Peers can communicate freely within their Docker network, and this is exactly how the tests are run using the `tester` container.

A test can be written in regular Go style while the framework provides convenience functions to create a new network, access a specific peer's web API or logs.

## How To Run

Prerequisites:

- Docker 17.12.0+
- Docker compose: file format 3.5

```bash
# Mac & Linux
cd tools/integration-tests
./runTests.sh
```

The tests produce `*.log` files for every networks' peer in the `logs` folder after every run.

On GitHub, logs of every peer are stored as artifacts, and can be downloaded for closer inspection once the job finishes.

## Creating Tests

Tests can be written in regular Go style. Each tested component should reside in its own test file in `tools/integration-tests/tester/tests`.
`main_test` with its `TestMain` function is executed before any test in the package and initializes the integration test framework.

Each test has to specify its network where the tests will be run. This can be done via the framework at the beginning of a test.

```go
// create a network with name 'testnetwork' with 6 peers and wait until every peer has at least 3 neighbors
n := f.CreateNetwork("testnetwork", 6, 3)
// must be called to create log files and properly clean up
defer n.Shutdown() 
```

## Other Tips

It is useful for development to only execute the test you are currently building. To do so, you can modify the `docker-compose.yml` file as follows:

```yaml
entrypoint: go test ./tests -run <YOUR_TEST_NAME> -v -mod=readonly
```