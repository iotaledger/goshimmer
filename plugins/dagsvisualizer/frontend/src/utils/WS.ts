export enum WSMsgType {
    Message,
    MessageBooked,
    MessageConfirmed,
    MessageTxGoFChanged,
    FutureMarkerUpdated,
    Transaction,
    TransactionBooked,
    TransactionGoFChanged,
    Branch,
    BranchParentsUpdate,
    BranchGoFChanged,
    BranchWeightChanged
}

export interface WSMessage {
    type: number;
    data: any;
}

type DataHandler = (data: any) => void;

const handlers = {};

export function registerHandler(msgType: number, handler: DataHandler) {
    handlers[msgType] = handler;
}

export function unregisterHandler(msgType: number) {
    delete handlers[msgType];
}

export function connectWebSocket(path: string, onOpen, onClose, onError) {
    const loc = window.location;
    let uri = 'ws:';

    if (loc.protocol === 'https:') {
        uri = 'wss:';
    }
    uri += '//' + loc.host + path;
    //uri += '//' + path;
    const ws = new WebSocket(uri);

    ws.onopen = onOpen;
    ws.onclose = onClose;
    ws.onerror = onError;

    ws.onmessage = (e) => {
        const wsMsg: WSMessage = JSON.parse(e.data);
        const handler: DataHandler = handlers[wsMsg.type];
        if (handler != null) {
            handler(wsMsg.data);
        }
    };
}
