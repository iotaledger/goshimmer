export enum WSMsgType {
    Status,
    MPSMetrics,
    Message,
    NeighborStats,
    Drng,
    TipsMetrics,
    Vertex,
    TipInfo,
}

export interface WSMessage {
    type: number;
    data: any;
}

type DataHandler = (data: any) => void;

let handlers = {};

export function registerHandler(msgTypeID: number, handler: DataHandler) {
    handlers[msgTypeID] = handler;
}

export function unregisterHandler(msgTypeID: number) {
    delete handlers[msgTypeID];
}

export function connectWebSocket(path: string, onOpen, onClose, onError) {
    let loc = window.location;
    let uri = 'ws:';

    if (loc.protocol === 'https:') {
        uri = 'wss:';
    }
    uri += '//' + loc.host + path;

    let ws = new WebSocket(uri);

    ws.onopen = onOpen;
    ws.onclose = onClose;
    ws.onerror = onError;

    ws.onmessage = (e) => {
        let msg: WSMessage = JSON.parse(e.data);
        let handler = handlers[msg.type];
        if (!handler) {
            return;
        }
        handler(msg.data);
    };
}
