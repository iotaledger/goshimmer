import { WSMsgType } from "../models/ws/wsMsgType";
import { WSMessage } from "../models/ws/IWSMsg";
import {IManaMessage} from "../models/mana/IManaMessage";
import {INetworkManaMessage} from "../models/mana/INetworkManaMessage";
import {IPledgeMessage} from "../models/mana/IPledgeMessage";
import {IRevokeMessage} from "../models/mana/IRevokeMessage";

type DataHandler<T> = (data: T) => void;

const handlers: { [id in WSMsgType]?: DataHandler<unknown> } = {};

export function registerHandler(msgTypeID: WSMsgType.Mana, handler: DataHandler<IManaMessage>);
export function registerHandler(msgTypeID: WSMsgType.ManaMapOverall, handler: DataHandler<INetworkManaMessage>);
export function registerHandler(msgTypeID: WSMsgType.ManaMapOnline, handler: DataHandler<INetworkManaMessage>);
export function registerHandler(msgTypeID: WSMsgType.ManaPledge, handler: DataHandler<IPledgeMessage>);
export function registerHandler(msgTypeID: WSMsgType.ManaRevoke, handler: DataHandler<IRevokeMessage>);


export function registerHandler<T>(msgTypeID: number, handler: DataHandler<T>): void {
    handlers[msgTypeID] = handler;
}

export function unregisterHandler(msgTypeID: number): void {
    delete handlers[msgTypeID];
}

export function connectDashboardWebSocket(
    address: string,
    onOpen: () => void,
    onClose: () => void,
    onError: () => void): void {
    const loc = new URL(address)
    let uri = "ws:";

    if (loc.protocol === "https:") {
        uri = "wss:";
    }
    uri += "//" + loc.host + "/ws";

    const ws = new WebSocket(uri);

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
