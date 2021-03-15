> This guide is meant for developers familiar with the Go programming language.

> **DISCLAIMER:** GoShimmer is a rapidly evolving prototype software. As such, the described steps here will likely change in the future. Specifically, we are envisioning to ease the process of dApp creation and installation for node owners. Furthermore, the current approach is in no way hardened and should be seen as purely experimental. Do not write any software for actual production use.

# Network Delay dApp
In this guide we are going to explain how to write a very simple dApp based on an actual dApp we are using in GoShimmer to help us measure the network delay, i.e., how long it takes for every active node in the network to receive a message. Gathering this data will enable us to set realistic parameters for FCoB.

The complete source code of the application can be found [in the repository](https://github.com/iotaledger/goshimmer/tree/develop/dapps/networkdelay). 

## Overview
Our network delay dApp should help us to identify the time it takes for every active node to receive and process a message. That can be done in a few simple steps:
1. A (known) node sends a special message containing a network delay object.
2. Upon receipt, every other node in the network answers to the special message by posting its current time to our remote logger.
3. For simplicity we gather the information in an [ELK stack](https://www.elastic.co/what-is/elk-stack). This helps us to easily interpret and analyze the data.

Within GoShimmer we need 3 components to realize this undertaking. First, we need to **define and register a network delay object type**. Second, we need a way to **initiate a message** with a network delay object via the web API. And lastly, we need to **listen** for network delay objects and take appropriate action.

If a node does not have our dApp installed and activated, the message will be simply treated as a raw data message without any particular meaning. In general that means that in order for a dApp to be useful, node owners need to explicitly install it. In our case we simply ship it with GoShimmer.

## Define & Register The Network Delay Object
First, we need to decide what data our network delay object should contain and define the byte layout accordingly.
In our case we need an `ID` to identify a network delay message and the `sent time` of the initiator. 
Therefore, we can define the byte layout as follows:
```
type<uint32-4bytes> // every object has to have this
length<uint32-4bytes> // every object has to have this
id<32bytes>
sentTime<int64-8bytes>
```

Next, we need to fulfill the `Payload` interface and provide the functionality to read/write an object from/to bytes. The [`hive.go/marshalutil`](https://github.com/iotaledger/hive.go/tree/master/marshalutil) package simplifies this step tremendously.
```Go
type Payload interface {
	// Type returns the type of the payload.
	Type() Type
	// Bytes returns the payload bytes.
	Bytes() []byte
	// Unmarshal unmarshals the payload from the given bytes.
	Unmarshal(bytes []byte) error
	// String returns a human-friendly representation of the payload.
	String() string
}
```

Finally, we need to register our network delay object type so that it can be properly unmarshalled. 
```Go
func init() {
	payload.RegisterType(Type, func(data []byte) (payload payload.Payload, err error) {
		payload = &Object{}
		err = payload.Unmarshal(data)

		return
	})
}
```

## Create The Web API Endpoints
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


## Listen for network delay objects
Every dApp listens for messages from the *communication layer* and when its data type is detected, takes appropriate action. For us that means listening for network delay objects and sending messages to our remote logger if we encounter any. Of course in this context, we only want to react to network delay objects which were issued by our analysis/entry node server. Therefore, matching the message signer's public key with a configured public key lets us only react to the appropriate network delay objects.

```Go
// subscribe to message-layer
messagelayer.Tangle.Events.MessageSolid.Attach(events.NewClosure(onReceiveMessageFromMessageLayer))

func onReceiveMessageFromMessageLayer(cachedMessage *message.CachedMessage, cachedMessageMetadata *messageTangle.CachedMessageMetadata) {
	defer cachedMessage.Release()
	defer cachedMessageMetadata.Release()

	solidMessage := cachedMessage.Unwrap()
	if solidMessage == nil {
		log.Debug("failed to unpack solid message from message layer")

		return
	}

	messagePayload := solidMessage.Payload()
	if messagePayload.Type() != Type {
		return
	}

	// check for node identity -> only answer if it's send from a configured node
	issuerPubKey := solidMessage.IssuerPublicKey()
	if issuerPubKey != originPublicKey || issuerPubKey == myPublicKey {
		return
	}

	// make sure we really got a message with our type. 
	// since we are in a distributed setting anyone could send a message with our type id and put other data in it.
	networkDelayObject, ok := messagePayload.(*Object)
	if !ok {
		log.Info("could not cast payload to network delay object")

		return
	}

	now := time.Now().UnixNano()

	sendToRemoteLog(networkDelayObject, now)
}
```

 