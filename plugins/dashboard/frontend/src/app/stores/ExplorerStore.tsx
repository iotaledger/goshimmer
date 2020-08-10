import {action, computed, observable} from 'mobx';
import {registerHandler, WSMsgType} from "app/misc/WS";
import {BasicPayload, DrngCbPayload, DrngPayload, DrngSubtype, PayloadType, ValuePayload, SyncBeaconPayload} from "app/misc/Payload";
import * as React from "react";
import {Link} from 'react-router-dom';
import {RouterStore} from "mobx-react-router";

export const GenesisMessageID = "1111111111111111111111111111111111111111111111111111111111111111";

export class Message {
    id: string;
    solidification_timestamp: number;
    issuance_timestamp: number;
    sequence_number: number;
    issuer_public_key: string;
    signature: string;
    trunk_message_id: string;
    branch_message_id: string;
    solid: boolean;
    payload_type: number;
    payload: any;
}

class AddressResult {
    address: string;
    output_ids: Array<Output>;
}

class Output {
    id: string;
    balances: Array<Balance>;
    inclusion_state: InclusionState;
    consumer_count: number;
    solidification_time: number;
}

class Balance {
    value: number;
    color: string;
}

class InclusionState {
	liked: boolean;
	rejected: boolean;
	finalized: boolean;
	conflicting: boolean;
	confirmed: boolean;
}

class SearchResult {
    message: MessageRef;
    address: AddressResult;
}

class MessageRef {
    id: string;
}

const liveFeedSize = 50;

enum QueryError {
    NotFound = 1
}

export class ExplorerStore {
    // live feed
    @observable latest_messages: Array<MessageRef> = [];

    // queries
    @observable msg: Message = null;
    @observable addr: AddressResult = null;

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

    @action
    reset = () => {
        this.msg = null;
        this.query_err = null;
    };

    @action
    updateAddress = (addr: AddressResult) => {
        this.addr = addr;
        this.query_err = null;
        this.query_loading = false;
    };

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
            case PayloadType.Value:
                this.payload = msg.payload as ValuePayload
                break;
            case PayloadType.Data:
                this.payload = msg.payload as BasicPayload
                break;
            case PayloadType.SyncBeacon:
                this.payload = msg.payload as SyncBeaconPayload
                // console.log(this.payload.sent_time);
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
                <tr key={msg.id} style={{'display':'block'}}>
                    <td style={{'display':'block'}}>
                        <Link to={`/explorer/message/${msg.id}`}>
                            {msg.id.substr(0, 35)}
                        </Link>
                    </td>
                </tr>
            );
        }
        return feed;
    }

}

export default ExplorerStore;
