---
description: Learn how to write simple dApps as GoShimmer plugins such as a chat dApp and a network delay dApp.
image: /img/logo/goshimmer_light.png
keywords:
- chat 
- payload
- block
- bytes layout
- web api endpoint
---
# How to Create dApps
 
:::info

This guide is meant for developers familiar with the Go programming language.

:::

:::warning DISCLAIMER

GoShimmer is a rapidly evolving prototype software. As such, the described steps here will likely change in the future. Specifically, we are envisioning to ease the process of dApp creation and installation for node owners. Furthermore, the current approach is in no way hardened and should be seen as purely experimental. Do not write any software for actual production use.

:::

## Introduction

Throughout this tutorial we will learn how to write simple dApps as GoShimmer plugins. We provide two different examples: a chat dApp and a network delay dApp. Hope you enjoy the reading!

## Chat dApp

In this guide we are going to explain how to write a very simple chat dApp so that anyone, connected to a GoShimmer node, could write a short block and read what is being written into the Tangle.

The complete source code of the application can be found [in the repository](https://github.com/iotaledger/goshimmer/tree/develop/plugins/chat). 

### Overview

Our chat dApp can be implemented in a few simple steps:
1. A node sends a special block containing a chat payload via the Tangle.
2. Upon receipt, every other node in the network processes this block and - if the chat dApp/plugin is enabled - triggers an event that a chat block has been received.

Within GoShimmer we need 3 components to realize this undertaking. First, we need to **define and register a chat payload type**. Second, we need a way to **initiate a block** with a chat payload via the web API. And lastly, we need to **listen** for chat payloads and take appropriate action.

If a node does not have our chat dApp installed and activated, the chat block will be simply treated as a raw data block without any particular meaning. In general that means that in order for a dApp to be useful, node owners need to explicitly install it. In our case we simply ship it with GoShimmer as a [plugin](../implementation_design/plugin.md).

### Define & Register The Chat Payload

First, we need to decide what data our chat payload should contain and define the byte layout accordingly.
In our case we need a `From` field to identify the sender of the block (e.g., a nickname, the ID of the node); a `To` field to identify an optional recipient of the block (e.g., a chat room ID, a nickname); a `Block` field containing the actual chat block.
Therefore, we can define the byte layout as follows:
```
length<uint32-4bytes> // every payload has to have this
type<uint32-4bytes> // every payload has to have this
From<string>
To<string>
Block<string>
```

Next, we need to fulfill the `Payload` interface and provide the functionality to read/write a payload from/to bytes. The [`hive.go/marshalutil`](https://github.com/iotaledger/hive.go/tree/master/marshalutil) package simplifies this step tremendously.
```Go
// Payload represents the generic interface for a payload that can be embedded in Blocks of the Tangle.
type Payload interface {
    // Type returns the Type of the Payload.
    Type() Type
    
    // Bytes returns a marshaled version of the Payload.
    Bytes() []byte
    
    // String returns a human readable version of the Payload.
    String() string
}
```

Finally, we need to create and register our chat payload type so that it can be properly unmarshalled. 
```Go
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

In order to issue a block with our newly created chat payload, we need to create a web API endpoint. Here we simply bind a json request containing the necessary fields: `from`, `to` and `block` and then issue it into the Tangle with `blocklayer.Tangle().IssuePayload(chatPayload)`. This plugin takes care of all the specifics and employs the `BlockFactory` to, i.a., select tips and sign the block.

```Go
webapi.Server().POST("chat", SendChatBlock)

// SendChatBlock sends a chat block.
func SendChatBlock(c echo.Context) error {
	req := &Request{}
	if err := c.Bind(req); err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels.NewErrorResponse(err))
	}
	chatPayload := NewPayload(req.From, req.To, req.Block)

	blk, err := blocklayer.Tangle().IssuePayload(chatPayload)
	if err != nil {
		return c.JSON(http.StatusBadRequest, Response{Error: err.Error()})
	}

	return c.JSON(http.StatusOK, Response{BlockID: blk.ID().Base58()})
}
```

### Listen for Chat Payloads

Every dApp listens for blocks from the *communication layer* and when its payload type is detected, takes appropriate action. For us that means listening for chat payload type and triggering an event if we encounter any. In this case the event will contain information about the chat block and also the `BlockID` in terms of a Tangle block as well as its issuance timestamp.

```Go
func onReceiveBlockFromBlockLayer(blockID tangle.BlockID) {
	var chatEvent *ChatEvent
	blocklayer.Tangle().Storage.Block(blockID).Consume(func(block *tangle.Block) {
		if block.Payload().Type() != Type {
			return
		}

		chatPayload, _, err := FromBytes(block.Payload().Bytes())
		if err != nil {
			app.LogError(err)
			return
		}

		chatEvent = &ChatEvent{
			From:      chatPayload.From,
			To:        chatPayload.To,
			Block:   chatPayload.Block,
			Timestamp: block.IssuingTime(),
			BlockID: block.ID().Base58(),
		}
	})

	if chatEvent == nil {
		return
	}

	app.LogInfo(chatEvent)
	Events.BlockReceived.Trigger(chatEvent)
}
```

## Network Delay dApp

In this guide we are going to explain how to write a very simple dApp based on an actual dApp we are using in GoShimmer to help us measure the network delay, i.e., how long it takes for every active node in the network to receive a block.

The complete source code of the application can be found [in the repository](https://github.com/iotaledger/goshimmer/tree/develop/plugins/networkdelay). 

### Overview

Our network delay dApp should help us to identify the time it takes for every active node to receive and process a block. That can be done in a few simple steps:
1. A (known) node sends a special block containing a network delay payload.
2. Upon receipt, every other node in the network answers to the special block by posting its current time to our remote logger.
3. For simplicity, we gather the information in an [ELK stack](https://www.elastic.co/what-is/elk-stack). This helps us to easily interpret and analyze the data.

Within GoShimmer we need 3 components to realize this undertaking. First, we need to **define and register a network delay payload type**. Second, we need a way to **initiate a block** with a network delay payload via the web API. And lastly, we need to **listen** for network delay payloads and take appropriate action.

If a node does not have our dApp installed and activated, the block will be simply treated as a raw data block without any particular meaning. In general that means that in order for a dApp to be useful, node owners need to explicitly install it. In our case we simply ship it with GoShimmer as a [plugin](../implementation_design/plugin.md).

### Define & Register The Network Delay Object

First, we need to decide what data our network delay payload should contain and define the byte layout accordingly.
In our case we need an `ID` to identify a network delay block and the `sent time` of the initiator. 
Therefore, we can define the byte layout as follows:
```
length<uint32-4bytes> // every payload has to have this
type<uint32-4bytes> // every payload has to have this
id<32bytes>
sentTime<int64-8bytes>
```

Next, we need to fulfill the `Payload` interface and provide the functionality to read/write a payload from/to bytes. The [`hive.go/marshalutil`](https://github.com/iotaledger/hive.go/tree/master/marshalutil) package simplifies this step tremendously.
```Go
// Payload represents the generic interface for a payload that can be embedded in Blocks of the Tangle.
type Payload interface {
    // Type returns the Type of the Payload.
    Type() Type
    
    // Bytes returns a marshaled version of the Payload.
    Bytes() []byte
    
    // String returns a human readable version of the Payload.
    String() string
}
```

Finally, we need to create and register our network delay payload type so that it can be properly unmarshalled. 
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

In order to issue a block with our newly created network delay payload, we need to create a web API endpoint. Here we simply create a random `ID` and the `sentTime` and then issue a block with `issuer.IssuePayload()`. This plugin takes care of all the specifics and employs the `BlockFactory` to, i.a., select tips and sign the block.

```Go
webapi.Server.POST("networkdelay", broadcastNetworkDelayObject)

func broadcastNetworkDelayObject(c echo.Context) error {
	// generate random id
	rand.Seed(time.Now().UnixNano())
	var id [32]byte
	if _, err := rand.Read(id[:]); err != nil {
		return c.JSON(http.StatusInternalServerError, Response{Error: err.Error()})
	}

	blk, err := issuer.IssuePayload(NewObject(id, time.Now().UnixNano()))
	if err != nil {
		return c.JSON(http.StatusBadRequest, Response{Error: err.Error()})
	}
	return c.JSON(http.StatusOK, Response{ID: blk.Id().String()})
}
```


### Listen for Network Delay Payloads

Every dApp listens for blocks from the *communication layer* and when its data type is detected, takes appropriate action. For us that means listening for network delay payloads and sending blocks to our remote logger if we encounter any. Of course in this context, we only want to react to network delay payloads which were issued by our analysis/entry node server. Therefore, matching the block signer's public key with a configured public key lets us only react to the appropriate network delay payloads.

```Go
func onReceiveBlockFromBlockLayer(blockID tangle.BlockID) {
    blocklayer.Tangle().Storage.Block(blockID).Consume(func(solidBlock *tangle.Block) {
        blockPayload := solidBlock.Payload()
        if blockPayload.Type() != Type {
            return
        }
    
        // check for node identity
        issuerPubKey := solidBlock.IssuerPublicKey()
        if issuerPubKey != originPublicKey || issuerPubKey == myPublicKey {
            return
        }
        
        networkDelayObject, ok := blockPayload.(*Object)
        if !ok {
            app.LogInfo("could not cast payload to network delay payload")
            return
        }
        
        now := clock.SyncedTime().UnixNano()
        
        // abort if block was sent more than 1min ago
        // this should only happen due to a node resyncing
        if time.Duration(now-networkDelayObject.sentTime) > time.Minute {
            app.LogDebugf("Received network delay block with >1min delay\n%s", networkDelayObject)
        return
        }
    
        sendToRemoteLog(networkDelayObject, now)
    })
}
```
