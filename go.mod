module github.com/iotaledger/goshimmer

go 1.16

replace github.com/linxGnu/grocksdb => github.com/gohornet/grocksdb v1.6.34-0.20210518222204-d6ea5eedcfb9

require (
	github.com/DataDog/zstd v1.4.8 // indirect
	github.com/ReneKroon/ttlcache/v2 v2.11.0
	github.com/StackExchange/wmi v0.0.0-20190523213315-cbe66965904d // indirect
	github.com/beevik/ntp v0.3.0
	github.com/capossele/asset-registry v0.0.0-20210521112927-c9d6e74574e8
	github.com/cockroachdb/errors v1.8.4
	github.com/drand/drand v1.1.1
	github.com/drand/kyber v1.1.2
	github.com/gin-gonic/gin v1.7.0
	github.com/go-ole/go-ole v1.2.4 // indirect
	github.com/go-resty/resty/v2 v2.6.0
	github.com/gorilla/websocket v1.4.2
	github.com/iotaledger/hive.go v0.0.0-20220210121915-5c76c0ccc668
	github.com/labstack/echo v3.3.10+incompatible
	github.com/labstack/gommon v0.3.0
	github.com/libp2p/go-libp2p v0.15.0
	github.com/libp2p/go-libp2p-core v0.9.0
	github.com/libp2p/go-yamux/v2 v2.2.0
	github.com/magiconair/properties v1.8.1
	github.com/markbates/pkger v0.17.1
	github.com/mr-tron/base58 v1.2.0
	github.com/multiformats/go-multiaddr v0.4.1
	github.com/multiformats/go-varint v0.0.6
	github.com/panjf2000/ants/v2 v2.4.7
	github.com/paulbellamy/ratecounter v0.2.0
	github.com/prometheus/client_golang v1.11.0
	github.com/shirou/gopsutil v2.20.5+incompatible
	github.com/spf13/afero v1.3.0 // indirect
	github.com/spf13/pflag v1.0.5
	github.com/spf13/viper v1.7.0
	github.com/stretchr/testify v1.7.0
	go.dedis.ch/kyber/v3 v3.0.13
	go.uber.org/atomic v1.9.0
	go.uber.org/dig v1.13.0
	golang.org/x/crypto v0.0.0-20211202192323-5770296d904e
	golang.org/x/exp v0.0.0-20210220032938-85be41e4509f // indirect
	golang.org/x/xerrors v0.0.0-20200804184101-5ec99f83aff1
	google.golang.org/genproto v0.0.0-20201203001206-6486ece9c497 // indirect
	google.golang.org/protobuf v1.27.1
	gopkg.in/src-d/go-git.v4 v4.13.1
)
