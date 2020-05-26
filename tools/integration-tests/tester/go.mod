module github.com/iotaledger/goshimmer/tools/integration-tests/tester

go 1.14

require (
	github.com/Microsoft/go-winio v0.4.14 // indirect
	github.com/docker/distribution v2.7.1+incompatible // indirect
	github.com/docker/docker v1.13.1
	github.com/docker/go-connections v0.4.0
	github.com/docker/go-units v0.4.0 // indirect
	github.com/drand/drand v0.8.1
	github.com/iotaledger/goshimmer v0.1.3
	github.com/iotaledger/hive.go v0.0.0-20200525142347-543f24c486b8
	github.com/opencontainers/go-digest v1.0.0-rc1 // indirect
	github.com/stretchr/testify v1.5.1
)

replace github.com/iotaledger/goshimmer => ../../..
