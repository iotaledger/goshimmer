# Integration tests with Docker

![Integration testing](../../images/integration-testing.png)

Running the integration tests spins up a `tester` container within which every test can specify its own GoShimmer network with Docker as schematically shown in the figure above.

Peers can communicate freely within their Docker network and this is exactly how the tests are run using the `tester` container.
Test can be written in regular Go style while the framework provides convenience functions to create a new network, access a specific peer's web API or logs.

## How to run
Prerequisites: 
- Docker 17.12.0+
- Docker compose: file format 3.5

```
# Mac & Linux
./runTests.sh
```
The tests produce `*.log` files for every networks' peer in the `logs` folder after every run.

On GitHub logs of every peer are stored as artifacts and can be downloaded for closer inspection once the job finishes.

## Creating tests
Tests can be written in regular Go style. Each tested component should reside in its own test file in `tester/tests`.
`main_test` with its `TestMain` function is executed before any test in the package and initializes the integration test framework.

Each test has to specify its network where the tests are run. This can be done via the framework at the beginning of a test.
```go
// create a network with name 'testnetwork' with 6 peers and wait until every peer has at least 3 neighbors
n := f.CreateNetwork("testnetwork", 6, 3)
// must be called to create log files and properly clean up
defer n.Shutdown() 
```

## Other tips
Useful for development is to only execute the test you're currently building. For that matter, simply modify the `docker-compose.yml` file as follows:
```yaml
entrypoint: go test ./tests -run <YOUR_TEST_NAME> -v -mod=readonly
```