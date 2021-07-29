
# How to create dApps

:::info
This guide is meant for developers familiar with the Go programming language.
:::

:::warning
**DISCLAIMER:** GoShimmer is a rapidly evolving prototype software. As such, the described steps in this documentation will likely change in the future.

Specifically, we are envisioning to ease the process of dApp creation and installation for node owners. 

Furthermore, the current approach is in no way hardened and should be seen as purely experimental. 

Do not write any software for actual production use.
:::

## Introduction

This tutorial is focused on how to write simple dApps as GoShimmer plugins.  You can find two different examples: 

- [Chat dApp](#chat-dapp).
- [Network delay dApp](#network-delay-dapp).

## Chat dApp

### Introduction

This section will explain how to write a simple chat dApp.  The chat dApp will allow anyone connected to a GoShimmer node to write a short message, and read what is being written into the Tangle.

The complete source code of the application can be found in the [official GitHub repository](https://github.com/iotaledger/goshimmer/tree/develop/plugins/chat). 

### Overview

A chat dApp can be implemented in two steps:

1. A node sends a special message containing a chat payload via the Tangle.
2. Upon receipt, every other node in the network processes this message and - if the chat dApp/plugin is enabled - triggers an event that a chat message has been received.

Within GoShimmer you will need 3 components to realize this undertaking.

1. [Define and register a chat payload type](#define--register-the-chat-payload). 
2. [Initiate a message with a chat payload via the web API](#create-the-web-api-endpoints)
3. [Listen for chat payloads and take appropriate action](#listen-for-chat-payloads).

If a node does not have your chat dApp installed and activated, the chat message will be treated as a raw data message without any particular meaning. In general that means that in order for a dApp to be useful, node owners need to explicitly install it. In this case, we can ship it with GoShimmer as a [plugin](../implementation_design/plugin.md).

### Define & Register The Chat Payload

The first thing you need to decide is what data your chat payload should contain, and define the byte layout accordingly. In this case you will need:

- A `From` field to identify the sender of the message (e.g., a nickname, the ID of the node).
- A `To` field to identify an optional recipient of the message (e.g., a chat room ID, a nickname). 
- A `Message` field containing the actual chat message.

Therefore, you can define the byte layout as follows:

```go
length<uint32-4bytes> // every payload has to have this
type<uint32-4bytes> // every payload has to have this
From<string>
To<string>
Message<string>
```

Next, you will need to fulfill the `Payload` interface, and provide the functionality to read/write a payload from/to bytes. The [`hive.go/marshalutil`](https://github.com/iotaledger/hive.go/tree/master/marshalutil) package simplifies this step tremendously.

```go
// Payload represents the generic interface for a payload that can be embedded in Messages of the Tangle.
type Payload interface {
    // Type returns the Type of the Payload.
    Type() Type
    
    // Bytes returns a marshaled version of the Payload.
    Bytes() []byte
    
    // String returns a human readable version of the Payload.
    String() string
}
```

Finally, you need to create and register our chat payload type so that it can be properly unmarshalled.

```go
// Type represents the identifier which addresses the chat payload type.
var Type = payload.NewType(payloadType, PayloadName, func(data []byte) (payload payload.Payload, err error) {
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

In order to issue a message with your newly created chat payload, you need to create a web API endpoint. The following example binds a json request containing the necessary fields `from`, `to` and `message`,  and then issues it into the Tangle with `messagelayer.Tangle().IssuePayload(chatPayload)`. This plugin takes care of all the specifics and employs the `MessageFactory` to select tips and sign the message.

```go
webapi.Server().POST("chat", SendChatMessage)

// SendChatMessage sends a chat message.
func SendChatMessage(c echo.Context) error {
	req := &Request{}
	if err := c.Bind(req); err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels.NewErrorResponse(err))
	}
	chatPayload := NewPayload(req.From, req.To, req.Message)

	msg, err := messagelayer.Tangle().IssuePayload(chatPayload)
	if err != nil {
		return c.JSON(http.StatusBadRequest, Response{Error: err.Error()})
	}

	return c.JSON(http.StatusOK, Response{MessageID: msg.ID().Base58()})
}
```

### Listen for chat payloads

Every dApp listens for messages from the _communication layer_, and when it detects its payload type, takes appropriate action. In this example, that means listening for chat payload type, and triggering an event if we encounter any. In this case the event will contain information about the chat message, and also the _MessageID_ (in terms of a Tangle message), as well as its issuance timestamp.

```go
func onReceiveMessageFromMessageLayer(messageID tangle.MessageID) {
	var chatEvent *ChatEvent
	messagelayer.Tangle().Storage.Message(messageID).Consume(func(message *tangle.Message) {
		if message.Payload().Type() != Type {
			return
		}

		chatPayload, _, err := FromBytes(message.Payload().Bytes())
		if err != nil {
			app.LogError(err)
			return
		}

		chatEvent = &ChatEvent{
			From:      chatPayload.From,
			To:        chatPayload.To,
			Message:   chatPayload.Message,
			Timestamp: message.IssuingTime(),
			MessageID: message.ID().Base58(),
		}
	})

	if chatEvent == nil {
		return
	}

	app.LogInfo(chatEvent)
	Events.MessageReceived.Trigger(chatEvent)
}
```

## Network Delay dApp

### Introduction

This section will explain how to write a very simple dApp based on an actual dApp used in GoShimmer to help measure the network delay (how long it takes every active node in the network to receive a message). Gathering this data will enable us to set realistic parameters for FCoB.

The complete source code of the application can be found in the [official GitHub repository](https://github.com/iotaledger/goshimmer/tree/develop/plugins/networkdelay). 

### Overview

A network delay dApp should help you identify the time it takes for every active node to receive and process a message. That can be done in three simple steps:

1. A (known) node sends a special message containing a network delay payload.
2. Upon receipt, every other node in the network answers to the special message by posting its current time to our remote logger.
3. For simplicity, we gather the information in an [ELK stack](https://www.elastic.co/what-is/elk-stack). This helps us to easily interpret and analyze the data.

Within GoShimmer you will need 3 components to realize this undertaking:

1. You need to [define and register a network delay payload type](#define--register-the-network-delay-object). 
2. You need a way to [initiate a message with a network delay payload via the web API](#create-the-web-api-endpoints). 
3. You need to [listen for network delay payloads](#listen-for-network-delay-payloads), and take appropriate action.

If a node does not have your chat dApp installed and activated, the chat message will be treated as a raw data message without any particular meaning. In general that means that in order for a dApp to be useful, node owners need to explicitly install it. In this case, we can ship it with GoShimmer as a [plugin](../implementation_design/plugin.md).

### Define & Register The Network Delay Object

The first thing you need to decide is what data your network delay payload should contain, and define the byte layout accordingly. In this case you will need an `ID` to identify a network delay message, and the `sent time` of the initiator. 

Therefore, you can define the byte layout as follows:

```go
length<uint32-4bytes> // every payload has to have this
type<uint32-4bytes> // every payload has to have this
id<32bytes>
sentTime<int64-8bytes>
```

Next, you will need to fulfill the `Payload` interface, and provide the functionality to read/write a payload from/to bytes. The [`hive.go/marshalutil`](https://github.com/iotaledger/hive.go/tree/master/marshalutil) package simplifies this step tremendously.

```go
// Payload represents the generic interface for a payload that can be embedded in Messages of the Tangle.
type Payload interface {
    // Type returns the Type of the Payload.
    Type() Type
    
    // Bytes returns a marshaled version of the Payload.
    Bytes() []byte
    
    // String returns a human readable version of the Payload.
    String() string
}
```

Finally, you need to create and register your network delay payload type so that it can be properly unmarshalled.

```go
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

In order to issue a message with your newly created network delay payload, you need to create a web API endpoint. Here you can simply create a random `ID` and the `sentTime`, and then issue a message using the  `issuer.IssuePayload()` method. This plugin takes care of all the specifics, and employs the `MessageFactory` to select tips and sign the message.

```go
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


### Listen for network delay payloads

Every dApp listens for messages from the _communication layer_, and when it detects its data type, takes appropriate action. In this case that means listening for network delay payloads and sending messages to our remote logger if we encounter any. In this context, you only want to react to network delay payloads which were issued by your analysis/entry node server. Therefore, matching the message signer's public key with a configured public key lets the dApp only react to the appropriate network delay payloads.

```go
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
            app.LogInfo("could not cast payload to network delay payload")
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
