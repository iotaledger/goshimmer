import {action, computed, observable} from 'mobx';
import {registerHandler, WSMsgType} from "app/misc/WS";
import {
    BasicPayload,
    DrngCbPayload,
    DrngPayload,
    DrngSubtype,
    getPayloadType,
    Output,
    PayloadType,
    SigLockedSingleOutput,
    TransactionPayload
} from "app/misc/Payload";
import * as React from "react";
import {Link} from 'react-router-dom';
import {RouterStore} from "mobx-react-router";

export const GenesisMessageID = "1111111111111111111111111111111111111111111111111111111111111111";
export const GenesisTransactionID = "11111111111111111111111111111111";

export enum GoF {
    None = 0,
    Low,
    Medium,
    High,
}

export class Message {
    id: string;
    solidification_timestamp: number;
    issuance_timestamp: number;
    sequence_number: number;
    issuer_public_key: string;
    issuer_short_id: string;
    signature: string;
    parentsByType: Map<string, Array<string>>;
    strongApprovers: Array<string>;
    weakApprovers: Array<string>;
    shallowLikeApprovers: Array<string>;
    shallowDislikeApprovers: Array<string>;
    solid: boolean;
    branchIDs: Array<string>;
    addedBranchIDs: Array<string>;
    subtractedBranchIDs: Array<string>;
    scheduled: boolean;
    booked: boolean;
    objectivelyInvalid: boolean;
    subjectivelyInvalid: boolean;
    gradeOfFinality: number;
    gradeOfFinalityTime: number;
    payload_type: number;
    payload: any;
    rank: number;
    sequenceID: number;
    isPastMarker: boolean;
    pastMarkerGap: number;
    pastMarkers: string;
    futureMarkers: string;
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
    branchIDs: Array<string>;
    solid: boolean;
    solidificationTime: number;
    consumerCount: number;
    confirmedConsumer: string // tx id of confirmed consumer
    gradeOfFinality: number
    gradeOfFinalityTime: number
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

class Branch {
    id: string;
    parents: Array<string>;
    conflictIDs: Array<string>;
    gradeOfFinality: number
}

class BranchChildren {
    branchID: string;
    childBranches: Array<BranchChild>
}

class BranchChild {
    branchID: string;
    type: string;
}

class BranchConflict {
    outputID: OutputID;
    branchIDs: Array<string>;
}

class BranchConflicts {
    branchID: string;
    conflicts: Array<BranchConflict>
}

class BranchVoters {
    branchID: string;
    voters: Array<string>
}

class SearchResult {
    message: MessageRef;
    address: AddressResult;
}

class MessageRef {
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
    @observable latest_messages: Array<MessageRef> = [];

    // queries
    @observable msg: Message = null;
    @observable addr: AddressResult = null;
    @observable tx: any = null;
    @observable txMetadata: any = null;
    @observable txAttachments: any = [];
    @observable output: any = null;
    @observable outputMetadata: OutputMetadata = null;
    @observable outputConsumers: OutputConsumers = null;
    @observable pendingMana: PendingMana = null;
    @observable branch: Branch = null;
    @observable branchChildren: BranchChildren = null;
    @observable branchConflicts: BranchConflicts = null;
    @observable branchVoters: BranchVoters = null;

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
        registerHandler(WSMsgType.Message, this.addLiveFeedMessage);
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
        if (this.search_result.message) {
            this.routerStore.push(`/explorer/message/${search}`);
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

    searchMessage = async (id: string) => {
        this.updateQueryLoading(true);
        try {
            let res = await fetch(`/api/message/${id}`);
            if (res.status === 404) {
                this.updateQueryError(QueryError.NotFound);
                return;
            }
            let msg: Message = await res.json();
            this.updateMessage(msg);
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
                let inputID = tx.inputs[i] ? tx.inputs[i].referencedOutputID.base58 : GenesisMessageID
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

    getBranch = async (id: string) => {
        try {
            let res = await fetch(`/api/branch/${id}`)
            if (res.status === 404) {
                this.updateQueryError(QueryError.NotFound);
                return;
            }
            if (res.status === 400) {
                this.updateQueryError(QueryError.BadRequest);
                return;
            }
            let branch: Branch = await res.json()
            this.updateBranch(branch)
        } catch (err) {
            this.updateQueryError(err);
        }
    }

    getBranchChildren = async (id: string) => {
        try {
            let res = await fetch(`/api/branch/${id}/children`)
            if (res.status === 404) {
                return;
            }
            let children: BranchChildren = await res.json()
            this.updateBranchChildren(children)
        } catch (err) {
            // ignore
        }
    }

    getBranchConflicts = async (id: string) => {
        try {
            let res = await fetch(`/api/branch/${id}/conflicts`)
            if (res.status === 404) {
                return;
            }
            let conflicts: BranchConflicts = await res.json()
            this.updateBranchConflicts(conflicts)
        } catch (err) {
            // ignore
        }
    }

    getBranchVoters = async (id: string) => {
        try {
            let res = await fetch(`/api/branch/${id}/voters`)
            if (res.status === 404) {
                return;
            }
            let branchVoters: BranchVoters = await res.json()
            this.updateBranchVoters(branchVoters)
        } catch (err) {
            // ignore
        }
    }

    @action
    reset = () => {
        this.msg = null;
        this.query_err = null;
        // reset all variables
        this.tx = null;
        this.txMetadata = null;
        this.txAttachments = [];
        this.output = null;
        this.outputMetadata = null;
        this.outputConsumers = null;
        this.pendingMana = null;
        this.branch = null;
        this.branchChildren = null;
        this.branchConflicts = null;
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
    updateBranch = (branch: Branch) => {
        this.branch = branch;
    }

    @action
    updateBranchChildren = (children: BranchChildren) => {
        this.branchChildren = children;
    }

    @action
    updateBranchConflicts = (conflicts: BranchConflicts) => {
        this.branchConflicts = conflicts;
    }

    @action
    updateBranchVoters = (branchVoters: BranchVoters) => {
        this.branchVoters = branchVoters;
    }

    @action
    updateMessage = (msg: Message) => {
        this.msg = msg;
        this.query_err = null;
        this.query_loading = false;
        switch (msg.payload_type) {
            case PayloadType.Drng:
                this.payload = msg.payload as DrngPayload
                if (this.payload.subpayload_type == DrngSubtype.Cb) {
                    this.subpayload = this.payload.drngpayload as DrngCbPayload
                } else {
                    this.subpayload = this.payload.drngpayload as BasicPayload
                }
                break;
            case PayloadType.Transaction:
                this.payload = msg.payload as TransactionPayload
                break;
            case PayloadType.Data:
                this.payload = msg.payload as BasicPayload
                break;
            case PayloadType.Faucet:
            default:
                this.payload = msg.payload as BasicPayload
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
    addLiveFeedMessage = (msg: MessageRef) => {
        // prevent duplicates (should be fast with only size 10)
        if (this.latest_messages.findIndex((t) => t.id == msg.id) === -1) {
            if (this.latest_messages.length >= liveFeedSize) {
                this.latest_messages.shift();
            }
            this.latest_messages.push(msg);
        }
    };

    @computed
    get msgsLiveFeed() {
        let feed = [];
        for (let i = this.latest_messages.length - 1; i >= 0; i--) {
            let msg = this.latest_messages[i];
            feed.push(
                <tr key={msg.id}>
                    <td>
                        <Link to={`/explorer/message/${msg.id}`}>
                            {msg.id}
                        </Link>
                    </td>
                    <td>
                        {getPayloadType(msg.payload_type)}
                    </td>
                </tr>
            );
        }
        return feed;
    }

}

export default ExplorerStore;
