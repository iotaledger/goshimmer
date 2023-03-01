import {action, computed, observable} from 'mobx';
import {registerHandler, WSMsgType} from "app/misc/WS";
import {
    BasicPayload,
    getPayloadType,
    Output,
    PayloadType,
    SigLockedSingleOutput,
    TransactionPayload,
    FaucetPayload,
    Transaction
} from "app/misc/Payload";
import * as React from "react";
import {Link} from 'react-router-dom';
import {RouterStore} from "mobx-react-router";

export const GenesisBlockID = "1111111111111111111111111111111111111111111111111111111111111111";
export const GenesisTransactionID = "11111111111111111111111111111111";

export class Block {
    id: string;
    solidification_timestamp: number;
    issuance_timestamp: number;
    sequence_number: number;
    issuer_public_key: string;
    issuer_short_id: string;
    signature: string;
    parentsByType: Map<string, Array<string>>;
    strongChildren: Array<string>;
    weakChildren: Array<string>;
    shallowLikeChildren: Array<string>;
    solid: boolean;
    conflictIDs: Array<string>;
    addedConflictIDs: Array<string>;
    subtractedConflictIDs: Array<string>;
    scheduled: boolean;
    booked: boolean;
    orphaned: boolean;
    objectivelyInvalid: boolean;
    subjectivelyInvalid: boolean;
    acceptance: boolean;
    acceptanceTime: number;
    confirmation: boolean;
    confirmationTime: number;
    confirmationBySlot: boolean;
    confirmationBySlotTime: number;
    payload_type: number;
    payload: any;
    rank: number;
    sequenceID: number;
    isPastMarker: boolean;
    pastMarkerGap: number;
    pastMarkers: string;
    ec: string;
    ei: number;
    ecr: string;
    prevEC: string;
    cumulativeWeight: number;
    latestConfirmedSlot: number;
}

export class AddressResult {
    address: string;
    explorerOutputs: Array<ExplorerOutput>;
}

export class ExplorerOutput {
    id: OutputID;
    output: Output;
    metadata: OutputMetadata
    txTimestamp: number;
    pendingMana: number;
}

class OutputID {
    base58: string;
    transactionID: string;
    outputIndex: number;
}

export class OutputMetadata {
    outputID: OutputID;
    conflictIDs: Array<string>;
    consumerCount: number;
    confirmedConsumer: string // tx id of confirmed consumer
    confirmationState: number
    confirmationStateTime: number
}

class OutputConsumer {
    transactionID: string;
    valid: string;
}

class OutputConsumers {
    outputID: OutputID;
    consumers: Array<OutputConsumer>
}

class TransactionMetadata {
    transactionID: string;
    conflictIDs: string[];
    booked: boolean;
    bookedTime: number;
    confirmationState: number;
    confirmationStateTime: number;
}

class PendingMana {
    mana: number;
    outputID: string;
    error: string;
    timestamp: number;
}

class Conflict {
    id: string;
    parents: Array<string>;
    conflictIDs: Array<string>;
    confirmationState: number;
}

class ConflictChildren {
    conflictID: string;
    childConflicts: Array<ConflictChild>
}

class ConflictChild {
    conflictID: string;
    type: string;
}

class ConflictConflict {
    outputID: OutputID;
    conflictIDs: Array<string>;
}

class ConflictConflicts {
    conflictID: string;
    conflicts: Array<ConflictConflict>
}

class ConflictVoters {
    conflictID: string;
    voters: Array<string>
}

class SlotInfo {
    id: string;
	index: number;
	rootsID: string;
	prevID: string ;
	cumulativeWeight: number;
}

class SlotBlocks {
    blocks: Array<string>;
}

class SlotTransactions {
    transactions: Array<string>;
}

class SlotUTXOs {
    createdOutputs: Array<string>;
    spentOutputs: Array<string>;
}
class SearchResult {
    block: BlockRef;
    address: AddressResult;
}

class BlockRef {
    id: string;
    payload_type: number;
}

class Tips {
    tips: Array<string>
}

const liveFeedSize = 50;

enum QueryError {
    NotFound = 1,
    BadRequest = 2
}

export class ExplorerStore {
    // live feed
    @observable latest_blocks: Array<BlockRef> = [];

    // queries
    @observable blk: Block = null;
    @observable addr: AddressResult = null;
    @observable tx: any = null;
    @observable txMetadata: any = null;
    @observable txAttachments: any = [];
    @observable output: any = null;
    @observable outputMetadata: OutputMetadata = null;
    @observable outputConsumers: OutputConsumers = null;
    @observable pendingMana: PendingMana = null;
    @observable conflict: Conflict = null;
    @observable conflictChildren: ConflictChildren = null;
    @observable conflictConflicts: ConflictConflicts = null;
    @observable conflictVoters: ConflictVoters = null;
    @observable tips: Tips = null;
    @observable slotInfo: SlotInfo = new SlotInfo;
    @observable slotBlocks: SlotBlocks = new SlotBlocks;
    @observable slotTransactions: SlotTransactions = new SlotTransactions;
    @observable slotUtxos: SlotUTXOs = new SlotUTXOs;

    // loading
    @observable query_loading: boolean = false;
    @observable query_err: any = null;

    // search
    @observable search: string = "";
    @observable search_result: SearchResult = null;
    @observable searching: boolean = false;
    @observable payload: any;
    @observable subpayload: any;

    routerStore: RouterStore;

    constructor(routerStore: RouterStore) {
        this.routerStore = routerStore;
        registerHandler(WSMsgType.Block, this.addLiveFeedBlock);
    }

    searchAny = async () => {
        this.updateSearching(true);
        try {
            let res = await fetch(`/api/search/${this.search}`);
            let result: SearchResult = await res.json();
            this.updateSearchResult(result);
        } catch (err) {
            this.updateQueryError(err);
        }
    };

    @action
    resetSearch = () => {
        this.search_result = null;
        this.searching = false;
    };

    @action
    updateSearchResult = (result: SearchResult) => {
        this.search_result = result;
        this.searching = false;
        let search = this.search;
        this.search = '';
        if (this.search_result.block) {
            this.routerStore.push(`/explorer/block/${search}`);
            return;
        }
        if (this.search_result.address) {
            this.routerStore.push(`/explorer/address/${search}`);
            return;
        }
        this.routerStore.push(`/explorer/404/${search}`);
    };

    @action
    updateSearch = (search: string) => {
        this.search = search;
    };

    @action
    updateSearching = (searching: boolean) => this.searching = searching;

    searchBlock = async (id: string) => {
        this.updateQueryLoading(true);
        const res = await this.fetchJson<never, Block>("get", `/api/block/${id}`)

        this.updateBlock(res);
    };

    searchAddress = async (id: string) => {
        this.updateQueryLoading(true);
        const res = await this.fetchJson<never, AddressResult>("get", `/api/address/${id}`)
        this.updateAddress(res);
    };

    @action
    getTransaction = async (id: string) => {
        const tx = await this.fetchJson<never, Transaction>("get", `/api/transaction/${id}`)

        for (let i = 0; i < tx.inputs.length; i++) {
            let inputID = tx.inputs[i] ? tx.inputs[i].referencedOutputID.base58 : GenesisBlockID
            try {
                let referencedOutputRes = await fetch(`/api/output/${inputID}`)
                if (referencedOutputRes.status === 404) {
                    let genOutput = new Output();
                    genOutput.output = new SigLockedSingleOutput();
                    genOutput.output.balance = 0;
                    genOutput.output.address = "LOADED FROM SNAPSHOT";
                    genOutput.type = "SigLockedSingleOutputType";
                    genOutput.outputID = tx.inputs[i].referencedOutputID;
                    tx.inputs[i].output = genOutput;
                }
                if (referencedOutputRes.status === 200) {
                    tx.inputs[i].output = await referencedOutputRes.json()
                }
            } catch (err) {
                // ignore
            }
            this.tx = tx;
        }
    }

    @action
    getTransactionAttachments = async (id: string) => {
        const attachments = await this.fetchJson<never, {transactionID: string, blockIDs: string[]}>("get", `/api/transaction/${id}/attachments`)
        this.txAttachments = attachments;
    }

    @action
    getTransactionMetadata = async (id: string) => {
        const res = await this.fetchJson<never, TransactionMetadata>("get", `/api/transaction/${id}/metadata`)
        this.txMetadata = res;
    }

    @action
    getOutput = async (id: string) => {
        const output = await this.fetchJson<never, Output>("get", `/api/output/${id}`)
        this.output = output;
    }

    @action
    getOutputMetadata = async (id: string) => {
        const res = await this.fetchJson<never, OutputMetadata>("get", `/api/output/${id}/metadata`)
        this.outputMetadata = res;
    }

    @action
    getOutputConsumers = async (id: string) => {
        const res = await this.fetchJson<never, OutputConsumers>("get", `/api/output/${id}/consumers`)
        this.outputConsumers = res;
    }

    @action
    getPendingMana = async (outputID: string) => {
        const res = await this.fetchJson<never, PendingMana>("get", `/api/mana/pending?OutputID=${outputID}`)
        this.pendingMana = res;
    }

    @action
    getConflict = async (id: string) => {
        const res = await this.fetchJson<never, Conflict>("get", `/api/conflict/${id}`)
        this.conflict = res;
    }

    @action
    getConflictChildren = async (id: string) => {
        const res = await this.fetchJson<never, ConflictChildren>("get", `/api/conflict/${id}/children`)
        this.conflictChildren = res;
    }

    @action
    getConflictConflicts = async (id: string) => {
        const res = await this.fetchJson<never, ConflictConflicts>("get", `/api/conflict/${id}/conflicts`)
        this.conflictConflicts = res;
    }

    @action
    getConflictVoters = async (id: string) => {
        const res = await this.fetchJson<never, ConflictVoters>("get", `/api/conflict/${id}/voters`)
        this.conflictVoters = res;
    }

    @action
    getSlotDetails = async (id: string) => {
        const res = await this.fetchJson<never, SlotInfo>("get", `/api/slot/commitment/${id}`)
        this.slotInfo = res;
    }

    @action
    getSlotBlocks = async (index: number) => {
        const res = await this.fetchJson<never, SlotBlocks>("get", `/api/slot/${index}/blocks`)
        this.slotBlocks = res;
    }

    @action
    getSlotTransactions = async (index: number) => {
       const res = await this.fetchJson<never, SlotTransactions>("get", `/api/slot/${index}/transactions`)
       this.slotTransactions = res;
    }

    @action
    getSlotUTXOs = async (index: number) => {
        const res = await this.fetchJson<never, SlotUTXOs>("get", `/api/slot/${index}/utxos`)
        this.slotUtxos = res;
    }

    @action
    getTips = async () => {
        const res = await this.fetchJson<never, Tips>("get", "/api/tips")
        this.tips = res;
    }

    @action
    reset = () => {
        this.blk = null;
        this.query_err = null;
        // reset all variables
        this.tx = null;
        this.txMetadata = null;
        this.txAttachments = [];
        this.output = null;
        this.outputMetadata = null;
        this.outputConsumers = null;
        this.pendingMana = null;
        this.conflict = null;
        this.conflictChildren = null;
        this.conflictConflicts = null;
        this.tips = null;
        this.slotBlocks = new SlotBlocks;
        this.slotInfo = new SlotInfo;
        this.slotTransactions = new SlotTransactions;
        this.slotUtxos = new SlotUTXOs;
    };

    @action
    updateAddress = (addr: AddressResult) => {
        this.addr = addr;
        this.query_err = null;
        this.query_loading = false;
    };

    @action
    updateBlock = (blk: Block) => {
        this.blk = blk;
        this.blk.conflictIDs = this.blk.conflictIDs ? this.blk.conflictIDs : []
        this.blk.addedConflictIDs = this.blk.addedConflictIDs ? this.blk.addedConflictIDs : []
        this.blk.subtractedConflictIDs = this.blk.subtractedConflictIDs ? this.blk.subtractedConflictIDs : []
        this.blk.strongChildren = this.blk.strongChildren ? this.blk.strongChildren : []
        this.blk.weakChildren = this.blk.weakChildren ? this.blk.weakChildren : []
        this.blk.shallowLikeChildren = this.blk.shallowLikeChildren ? this.blk.shallowLikeChildren : []
        this.blk.parentsByType = this.blk.parentsByType ? this.blk.parentsByType : new Map<string, Array<string>>()

        this.query_err = null;
        this.query_loading = false;
        switch (blk.payload_type) {
            case PayloadType.Transaction:
                this.payload = blk.payload as TransactionPayload
                break;
            case PayloadType.Data:
                this.payload = blk.payload as BasicPayload
                break;
            case PayloadType.Faucet:
                this.payload = blk.payload as FaucetPayload
            default:
                this.payload = blk.payload as BasicPayload
                break;
        }
    };

    @action
    updateQueryLoading = (loading: boolean) => this.query_loading = loading;

    @action
    updateQueryError = (err: any) => {
        this.query_err = err;
        this.query_loading = false;
        this.searching = false;
    };

    @action
    addLiveFeedBlock = (blk: BlockRef) => {
        // prevent duplicates (should be fast with only size 10)
        if (this.latest_blocks.findIndex((t) => t.id == blk.id) === -1) {
            if (this.latest_blocks.length >= liveFeedSize) {
                this.latest_blocks.shift();
            }
            this.latest_blocks.push(blk);
        }
    };

    @computed
    get blksLiveFeed() {
        let feed = [];
        for (let i = this.latest_blocks.length - 1; i >= 0; i--) {
            let blk = this.latest_blocks[i];
            feed.push(
                <tr key={blk.id}>
                    <td>
                        <Link to={`/explorer/block/${blk.id}`}>
                            {blk.id}
                        </Link>
                    </td>
                    <td>
                        {getPayloadType(blk.payload_type)}
                    </td>
                </tr>
            );
        }
        return feed;
    }

    @computed
    get tipsList() {
        let list = [];
        if (this.tips) {
            for (let i = 0; i < this.tips.tips.length; i++) {
                let blkId = this.tips.tips[i];
                list.push(
                    <tr key={blkId}>
                        <td>
                            <Link to={`/explorer/block/${blkId}`}>
                                {blkId}
                            </Link>
                        </td>
                    </tr>
                );
            }
        }
        return list;
    }

    async fetchJson<T, U>(
        method: 'get' | 'delete',
        route: string,
        requestData?: T
    ): Promise<U> {

        const body = requestData ? JSON.stringify(requestData, function (_, v) {
            // keep Uint8Array as it is
            if (v instanceof Uint8Array) {
                return Array.from(v);
            }
            return v;
        })
        : undefined;

        const response = await fetch(`${route}`, {
            method,
            headers:{ 'Content-Type': 'application/json' },
            body
        });

        if (response.ok)  {
            const responseData: U = await response.json();
            return responseData;
        }

        switch (response.status) {
            case 404:
                this.updateQueryError(QueryError.NotFound);
                break;
            case 400:
                this.updateQueryError(QueryError.BadRequest);
                break;
            default:
                this.updateQueryError('unexpected error')
                break;
        }
        return {} as U;
    }
}

export default ExplorerStore;
