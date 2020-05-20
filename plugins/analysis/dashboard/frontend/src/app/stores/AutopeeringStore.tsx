import {RouterStore} from "mobx-react-router";
import {action} from "mobx";
//import * as React from "react";
import {registerHandler, WSMsgType} from "app/misc/WS";




export class AddNodeMessage {
    id: string;
}

export class RemoveNodeMessage {
    id: string;
}

export class ConnectNodesMessage {
    source: string;
    target: string
}

export class DisconnectNodesMessage {
    source: string;
    target: string
}

export class AutopeeringStore {
    routerStore: RouterStore;


    constructor(routerStore: RouterStore) {
        this.routerStore = routerStore;

        registerHandler(WSMsgType.AddNode, this.addNode);
        registerHandler(WSMsgType.RemoveNode, this.removeNode);
        registerHandler(WSMsgType.ConnectNodes, this.connectNodes);
        registerHandler(WSMsgType.RemoveNode, this.disconnectNodes);
    }

    @action
    addNode = (msg: AddNodeMessage) => {
        console.log(msg)
    }

    @action
    removeNode = (msg: RemoveNodeMessage) => {
        console.log(msg)
    }

    @action
    connectNodes = (msg: ConnectNodesMessage) => {
        console.log(msg)
    }

    @action
    disconnectNodes = (msg: DisconnectNodesMessage) => {
        console.log(msg)
    }


}

export default AutopeeringStore;