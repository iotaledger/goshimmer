# Event driven model

Describe how event driven works (pub-sub) and link to some external articles, nicely looking images etc. to visualize
the concept.

Event driven model is popular approach often used for example in GUI applications, where a program is waiting for some external event to take place (e.g. mouse click) in order to perform some action.

In case of GoShimmer there is no GUI, however applies this approach to architecture as it's really flexible and is used to handle communication with other nodes and other internal parts. 
In GoShimmer some those events are e.g. arrival of new tangle message, peering request or plugin start. 
When an event occurs, an event handler (a function) is executed and properly handles the 

Each event can accept different set of parameters and is handled by multiple different callbacks.
An event is triggered by event producer after a change of state of the program occurs. 
Callbacks in GoShimmer are functions that accept set of parameters in order to perform some action after event 

## Glossary:

At first let's define some terms used further to avoid misunderstandings:

* Event - represents the type of event (e.g. new message or peering request) as well as set of handlers and trigger functions. Each type of event is separately defined. 
  That means that events are independent of each other - each event has its own set of handlers and is triggered separately.

* Event handler (callback) - is a function that is executed when an event of given type occurs. Event handler can accept multiple arguments (e.g. message ID) so that it can perform appropriate actions.
Every handler must accept the same set of parameters (does it?). Each event has different set of handlers (there can be multiple handlers) that are executed when event is triggered.

* Trigger - is a method that triggers execution of event handlers. All handler parameters are passed to trigger method.


## Creating new event with custom callbacks

In order to create new event and its handler:

1. Create new event stream using `events.NewEvent(handlerCaller)` function:

```go
events: Events{
ConnectionFailed: events.NewEvent(peerAndErrorCaller),
NeighborAdded:    events.NewEvent(neighborCaller),
NeighborRemoved:  events.NewEvent(neighborCaller),
MessageReceived:  events.NewEvent(messageReceived),
},
```

1. Create function that will call event handler.

```go
func pluginCaller(handler interface{}, params ...interface{}) {
handler.(func (*Plugin))(params[0].(*Plugin))
}
```

1. Now the event stream is created and we can append some listeners (callbacks) (describe what is closure and what it
   does). One can attach multiple callbacks. Do all events must receive the same set of params?

```go
    nbr.Events.Close.Attach(events.NewClosure(func () {
// assure that the neighbor is removed and notify
_ = m.DropNeighbor(peer.ID())
m.events.NeighborRemoved.Trigger(nbr)
}))
```

1. In order to trigger event with some parameters we need to run Trigger method on the event stream object with some
   parameters:

```go
messageID MessageID
b.tangle.Events.MessageInvalid.Trigger(messageID)
```