package main

import (
	"fmt"

	flag "github.com/spf13/pflag"
	"github.com/spf13/viper"

	"github.com/iotaledger/goshimmer/client"
)

const (
	cfgNodeURI = "nodeURIs"
	cfgMPS     = "mps"
	cfgEnable  = "enable"
	cfgImif    = "imif"
)

func init() {
	flag.StringSlice(cfgNodeURI, []string{"http://127.0.0.1:8080"}, "the URI of the node APIs")
	flag.Int(cfgMPS, 1000, "spam count in messages per second (MPS)")
	flag.Bool(cfgEnable, false, "enable/disable spammer")
	flag.String(cfgImif, "uniform", "inter message issuing function: uniform or poisson")
}

func main() {
	// example usage:
	//   go run main.go --nodeURIs=http://127.0.0.1:8080 --mps=600 --enable=true --imif=uniform
	flag.Parse()
	if err := viper.BindPFlags(flag.CommandLine); err != nil {
		panic(err)
	}

	mps := viper.GetInt(cfgMPS)
	if mps <= 0 {
		panic("invalid value for `mps` [>0]")
	}
	enableSpammer := viper.GetBool(cfgEnable)
	imif := viper.GetString(cfgImif)

	var apis []*client.GoShimmerAPI
	for _, api := range viper.GetStringSlice(cfgNodeURI) {
		apis = append(apis, client.NewGoShimmerAPI(api))
	}

	for _, api := range apis {
		resp, err := api.ToggleSpammer(enableSpammer, mps, imif)
		if err != nil {
			fmt.Println(err)
			continue
		}
		fmt.Println(resp)
	}
}
