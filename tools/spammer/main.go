package main

import (
	"fmt"

	"github.com/iotaledger/goshimmer/client"
	flag "github.com/spf13/pflag"
	"github.com/spf13/viper"
)

const (
	cfgNodeURI = "node"
	cfgMessage = "message"
)

func init() {
	flag.String(cfgNodeURI, "http://127.0.0.1:8080", "the URI of the node API")
	flag.String(cfgMessage, "", "the URI of the node API")
}

func main() {
	flag.Parse()
	if err := viper.BindPFlags(flag.CommandLine); err != nil {
		panic(err)
	}
	goshimAPI := client.NewGoShimmerAPI(viper.GetString(cfgNodeURI))
	messageBytes := []byte(viper.GetString(cfgMessage))
	var issued, failed int
	for {
		fmt.Printf("issued %d, failed %d\r", issued, failed)
		_, err := goshimAPI.Data(messageBytes)
		if err != nil {
			failed++
			continue
		}
		issued++
	}
}
