---
description: When an event is triggered, an event handler (or a collection of handlers) is executed and the state of the application is updated as necessary. In GoShimmer some of those events can be the arrival of new tangle message, peering request or plugin start.
image: /img/logo/goshimmer_light.png
keywords:
- events
- plugin
- handler
- trigger
- specific type
---
# Event Driven Model

Event driven model is popular approach often used for example in GUI applications, where a program is waiting for some external event to take place (e.g. mouse click) in order to perform some action.
In case of GoShimmer there is no GUI, however it applies this architecture approach as it's really flexible and is used to handle communication with other nodes and other internal parts. 
In GoShimmer some of those events can be e.g. arrival of new tangle message, peering request or plugin start. 
When an event is triggered, an event handler (or a collection of handlers) is executed and the state of the application is updated as necessary.
 
## Glossary

At first let's define some terms used further to avoid misunderstandings:

### Event
Represents the type of event (e.g. new message or peering request) as well as set of handlers and trigger functions. Each type of event is separately defined 
  which means that events are independent of each other - each event has its own set of handlers and is triggered separately.

### Event handler (callback) 
A function that is executed when an event of given type occurs. An event handler can accept multiple arguments (e.g. message ID or plugin) so that it can perform appropriate actions.
  Every handler must accept the same set of parameters. Each event has a different set of handlers (there can be multiple handlers) that are executed when the event is triggered.

### Trigger
A method that triggers execution of event handlers with given parameter values.


## Creating a New Event With Custom Callbacks

Below are the steps that show the example code necessary to create a custom event, attach a handler and trigger the event. 

1. Create a function that will call event handlers (handler caller) for a specific event. 
   Each event has only one handler caller. It enforces that all handlers for the event must share the same interface, because the caller will pass a fixed set of arguments of specific types to handler function. 
   It's not possible to pass different number of arguments or types to the handler function. 
   Callers for all events must also share the same interface - the first argument represents the handler function that will be called represented by a generic argument.
   Further arguments represent parameters that will be passed to the handler during execution. Below are example callers that accept one and two parameters respectively. 
   More arguments can be passed in similar manner. 
   
```go
func singleArgCaller(handler interface{}, params ...interface{}) {
    handler.(func (*Plugin))(params[0].(*Plugin))
}

func twoArgsCaller(handler interface{}, params ...interface{}) {
    handler.(func(*peer.Peer, error))(params[0].(*peer.Peer), params[1].(error))
}
```

`handler.(func (*Plugin))(params[0].(*Plugin))` - this code seems a little complicated, so to make things simpler we will divide into smaller parts and explain each:

* `handler.(func (*Plugin))` (A) - this part does type-cast the handler from generic type onto type of desired, specific function type - in this case it's a function that accepts `*Plugin` as its only parameter.
* `params[0].(*Plugin)` (B)- similarly to previous part, first element of parameter slice is type-casted onto `*Plugin` type, so that it matches the handler function interface.
* `handler.(func (*Plugin))(params[0].(*Plugin))` - the whole expression calls the type-casted handler function with the type-casted parameter value. We can also write this as `A(B)` to make things simpler.

The above explanation also allows a better understanding of why all handlers must share the same interface - handler caller passes fixed number of parameters and does type-casting of arguments onto specific types.


2. Next, a new event object needs to be created. We pass the handler caller as an argument, which is saved inside the object to be called when the event is triggered.

```go
import "github.com/iotaledger/hive.go/events"

ThisEvent := events.NewEvent(singleArgCaller)
```

3. After creating the event, handlers (or callbacks) can be attached to it. An event can have multiple callbacks, however they all need to share the same interface. 
   One thing to note, is that functions are not passed directly - first they are wrapped into a `events.Closure` object like in the example below. 

```go
ThisEvent.Attach(events.NewClosure(func (arg *Plugin) {
    // do something
}))
```

4. In order to trigger the event with some parameters we need to run the `.Trigger` method on the event object with parameters that handler functions will receive:

```go
somePlugin Plugin
ThisEvent.Trigger(&somePlugin)
```