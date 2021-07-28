# Plugin system

GoShimmer is a complex application that is used in a research environment where requirements often changed and new ideas arise. The Plugin system allows to quickly and easily add and remove modules that need to be started. 

However, one thing that might be non-intuitive about the use of plugins is that it is taken to an extreme - everything runs through plugins. The only code that is not executed through a plugin system is the code responsible for configuring and starting the plugins.

All new future features added to the GoShimmer must be added by creating a new plugin. 


## Plugin structure

A `Plugin` structure is defined as follows:

```go
type Plugin struct {
	Node   *Node
	Name   string
	Status int
	Events pluginEvents
	wg     *sync.WaitGroup
}
```
 
* `Node`: Contains a pointer to a `Node` object, which contains references to all the plugins and a node-level logger. #TODO: figure out why it is there - not really used anywhere
* `Name`: Descriptive name of the plugin.
* `Status`: Flag indicating whether plugin is enabled or disabled.
* `Events`: Structure containing events used to properly deploy the plugin. Details described in the [plugin events section](#plugin-events).
* `wg`: A private field containing WaitGroup. #TODO: figure out why it is there - not really used anywhere

## Plugin events

Each plugin defines 3 events: `Init`, `Configure`, `Run`. 
These events are triggered during different stages of node startup.  However, the plugin does not have to define handlers for all of those events in order to do what it's been designed for.

1. `Init`: Is triggered almost immediately after you start a node. It is used in plugins that are critical for GoShimmer, such as reading config files or initializing a global logger. Most plugins don't need to use this event.
   
2. `Configure`: You can use this event to configure the plugin before it is started. You can use it to define events related to internal plugin logic or initialize objects used by the plugin.
   
3. `Run`: This should be the last event that is triggered. The event handler function contains the main logic of the plugin.
   
   For many plugins, the event handler function creates a separate worker that works in the background, so that the handler function for one plugin can finish, and allow other plugins to be started.  

   Each event could potentially have more than one handler. However, currently, all existing plugins follow a convention where each event has only one handler.

It is important to note that each event is triggered for all plugins sequentially. The event `Init` is triggered for all plugins, then `Configure` is triggered for all plugins, and finally `Run`.

Because some plugins rely on other plugins' initialization or configuration, this order is crucial. The order in which plugins are initialized, configured and run is also important and will be addressed in the following sections. 

Handler functions for all plugin events share the same interface, so they could potentially be used interchangeably. Handler functions look like this:

```go
func configure(_ *node.Plugin) {
    // configure stuff
}

func run(*node.Plugin) { 
    // run plugin	
}
```

The handler functions receive one argument of type `*Plugin`. The code responsible for triggering those events passes a pointer to the plugin object itself. The object needs to be passed so that the handler function can access plugin fields, for  example a plugin name to configure logger.

## Creating new plugin

A plugin object can be created by calling the `node.NewPlugin` method. 
The method creates and returns a new plugin object. It will also register it so that GoShimmer knows the plugin is available.

It accepts the following arguments:

* `name string`: The plugin name.
* `status int`: Flag indicating whether the plugin is enabled or disabled by default. This can be overridden by enabling/disabling the plugin in the external configuration file. 
  
   Possible values: `node.Enabled`, `node.Disabled`. 
* `callbacks ...Callback`: List of event handler functions. The method will correctly create a plugin when passing up to 2 callbacks. 
  
  :::info
  `type Callback = func(plugin *Plugin)`, which is a raw function type without being wrapped in `events.Closure`.
  :::

There is a couple of ways that the method can be called, depending on which plugin events you need to configure. 

* Define the `Configure` and `Run` event handlers. This the most common usage for plugins currently.

```go
plugin = node.NewPlugin(PluginName, node.Enabled, configure, run)
```

* Define only the `Configure` event. This method is used for plugins that are used to configure objects managed by other plugins, such as creating API endpoints. 

```go
plugin = node.NewPlugin(PluginName, node.Enabled, configure)
```

* Define a plugin without `Configure` or `Run` event handlers. This method is used to create plugins that perform some action when the `Init` event is triggered.

```go
plugin = node.NewPlugin(PluginName, node.Enabled)
```

However, the `Init` event handler cannot be attached using the `node.NewPlugin` method. In order to specify this handler, the plugin creator needs to attach it manually to the event.  For example, inside the package's `init()` method in the file containing the rest of the plugin definition.

```go
func init() {
	plugin.Events.Init.Attach(events.NewClosure(func(*node.Plugin) {
		// do something
	}))
}
```

It is important to note, that the `node.NewPlugin` method accepts handler functions in a raw format, that is, without being wrapped by the `events.Closure` object, as the method does the wrapping inside.

However, when attaching the `Init` event handler manually, it must be wrapped by the `events.Closure` object. 

It is crucial that each plugin is created only once, and that `sync.Once` class is used to guarantee that. The following is the example of a file containing sample plugin definition. All plugins follow this format.  

```go
const PluginName = "SamplePlugin"

var (
	// plugin is the plugin instance of the new plugin plugin.
	plugin     *node.Plugin
	pluginOnce sync.Once
)

// Plugin gets the plugin instance.
func Plugin() *node.Plugin {
	pluginOnce.Do(func() {
		plugin = node.NewPlugin(PluginName, node.Enabled, configure, run)
	})
	return plugin
}

// Handler functions
func init() {
    plugin.Events.Init.Attach(events.NewClosure(func(*node.Plugin) {
        // do something
    }))
}

func configure(_ *node.Plugin) {
    // configure stuff
}

func run(*node.Plugin) {
    // run stuff	
}
```

## Running new plugin

In order to correctly add a new plugin to GoShimmer, it should be defined, and it must also be passed to the `node.Run` method.

Because there are plenty of plugins, in order to improve readability and make managing plugins easier, they are grouped into separate wrappers passed to the `node.Run` method.

When adding a new plugin, it must be added into one of those groups, or a new group must be created.

```go
node.Run(
    plugins.Core,
    plugins.Research,
    plugins.UI,
    plugins.WebAPI,
)
```

You can add a plugin by calling the `Plugin()` method of the newly created plugin and passing the argument further. You can find an example group definition below. When you add the group, the plugin is then correctly added, and will be run when GoShimmer starts.

```go
var Core = node.Plugins(
    banner.Plugin(),
    newPlugin.Plugin(),
    // other plugins ommited 
)
```

## Background workers

In order to run plugins beyond the scope of the short-lived `Run` event handler, multiple `daemon.BackgroundWorker` instances can be started inside the handler function.  This allows the `Run` event handler to finish quickly, and the plugin logic can continue running concurrently in a separate goroutine. 

You can start a background worker by running the `daemon.BackgroundWorker` method, which accepts following arguments:

* `name string`: The background worker name.
  
* `handler WorkerFunc`: Long-running function that will be started in its own goroutine. It accepts a single argument of type `<-chan struct{}`. When something is sent to that channel, the worker will shut down. Note: `type WorkerFunc = func(shutdownSignal <-chan struct{})`
  
* `order ...int`: Value used to define in which shutdown order this particular background worker must be shut down (higher = earlier).

   The parameter can accept either one or zero values, more values will be ignored. When passing zero values, the default value of `0` is assumed.
  Instead of passing integers manually,  you should use the `github.com/iotaledger/goshimmer/packages/shutdown` package which will normalize the values. 
   Because different plugins depend on others working correctly, correct shutdown order is as important as correct start order. So when one plugin shuts down too soon, another plugin may run into errors, crash and leave an incorrect state. 
  
  
You can use the following example code to create a background worker: 

```go
func start(shutdownSignal <-chan struct{}) {
	// long-running function
	// possibly start goroutines here
	// wait for shutdown signal
    <-shutdownSignal
}

if err := daemon.BackgroundWorker(backgroundWorkerName, start, shutdown.PriorityGossip); err != nil {
	log.Panicf("Failed to start as daemon: %s", err)
}
```
