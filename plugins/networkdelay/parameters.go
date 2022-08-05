package networkdelay

import "github.com/iotaledger/goshimmer/plugins/config"

// ParametersDefinition contains the definition of the parameters used by the networkdelay plugin.
type ParametersDefinition struct {
	// OriginPublicKey defines the default issuer node public key in base58 encoding.
	OriginPublicKey string `default:"9DB3j9cWYSuEEtkvanrzqkzCQMdH1FGv3TawJdVbDxkd" usage:"default issuer node public key"`
}

// Parameters contains the configuration used by the networkdelay plugin.
var Parameters = &ParametersDefinition{}

func init() {
	config.BindParameters(Parameters, "networkdelay")
}
