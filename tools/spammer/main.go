package main

import (
	"fmt"

	flag "github.com/spf13/pflag"
	"github.com/spf13/viper"

	"github.com/iotaledger/goshimmer/client"
)

const (
	cfgNodeURI = "nodeURIs"
	cfgMPM     = "mpm"
	cfgEnable  = "enable"
)

func init() {
	flag.StringSlice(cfgNodeURI, []string{"http://127.0.0.1:8080"}, "the URI of the node APIs")
	flag.Int(cfgMPM, 1000, "spam count in messages per minute (MPM)")
	flag.Bool(cfgEnable, false, "enable/disable spammer")
}

func main() {
	// example usage:
	//   go run main.go --nodeURIs=http://127.0.0.1:8080 --mpm=600 --enable=true
	flag.Parse()
	if err := viper.BindPFlags(flag.CommandLine); err != nil {
		panic(err)
	}

	mpm := viper.GetInt(cfgMPM)
	if mpm <= 0 {
		panic("invalid value for `mpm` [>0]")
	}
	enableSpammer := viper.GetBool(cfgEnable)

	apis := []*client.GoShimmerAPI{}
	for _, api := range viper.GetStringSlice(cfgNodeURI) {
		apis = append(apis, client.NewGoShimmerAPI(api))
	}

	for _, api := range apis {
		resp, err := api.ToggleSpammer(enableSpammer, mpm)
		if err != nil {
			fmt.Println(err)
			continue
		}
		fmt.Println(resp)
	}
}
