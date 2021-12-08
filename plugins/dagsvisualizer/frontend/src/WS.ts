export enum WSMsgType {
	Message,
	MessageBooked,
	MessageConfirmed,
	FutureMarkerUpdated,
	Transaction,
	TransactionConfirmed,
	Branch,
	BranchParentsUpdate,
    BranchConfirmed,
    BranchWeightChanged
}

export interface WSMessage {
    type: number;
    data: any;
}

type DataHandler = (data: any) => void;

let handlers = {};

export function registerHandler(msgType: number, handler: DataHandler) {
    handlers[msgType] = handler;
}

export function unregisterHandler(msgType: number) {
    delete handlers[msgType];
}

export function connectWebSocket(path: string, onOpen, onClose, onError) {
    let loc = window.location;
    let uri = 'ws:';

    if (loc.protocol === 'https:') {
        uri = 'wss:';
    }
    uri += '//' + loc.host + path;
    //uri += '//' + path;
    let ws = new WebSocket(uri);

    ws.onopen = onOpen;
    ws.onclose = onClose;
    ws.onerror = onError;

    ws.onmessage = (e) => {
        let wsMsg: WSMessage = JSON.parse(e.data)
        let handler: DataHandler = handlers[wsMsg.type]
        if (handler != null) {
            handler(wsMsg.data)
        }
    };
}
