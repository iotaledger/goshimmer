import {IAddNodeBlock} from "../models/blocks/IAddNodeBlock";
import {IRemoveNodeBlock} from "../models/blocks/IRemoveNodeBlock";
import {IConnectNodesBlock} from "../models/blocks/IConnectNodesBlock";
import {IDisconnectNodesBlock} from "../models/blocks/IDisconnectNodesBlock";
import {WSBlkType} from "../models/ws/wsBlkType";
import {WSBlock} from "../models/ws/IWSBlk";

type DataHandler<T> = (data: T) => void;

const handlers: { [id in WSBlkType]?: DataHandler<unknown> } = {};

export function registerHandler(blkTypeID: WSBlkType.addNode, handler: DataHandler<IAddNodeBlock>);
export function registerHandler(blkTypeID: WSBlkType.removeNode, handler: DataHandler<IRemoveNodeBlock>);
export function registerHandler(blkTypeID: WSBlkType.connectNodes, handler: DataHandler<IConnectNodesBlock>);
export function registerHandler(blkTypeID: WSBlkType.disconnectNodes, handler: DataHandler<IDisconnectNodesBlock>);
export function registerHandler(blkTypeID: WSBlkType.BlkManaDashboardAddress, handler: DataHandler<string>);

export function registerHandler<T>(blkTypeID: number, handler: DataHandler<T>): void {
    handlers[blkTypeID] = handler;
}

export function unregisterHandler(blkTypeID: number): void {
    delete handlers[blkTypeID];
}

let ws: WebSocket | null

export function sendBlock(blk: Object) {
    if (ws) {
        ws.send(JSON.stringify(blk))
    }
}

export function connectWebSocket(
    path: string,
    onOpen: () => void,
    onClose: () => void,
    onError: () => void): void {

    if (ws) {
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
        const blk: WSBlock = JSON.parse(e.data) as WSBlock;
        // Just a ping, do nothing
        if (blk.type === WSBlkType.ping) {
            return;
        }
        const handler = handlers[blk.type];
        if (handler) {
            handler(blk.data);
        }
    };
}
