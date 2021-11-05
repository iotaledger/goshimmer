import { IAddNodeMessage } from "../models/messages/IAddNodeMessage";
import { IRemoveNodeMessage } from "../models/messages/IRemoveNodeMessage";
import { IConnectNodesMessage } from "../models/messages/IConnectNodesMessage";
import { IDisconnectNodesMessage } from "../models/messages/IDisconnectNodesMessage";
import {WSMsgType} from "../models/ws/wsMsgType";
import { WSMessage } from "../models/ws/IWSMsg";

type DataHandler<T> = (data: T) => void;

const handlers: { [id in WSMsgType]?: DataHandler<unknown> } = {};

export function registerHandler(msgTypeID: WSMsgType.addNode, handler: DataHandler<IAddNodeMessage>);
export function registerHandler(msgTypeID: WSMsgType.removeNode, handler: DataHandler<IRemoveNodeMessage>);
export function registerHandler(msgTypeID: WSMsgType.connectNodes, handler: DataHandler<IConnectNodesMessage>);
export function registerHandler(msgTypeID: WSMsgType.disconnectNodes, handler: DataHandler<IDisconnectNodesMessage>);
export function registerHandler(msgTypeID: WSMsgType.MsgManaDashboardAddress, handler: DataHandler<string>);

export function registerHandler<T>(msgTypeID: number, handler: DataHandler<T>): void {
    handlers[msgTypeID] = handler;
}

export function unregisterHandler(msgTypeID: number): void {
    delete handlers[msgTypeID];
}

let ws: WebSocket | null

export function sendMessage(msg: Object){
    if(ws){
        ws.send(JSON.stringify(msg))
    }
}

export function connectWebSocket(
    path: string,
    onOpen: () => void,
    onClose: () => void,
    onError: () => void): void {

    if (ws){
        return
    }
    const loc = window.location;
    let uri = "ws:";

    if (loc.protocol === "https:") {
        uri = "wss:";
    }
    uri += "//" + loc.host + path;

    ws = new WebSocket(uri);

    ws.onopen = onOpen;
    ws.onclose = onClose;
    ws.onerror = onError;

    ws.onmessage = (e) => {
        const msg: WSMessage = JSON.parse(e.data) as WSMessage;
        // Just a ping, do nothing
        if (msg.type === WSMsgType.ping) {
            return;
        }
        const handler = handlers[msg.type];
        if (handler) {
            handler(msg.data);
        }
    };
}
