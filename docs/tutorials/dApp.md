
# How to create a simple dApp

> This guide is meant for developers familiar with the Go programming language.

> **DISCLAIMER:** GoShimmer is a rapidly evolving prototype software. As such, the described steps here will likely change in the future. Specifically, we are envisioning to ease the process of dApp creation and installation for node owners. Furthermore, the current approach is in no way hardened and should be seen as purely experimental. Do not write any software for actual production use.

## Network Delay dApp
In this guide we are going to explain how to write a very simple dApp based on an actual dApp we are using in GoShimmer to help us measure the network delay, i.e., how long it takes for every active node in the network to receive a message. Gathering this data will enable us to set realistic parameters for FCoB.

The complete source code of the application can be found [in the repository](https://github.com/iotaledger/goshimmer/tree/develop/plugins/networkdelay). 

### Overview
Our network delay dApp should help us to identify the time it takes for every active node to receive and process a message. That can be done in a few simple steps:
1. A (known) node sends a special message containing a network delay object.
2. Upon receipt, every other node in the network answers to the special message by posting its current time to our remote logger.
3. For simplicity, we gather the information in an [ELK stack](https://www.elastic.co/what-is/elk-stack). This helps us to easily interpret and analyze the data.

Within GoShimmer we need 3 components to realize this undertaking. First, we need to **define and register a network delay object type**. Second, we need a way to **initiate a message** with a network delay object via the web API. And lastly, we need to **listen** for network delay objects and take appropriate action.

If a node does not have our dApp installed and activated, the message will be simply treated as a raw data message without any particular meaning. In general that means that in order for a dApp to be useful, node owners need to explicitly install it. In our case we simply ship it with GoShimmer as a [plugin](../implementation_design/plugin.md).

### Define & Register The Network Delay Object
First, we need to decide what data our network delay object should contain and define the byte layout accordingly.
In our case we need an `ID` to identify a network delay message and the `sent time` of the initiator. 
Therefore, we can define the byte layout as follows:
```
length<uint32-4bytes> // every object has to have this
type<uint32-4bytes> // every object has to have this
id<32bytes>
sentTime<int64-8bytes>
```

Next, we need to fulfill the `Payload` interface and provide the functionality to read/write an object from/to bytes. The [`hive.go/marshalutil`](https://github.com/iotaledger/hive.go/tree/master/marshalutil) package simplifies this step tremendously.
```Go
// Payload represents the generic interface for an object that can be embedded in Messages of the Tangle.
type Payload interface {
    // Type returns the Type of the Payload.
    Type() Type
    
    // Bytes returns a marshaled version of the Payload.
    Bytes() []byte
    
    // String returns a human readable version of the Payload.
    String() string
}
```

Finally, we need to create and register our network delay object type so that it can be properly unmarshalled. 
```Go
// Type represents the identifier which addresses the network delay Object type.
var Type = payload.NewType(189, ObjectName, func(data []byte) (payload payload.Payload, err error) {
    var consumedBytes int
    payload, consumedBytes, err = FromBytes(data)
    if err != nil {
        return nil, err
    }
    if consumedBytes != len(data) {
        return nil, errors.New("not all payload bytes were consumed")
    }
    return
})
```

### Create The Web API Endpoints
In order to issue a message with our newly created network delay object, we need to create a web API endpoint. Here we simply create a random `ID` and the `sentTime` and then issue a message with `issuer.IssuePayload()`. This plugin takes care of all the specifics and employs the `MessageFactory` to, i.a., select tips and sign the message.

```Go
webapi.Server.POST("networkdelay", broadcastNetworkDelayObject)

func broadcastNetworkDelayObject(c echo.Context) error {
	// generate random id
	rand.Seed(time.Now().UnixNano())
	var id [32]byte
	if _, err := rand.Read(id[:]); err != nil {
		return c.JSON(http.StatusInternalServerError, Response{Error: err.Error()})
	}

	msg, err := issuer.IssuePayload(NewObject(id, time.Now().UnixNano()))
	if err != nil {
		return c.JSON(http.StatusBadRequest, Response{Error: err.Error()})
	}
	return c.JSON(http.StatusOK, Response{ID: msg.Id().String()})
}
```


### Listen for network delay objects
Every dApp listens for messages from the *communication layer* and when its data type is detected, takes appropriate action. For us that means listening for network delay objects and sending messages to our remote logger if we encounter any. Of course in this context, we only want to react to network delay objects which were issued by our analysis/entry node server. Therefore, matching the message signer's public key with a configured public key lets us only react to the appropriate network delay objects.

```Go
func onReceiveMessageFromMessageLayer(messageID tangle.MessageID) {
    messagelayer.Tangle().Storage.Message(messageID).Consume(func(solidMessage *tangle.Message) {
        messagePayload := solidMessage.Payload()
        if messagePayload.Type() != Type {
            return
        }
    
        // check for node identity
        issuerPubKey := solidMessage.IssuerPublicKey()
        if issuerPubKey != originPublicKey || issuerPubKey == myPublicKey {
            return
        }
        
        networkDelayObject, ok := messagePayload.(*Object)
        if !ok {
            app.LogInfo("could not cast payload to network delay object")
            return
        }
        
        now := clock.SyncedTime().UnixNano()
        
        // abort if message was sent more than 1min ago
        // this should only happen due to a node resyncing
        if time.Duration(now-networkDelayObject.sentTime) > time.Minute {
            app.LogDebugf("Received network delay message with >1min delay\n%s", networkDelayObject)
        return
        }
    
        sendToRemoteLog(networkDelayObject, now)
    })
}
```
