# Plugin system

GoShimmer is a complex application that is used in research environment where requirements often changed and new ideas arise. 
Plugin system allows to quickly and easily add and remove modules that need to be started. However, one thing that might be non-intuitive about the use of plugins is that it's taken to an extreme - everything is run through plugins. 
The only code that is not executed through a plugin system is the code responsible for configuring and starting the plugins.
All new future features added to GoShimmer must be added by creating new plugin. 


## Plugin structure

`Plugin` structure is defined as following.

```go
type Plugin struct {
	Node   *Node
	Name   string
	Status int
	Events pluginEvents
	wg     *sync.WaitGroup
}
```

Below is a brief description of each field: 
* `Node` - contains a pointer to `Node` object which contains references to all the plugins and node-level logger. #TODO: figure out why it is there - not really used anywhere
* `Name` - descriptive name of the plugin.
* `Status` - flag indicating whether plugin is enabled or disabled.
* `Events` - structure containing events used to properly deploy the plugin. Details described below.
* `wg` - a private field containing WaitGroup. #TODO: figure out why it is there - not really used anywhere

## Plugin events

Each plugin defines 3 events: * `Init`, `Configure`, `Run`. 
Those events are triggered during different stages of node startup, but plugin doesn't have to define handlers for all of those events in order to do what it's been designed to.
Execution order and purpose of each event is described below: 

1. `Init` - is triggered almost immediately after a node is started. It's used in plugins that are critical for GoShimmer such as reading config file or initializing global logger. Most plugins don't need to use this event.
2. `Configure` - this event is used to configure the plugin before it is started. It is used to define events related to plugin logic or initialize some objects used by the plugin. 
3. `Run` - this event is triggered as the last one. Even handler function contains main logic of the plugin. 
   For many plugins, the event handler function creates a separate worker that works in the background, so that the handler function for one can finish and allow other plugins be started.  

Each event could potentially have more than one handler, however currently all plugins follow a convention where each event has only one handler.

It is important to note that each event is triggered for all plugins sequentially, so that event `Init` is triggered for all plugins, then `Configure` is triggered for all plugins and finally `Run`. 
Such order is crucial, because some plugins rely on other plugins' initialization or configuration. The order in which plugins are initialized, configured and run is also important and this is described below. 

Handler functions for all plugin events share the same interface, so they could potentially be used interchangeably. Sample handler functions look like this:

```go
func configure(_ *node.Plugin) {
	// configure stuff
}

func run(*node.Plugin) {
    // run plugin	
}
```

The handler functions receive one argument of type `*Plugin`. The code responsible for triggering those events passes a pointer to the plugin itself. 
The object needs to be passed so that the handler function can access plugin fields (e.g. plugin name to configure logger).

## Creating new plugin

A plugin must define either
* init - just like config plugin
* run - when no configuration is needed
* run and configure - when plugin needs to be configured before it can be started (gossip plugin)
* all steps altogether
* configure step cannot be defined without run

Plugins are initialized, configured and run in specific order, just like passed in main function.


Plugin must be created using `node.NewPlugin` method. It is crucial that each plugin is created only once, 

```go
const PluginName = "SamplePlugin"

var (
	// plugin is the plugin instance of the DRNG plugin.
	plugin     *node.Plugin
	pluginOnce sync.Once
	log        *logger.Logger
)

// Plugin gets the plugin instance.
func Plugin() *node.Plugin {
	pluginOnce.Do(func() {
		plugin = node.NewPlugin(PluginName, node.Enabled, configure, run)
	})
	return plugin
}

// Handler functions

func configure(_ *node.Plugin) {
	configureEvents()
}

func run(*node.Plugin) {
	
	
}



```


## Running new plugin

describe how plugins are initiated with node and plugins 


```go
node.Run(
		plugins.Core,
```

```go
node.Plugins(
	remotelog.Plugin(),
```


Describe how plugins can be enabled or disabled by default and how they can be disabled/enabled in config and link to documentation.



## Example: creating custom plugin
Describe plugin variables and what is and can be stored there.



## Background workers



Describe how background workers work and why they are used.

### Shutdown order

Describe shutdown process of background workers.



