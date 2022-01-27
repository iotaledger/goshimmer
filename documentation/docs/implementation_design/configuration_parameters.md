---
description: You can pass configuration parameters in two ways when running GoShimmer, through a JSON configuration file or through command line arguments. Parameters are grouped into embedded objects containing parameters for a single plugin or functionality. There is no limit on how deep the configuration object may be embedded.
image: /img/logo/goshimmer_light.png
keywords:
- json 
- command line
- embedded object
- parameters
---

# Configuration Parameters

## Customizing Configuration

Users can pass configuration parameters in two ways when running GoShimmer. One way is through a JSON configuration file and another way is through command line arguments.
Settings passed through command line arguments take precedence. The JSON configuration file is structured as a JSON object containing parameters and their values.
Parameters are grouped into embedded objects containing parameters for a single plugin or functionality. There is no limit on how deep the configuration object may be embedded.
For example, the config below contains example parameters for the PoW plugin.

```json
{
  "pow": {
    "difficulty": 2,
    "numThreads": 1,
    "timeout": "10s"
  }
}
```

The same arguments can be passed through command line arguments in the following way. Embedded objects' values are described using JSON dot-notation.
Additionally,the user can pass the path of the JSON config file through a command-line argument as well, as shown in an example below. 

```shell
goshimmer \
--config=/tmp/config.json \
--pow.difficulty=2 \
--pow.numThreads=1 \
--pow.timeout=10s 
```

## Custom Parameter Fields

Currently, in the code there are two ways in which parameters are registered with GoShimmer. However, one is deprecated way, while the second should be used any longer when adding new parameters.

### New Way

Defining configuration parameters using the new way is really similar, however the parameters are not registered directly with the package reading the configuration,
but rather with our custom package that contains all the logic required to make it work seamlessly. 

In this approach, instead of defining a parameter name, a new type is defined with all necessary parameters, their default values and usage descriptions using Go's struct field tags.
A variable is then initialized with the defined type.

One difference is that parameter names do not contain the namespace they belong to, the namespace is set when registering the parameters structure with the `configuration` package. One `parameters.go` file can contain definitions and register multiple parameter structures.

```go
package customPlugin

import "github.com/iotaledger/hive.go/configuration"

// Parameters contains the configuration parameters used by the custom plugin.
type ParametersDefinition struct {
	// ParamName contains some value used within the plugin
	ParamName float64 `default:"0.31" usage:"ParamName used in some calculation"`

	// ParamGroup contains an example of embedded configuration definitions.
	ParamGroup struct {
		// DetailedParam1 is the example value
		DetailedParam1        string `default:"defaultValue" usage:"DetailedParam1 used in the plugin"`
		// DetailedParam2 is the example value
		DetailedParam2        string `default:"defaultValue" usage:"DetailedParam2 used in the plugin"`
	}
}

var Parameters = &ParametersDefinition{}

func init() {
	configuration.BindParameters(Parameters, "customPlugin")
}
```

In order to access the parameter value, a user can simply access the structure's field: `Parameters.ParamName` or `Parameters.ParamGroup.DetailedParam1` 
and it will be populated either with the default value or values passed through a JSON config or command-line argument. 

This approach makes it more simple to define new parameters as well as makes accessing configuration values more clear. 

### Old, Deprecated Way

The old way is described shortly to give a basic understanding of how it works, but it should not be used any longer when adding new parameters.

In a package where the parameters will be used, create a `parameters.go` file, that contains the definition of constants, which define parameter names in JSON dot-notation.
The constants will be later used in the code to access the parameter value.
The file should also contain an `init()` function, which registers the parameters with the `flag` library responsible for parsing configuration along with its default value and short description.
It should include comments describing what the parameter is for. Here is an example `parameters.go` file:

```go
package customPackage

import (
	flag "github.com/spf13/pflag"
)
const (
	// ParamName contains some value used within the plugin
	ParamName = "customPlugin.paramName"
)

func init() {
	flag.Float64(paramName, 0.31, "ParamName used in some calculation")
}
```

The parameter values can be accessed in the code in the following way through the `config` plugin:

```go
import "github.com/iotaledger/goshimmer/plugins/config"

config.Node().Int(CfgGossipPort)
```
