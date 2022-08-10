import {WSBlkTypeDashboard} from "../models/ws/WSBlkTypeDashboard";
import {WSBlock} from "../models/ws/IWSBlk";
import {IManaBlock} from "../models/mana/IManaBlock";
import {INetworkManaBlock} from "../models/mana/INetworkManaBlock";
import {IPledgeBlock} from "../models/mana/IPledgeBlock";
import {IRevokeBlock} from "../models/mana/IRevokeBlock";

type DataHandler<T> = (data: T) => void;

const handlers: { [id in WSBlkTypeDashboard]?: DataHandler<unknown> } = {};

export function registerHandler(blkTypeID: WSBlkTypeDashboard.Mana, handler: DataHandler<IManaBlock>);
export function registerHandler(blkTypeID: WSBlkTypeDashboard.ManaMapOverall, handler: DataHandler<INetworkManaBlock>);
export function registerHandler(blkTypeID: WSBlkTypeDashboard.ManaMapOnline, handler: DataHandler<INetworkManaBlock>);
export function registerHandler(blkTypeID: WSBlkTypeDashboard.ManaPledge, handler: DataHandler<IPledgeBlock>);
export function registerHandler(blkTypeID: WSBlkTypeDashboard.ManaInitPledge, handler: DataHandler<IPledgeBlock>);
export function registerHandler(blkTypeID: WSBlkTypeDashboard.ManaRevoke, handler: DataHandler<IRevokeBlock>);
export function registerHandler(blkTypeID: WSBlkTypeDashboard.ManaInitRevoke, handler: DataHandler<IRevokeBlock>);
export function registerHandler(blkTypeID: WSBlkTypeDashboard.ManaInitDone, handler: DataHandler<null>);


export function registerHandler<T>(blkTypeID: number, handler: DataHandler<T>): void {
    handlers[blkTypeID] = handler;
}

export function unregisterHandler(blkTypeID: number): void {
    delete handlers[blkTypeID];
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
        const blk: WSBlock = JSON.parse(e.data) as WSBlock;
        const handler = handlers[blk.type];
        if (handler) {
            handler(blk.data);
        }
    };
}
