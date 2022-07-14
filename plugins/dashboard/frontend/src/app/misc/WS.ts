export enum WSMsgType {
    Status,
    BPSMetrics,
    Block,
    NeighborStats,
    ComponentCounterMetrics,
    TipsMetrics,
    Vertex,
    TipInfo,
    Mana,
    ManaMapOverall,
    ManaMapOnline,
    ManaAllowedPledge,
    ManaPledge,
    ManaInitPledge,
    ManaRevoke,
    ManaInitRevoke,
    ManaInitDone,
    BlkManaDashboardAddress,
    Chat,
    RateSetter,
    ConflictSet,
    Conflict,
}

export interface WSBlock {
    type: number;
    data: any;
}

type DataHandler = (data: any) => void;

let handlers = {};

export function registerHandler(blkTypeID: number, handler: DataHandler) {
    handlers[blkTypeID] = handler;
}

export function unregisterHandler(blkTypeID: number) {
    delete handlers[blkTypeID];
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
        let blk: WSBlock = JSON.parse(e.data);
        let handler = handlers[blk.type];
        if (!handler) {
            return;
        }
        handler(blk.data);
    };
}
