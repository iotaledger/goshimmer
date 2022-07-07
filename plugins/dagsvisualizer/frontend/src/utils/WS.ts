export enum WSBlkType {
    Block,
    BlockBooked,
    BlockConfirmed,
    BlockTxConfirmationStateChanged,
    Transaction,
    TransactionBooked,
    TransactionConfirmationStateChanged,
    Conflict,
    ConflictParentsUpdate,
    ConflictConfirmationStateChanged,
    ConflictWeightChanged
}

export interface WSBlock {
    type: number;
    data: any;
}

type DataHandler = (data: any) => void;

const handlers = {};

export function registerHandler(blkType: number, handler: DataHandler) {
    handlers[blkType] = handler;
}

export function unregisterHandler(blkType: number) {
    delete handlers[blkType];
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
        const wsBlk: WSBlock = JSON.parse(e.data);
        const handler: DataHandler = handlers[wsBlk.type];
        if (handler != null) {
            handler(wsBlk.data);
        }
    };
}
