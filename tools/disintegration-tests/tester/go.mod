module github.com/iotaledger/goshimmer/tools/disintegration-tests/tester

go 1.16

replace github.com/iotaledger/goshimmer => ../../..

replace github.com/iotaledger/goshimmer/tools/integration-tests/tester => ../../integration-tests/tester

require (
	github.com/Microsoft/go-winio v0.4.17 // indirect
	github.com/docker/docker v1.13.1
	github.com/iotaledger/goshimmer/tools/integration-tests/tester v0.0.0-20210618092120-fbd4de6e886f
	github.com/klauspost/compress v1.11.13 // indirect
	github.com/matttproud/golang_protobuf_extensions v1.0.2-0.20181231171920-c182affec369 // indirect
	github.com/pelletier/go-toml v1.8.1 // indirect
	github.com/prometheus/client_golang v1.7.1 // indirect
	github.com/prometheus/procfs v0.6.0 // indirect
	github.com/stretchr/testify v1.7.0
	golang.org/x/crypto v0.0.0-20210322153248-0c34fe9e7dc2 // indirect
	golang.org/x/sys v0.0.0-20210616094352-59db8d763f22 // indirect
	honnef.co/go/tools v0.0.1-2020.1.3 // indirect
)
