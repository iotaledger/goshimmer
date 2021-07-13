# Configuration parameters

## Customizing Configuration

You can pass configuration parameters in two ways when running GoShimmer:

1. Through a JSON configuration file
2. Through command line arguments.

Settings passed through command line arguments take precedence. The JSON configuration file is structured as a JSON object containing parameters and their values.

Parameters are grouped into embedded objects, which contain parameters for a single plugin or functionality. There is no limit on how deep you can embed the configuration object.

For example, the configuration below contains example parameters for the PoW plugin.

```json
{
  "pow": {
    "difficulty": 2,
    "numThreads": 1,
    "timeout": "10s"
  }
}
```

The same arguments can be passed through command line arguments in the following way

```bash
goshimmer \
--pow.difficulty=2 \
--pow.numThreads=1 \
--pow.timeout=10s 
```

The embedded objects' values are described using JSON dot-notation. Additionally, you can pass the path of the JSON config file through a command-line argument as well, as shown in an example below. 

```bash
goshimmer \
--config=/tmp/config.json
```

## Custom parameter fields

Currently, there are two ways in which parameters are registered with GoShimmer:

* [Define a new structure](#define-a-new-structure)
* [Create a `parameters.go` File (deprecated)](#create-a-parametersgo-file-deprecated)

### Define a New Structure

In this approach, instead of defining a parameter name, a new type is defined with all necessary parameters, their default values and usage descriptions using Go's struct field tags.
A variable is then initialized with the defined type.

The parameters are not registered directly with the package reading the configuration, but rather with our custom package that contains all the logic required to make it work seamlessly. 

One difference is that parameter names do not contain the namespace they belong to.  The namespace is set when registering the parameters structure with the `configuration` package. One `parameters.go` file can contain definitions and register multiple parameter structures.

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

In order to access the parameter value, you should access the structure's field: `Parameters.ParamName` or `Parameters.ParamGroup.DetailedParam1`. It will be populated either with the default value, or values passed through a JSON configuration or command-line argument. 

This approach makes it simpler to define new parameters, as well as makes accessing configuration values more clear. 

### Create a `parameters.go` File (deprecated)

:::warning
This method for configuration parameters is described shortly to give a basic understanding of how it works, but it should not be used any longer when adding new parameters.
:::

In a package where the parameters will be used, create a `parameters.go` file, that contains the definition of constants.  These constants define parameter names in JSON dot-notation. The constants will be later used in the code to access the parameter value. 

The file should also contain an `init()` function, which registers the parameters with the `flag` library responsible for parsing configuration, along with its default value and short description.

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
