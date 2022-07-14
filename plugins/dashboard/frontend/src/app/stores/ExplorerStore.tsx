import {action, computed, observable} from 'mobx';
import {registerHandler, WSMsgType} from "app/misc/WS";
import {
    BasicPayload,
    getPayloadType,
    Output,
    PayloadType,
    SigLockedSingleOutput,
    TransactionPayload
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
    objectivelyInvalid: boolean;
    subjectivelyInvalid: boolean;
    confirmationState: number;
    confirmationStateTime: number;
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
    latestConfirmedEpoch: number;
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
    base58:  string;
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

class SearchResult {
    block: BlockRef;
    address: AddressResult;
}

class BlockRef {
    id: string;
    payload_type: number;
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
        try {
            let res = await fetch(`/api/block/${id}`);
            if (res.status === 404) {
                this.updateQueryError(QueryError.NotFound);
                return;
            }
            let blk: Block = await res.json();
            this.updateBlock(blk);
        } catch (err) {
            this.updateQueryError(err);
        }
    };

    searchAddress = async (id: string) => {
        this.updateQueryLoading(true);
        try {
            let res = await fetch(`/api/address/${id}`);
            if (res.status === 404) {
                this.updateQueryError(QueryError.NotFound);
                return;
            }
            let addr: AddressResult = await res.json();
            this.updateAddress(addr);
        } catch (err) {
            this.updateQueryError(err);
        }
    };

    getTransaction = async (id: string) => {
        try {
            let res = await fetch(`/api/transaction/${id}`)
            if (res.status === 404) {
                this.updateQueryError(QueryError.NotFound);
                return;
            }
            let tx = await res.json()
            for(let i = 0; i < tx.inputs.length; i++) {
                let inputID = tx.inputs[i] ? tx.inputs[i].referencedOutputID.base58 : GenesisBlockID
                try{
                    let referencedOutputRes = await fetch(`/api/output/${inputID}`)
                    if (referencedOutputRes.status === 404){
                        let genOutput = new Output();
                        genOutput.output = new SigLockedSingleOutput();
                        genOutput.output.balance = 0;
                        genOutput.output.address = "LOADED FROM SNAPSHOT";
                        genOutput.type = "SigLockedSingleOutputType";
                        genOutput.outputID = tx.inputs[i].referencedOutputID;
                        tx.inputs[i].output = genOutput;
                    }
                    if (referencedOutputRes.status === 200){
                        tx.inputs[i].output = await referencedOutputRes.json()
                    }
                }catch(err){
                    // ignore
                }
            }
            this.updateTransaction(tx)
        } catch (err) {
            this.updateQueryError(err);
        }
    }

    getTransactionAttachments = async (id: string) => {
        try {
            let res = await fetch(`/api/transaction/${id}/attachments`)
            if (res.status === 404) {
                this.updateQueryError(QueryError.NotFound);
                return;
            }
            let attachments = await res.json()
            this.updateTransactionAttachments(attachments)
        } catch (err) {
            this.updateQueryError(err);
        }
    }

    getTransactionMetadata = async (id: string) => {
        try {
            let res = await fetch(`/api/transaction/${id}/metadata`)
            if (res.status === 404) {
                this.updateQueryError(QueryError.NotFound);
                return;
            }
            let metadata = await res.json()
            this.updateTransactionMetadata(metadata)
        } catch (err) {
            this.updateQueryError(err);
        }
    }

    getOutput = async (id: string) => {
        try {
            let res = await fetch(`/api/output/${id}`)
            if (res.status === 404) {
                this.updateQueryError(QueryError.NotFound);
                return;
            }
            if (res.status === 400) {
                this.updateQueryError(QueryError.BadRequest);
                return;
            }
            let output: any = await res.json()
            if (output.error) {
                this.updateQueryError(output.error)
                return
            }
            this.updateOutput(output)
        } catch (err) {
            this.updateQueryError(err);
        }
    }

    getOutputMetadata = async (id: string) => {
        try {
            let res = await fetch(`/api/output/${id}/metadata`)
            if (res.status === 404) {
                return;
            }
            if (res.status === 400) {
                return;
            }
            let metadata: OutputMetadata = await res.json()
            this.updateOutputMetadata(metadata)
        } catch (err) {
            //ignore
        }
    }

    getOutputConsumers = async (id: string) => {
        try {
            let res = await fetch(`/api/output/${id}/consumers`)
            if (res.status === 404) {
                return;
            }
            if (res.status === 400) {
                return;
            }
            let consumers: OutputConsumers = await res.json()
            this.updateOutputConsumers(consumers)
        } catch (err) {
            //ignore
        }
    }

    getPendingMana = async (outputID: string) => {
        try {
            let res = await fetch(`/api/mana/pending?OutputID=${outputID}`)
            if (res.status === 404) {
                return;
            }
            if (res.status === 400) {
                return;
            }
            let pendingMana: PendingMana = await res.json()
            this.updatePendingMana(pendingMana)
        } catch (err) {
            // ignore
        }
    }

    getConflict = async (id: string) => {
        try {
            let res = await fetch(`/api/conflict/${id}`)
            if (res.status === 404) {
                this.updateQueryError(QueryError.NotFound);
                return;
            }
            if (res.status === 400) {
                this.updateQueryError(QueryError.BadRequest);
                return;
            }
            let conflict: Conflict = await res.json()
            this.updateConflict(conflict)
        } catch (err) {
            this.updateQueryError(err);
        }
    }

    getConflictChildren = async (id: string) => {
        try {
            let res = await fetch(`/api/conflict/${id}/children`)
            if (res.status === 404) {
                return;
            }
            let children: ConflictChildren = await res.json()
            this.updateConflictChildren(children)
        } catch (err) {
            // ignore
        }
    }

    getConflictConflicts = async (id: string) => {
        try {
            let res = await fetch(`/api/conflict/${id}/conflicts`)
            if (res.status === 404) {
                return;
            }
            let conflicts: ConflictConflicts = await res.json()
            this.updateConflictConflicts(conflicts)
        } catch (err) {
            // ignore
        }
    }

    getConflictVoters = async (id: string) => {
        try {
            let res = await fetch(`/api/conflict/${id}/voters`)
            if (res.status === 404) {
                return;
            }
            let conflictVoters: ConflictVoters = await res.json()
            this.updateConflictVoters(conflictVoters)
        } catch (err) {
            // ignore
        }
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
    };

    @action
    updateAddress = (addr: AddressResult) => {
        this.addr = addr;
        this.query_err = null;
        this.query_loading = false;
    };

    @action
    updateTransaction = (tx: any) => {
        this.tx = tx;
    }

    @action
    updateTransactionAttachments = (attachments: any) => {
        this.txAttachments = attachments;
    }

    @action
    updateTransactionMetadata = (metadata: any) => {
        this.txMetadata = metadata;
    }

    @action
    updateOutput = (output: any) => {
        this.output = output;
    }

    @action
    updateOutputMetadata = (metadata: OutputMetadata) => {
        this.outputMetadata = metadata;
    }

    @action
    updateOutputConsumers = (consumers: OutputConsumers) => {
        this.outputConsumers = consumers;
    }

    @action
    updatePendingMana = (pendingMana: PendingMana) => {
        this.pendingMana = pendingMana;
    }

    @action
    updateConflict = (conflict: Conflict) => {
        this.conflict = conflict;
    }

    @action
    updateConflictChildren = (children: ConflictChildren) => {
        this.conflictChildren = children;
    }

    @action
    updateConflictConflicts = (conflicts: ConflictConflicts) => {
        this.conflictConflicts = conflicts;
    }

    @action
    updateConflictVoters = (conflictVoters: ConflictVoters) => {
        this.conflictVoters = conflictVoters;
    }

    @action
    updateBlock = (blk: Block) => {
        this.blk = blk;
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

}

export default ExplorerStore;
