# Configuration parameters
## Customizing configuration
Users can pass configuration parameters two ways when running GoShimmer. One way is through JSON configuration file and another way is through command line arguments.
Settings passed through command line arguments take precedence. The JSON configuration file is structured as an JSON object containing parameters and their values.
Parameters are grouped into embedded objects containing parameters for a single plugin or functionality. There is no limit on how deep the configuration object may be embedded.
For example, below config contains example parameters for the PoW plugin.

```json
{
  "pow": {
    "difficulty": 2,
    "numThreads": 1,
    "timeout": "10s"
  }
}
```

The same arguments can be passed through command line arguments in a following way. Embedded objects' values are described using JSON dot-notation.
Additionally,the user can pass the path of the JSON config file through a command-line argument as well, as shown in an example below. 

```bash
goshimmer \
--config=/tmp/config.json \
--pow.difficulty=2 \
--pow.numThreads=1 \
--pow.timeout=10s 
```

## Custom parameter fields

Currently, in the code there are two ways in which parameters are registered with GoShimmer. One is deprecated way, and the other is the one that should be used when adding new parameters.

### Old, deprecated way
The old way is described shortly to give a basic understanding of how it works, as it should not be used when adding new parameters.

In a package where the parameters will be used, create a `parameters.go` file, that contains definition of constants, which define parameter names in JSON dot-notation. 
The constants will be later used in the code to access the parameter value. 
The file should also contain `init()` function, which registers the parameters with the `flag` library responsible for parsing configuration along with its default value and short description.
It should include comments describing what the parameter is for. An example `parameters.go` file:

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

The parameter values can be accessed in the code in a following way through the `config` plugin:

```go
import "github.com/iotaledger/goshimmer/plugins/config"

config.Node().Int(CfgGossipPort)
```

### New way

Defining configuration parameters using the new way is really similar, however the parameters are not registered directly with the package reading the configuration,
but rather with our custom package that contains all the logic required to make it work seamlessly. 

In this approach, instead of defining a parameter name, a new structure is defined with all necessary parameters, their default values and usage descriptions using Go's struct field tags.

One difference is that parameter names do not contain the namespace they belong to, the namespace is set when registering the parameters structure with the `configuration` package. One `parameters.go` file can contain definitions and register multiple parameter structures.

```go
package customPlugin

import "github.com/iotaledger/hive.go/configuration"

// Parameters contains the configuration parameters used by the custom plugin.
var Parameters = struct {
	// ParamName contains some value used within the plugin
	ParamName float64 `default:"0.31" usage:"ParamName used in some calculation"`

	// ParamGroup contains example embedded configuration definition.
	ParamGroup struct {
		// DetailedParam1 is the example value
		DetailedParam1        string `default:"defaultValue" usage:"DetailedParam1 used in the plugin"`
		// DetailedParam2 is the example value
		DetailedParam2        string `default:"defaultValue" usage:"DetailedParam2 used in the plugin"`
	}
}{}

func init() {
	configuration.BindParameters(&Parameters, "customPlugin")
}
```

In order to access the parameter value, user can simply access the structure's field: `Parameters.ParamName` or `Parameters.ParamGroup.DetailedParam1` 
and it will be populated either with default value or values passed through JSON config or command-line argument. 

This approach makes it more simple to define new parameters as well as makes accessing configuration values more clear. 
