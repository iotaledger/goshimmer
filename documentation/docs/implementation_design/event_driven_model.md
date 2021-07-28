# Event driven model

The event driven model is popular approach often used in GUI applications.  For example a program is waiting for some external event to take place, like a mouse click, in order to perform some action.

In the case of GoShimmer there is no GUI. However, it applies this architecture approach as it is really flexible, and it is used to handle communication with other nodes and other internal parts.

In GoShimmer some of those events can be the arrival of new tangle message, peering request or plugin start.

When an event is triggered, an event handler (or a collection of handlers) will be executed,  and the state of the application will be updated as necessary.
 
## Glossary

Please read the definition of some used terms to avoid misunderstandings:

- Event: Represents the type of event, for example new message or peering request, as well as set of handlers and trigger functions. Each event type defined separately, which means that events are independent of each other.  Therefore, each event has its own set of handlers and is triggered separately.

- Event handler (callback):  The event handler is a function that is executed when an event of given type occurs. An event handler can accept multiple arguments, for example the message ID or plugin, so that it can perform appropriate actions.
  
  Every handler must accept the same set of parameters. Each event has a different set of handlers (there can be multiple handlers) that are executed when the event is triggered.

* Trigger: A method that triggers execution of event handlers with given parameter values.


## Creating a New Event With Custom Callbacks

You can follow these steps to create a custom event, attach a handler and trigger the event. 

1. Create a function that will call event handlers (handler caller) for a specific event. 
   
    Each event has only one handler caller. It enforces that all handlers for the event must share the same interface, because the caller will pass a fixed set of arguments of specific types to handler function. 
   
   It's not possible to pass different number of arguments or types to the handler function.
   
   Callers for all events must also share the same interface - the first argument represents the handler function that will be called, represented by a generic argument.
   
   Further arguments represent parameters that will be passed to the handler during execution. Below are example callers that accept one and two parameters respectively. More arguments can be passed in similar manner. 
   
    ```go
    func singleArgCaller(handler interface{}, params ...interface{}) {
        handler.(func (*Plugin))(params[0].(*Plugin))
    }
    
    func twoArgsCaller(handler interface{}, params ...interface{}) {
        handler.(func(*peer.Peer, error))(params[0].(*peer.Peer), params[1].(error))
    }
    ```

    `handler.(func (*Plugin))(params[0].(*Plugin))` - this code seems a little complicated, so to make things simpler we will divide it into smaller parts:
    
    * `handler.(func (*Plugin))` (A): This part does type-cast the handler from generic type onto the desired type, specific function type.  In this case it is a function that accepts `*Plugin` as its only parameter.
      
    * `params[0].(*Plugin)` (B): Similarly to previous part, the first element of parameter slice is type-casted onto a `*Plugin` type, so that it matches the handler function interface.
      
    * `handler.(func (*Plugin))(params[0].(*Plugin))`: The whole expression calls the type-casted handler function with the type-casted parameter value. You can also write this as `A(B)` to make things simpler.
    
    The above explanation also allows a better understanding of why all handlers must share the same interface - handler caller passes fixed number of parameters and does type-casting of arguments onto specific types.


2. Next,  you need to create a new event object. You should pass the handler caller as an argument, which is saved inside the object to be called when the event is triggered.

    ```go
    import "github.com/iotaledger/hive.go/events"
    
    ThisEvent := events.NewEvent(singleArgCaller)
    ```

3. Once you have created the event, you can attach handlers (or callbacks) to it.  An event can have multiple callbacks, however they all need to share the same interface.
   
   One thing to note, is that you can not pass functions directly - you should wrap them into a `events.Closure` object first, like in the example below. 

    ```go
    ThisEvent.Attach(events.NewClosure(func (arg *Plugin) {
        // do something
    }))
    ```

4. In order to trigger the event with some parameters you need to run the `.Trigger` method on the event object with parameters that handler functions will receive:

    ```go
    somePlugin Plugin
    ThisEvent.Trigger(&somePlugin)
    ```