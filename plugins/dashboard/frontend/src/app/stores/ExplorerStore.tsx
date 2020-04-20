import {action, computed, observable} from 'mobx';
import {registerHandler, WSMsgType} from "app/misc/WS";
import * as React from "react";
import {Link} from 'react-router-dom';
import {RouterStore} from "mobx-react-router";

export class Transaction {
    hash: string;
    signature_message_fragment: string;
    address: string;
    value: number;
    timestamp: number;
    trunk: string;
    branch: string;
    solid: boolean;
}

class AddressResult {
    balance: number;
    txs: Array<Transaction>;
    spent: boolean;
}

class SearchResult {
    tx: Transaction;
    address: AddressResult;
    milestone: Transaction;
    bundles: Array<Array<Transaction>>;
}

class Tx {
    hash: string;
    value: number;
}

const liveFeedSize = 10;

enum QueryError {
    NotFound
}

export class ExplorerStore {
    // live feed
    @observable latest_txs: Array<Tx> = [];

    // queries
    @observable tx: Transaction = null;
    @observable addr: AddressResult = null;

    // loading
    @observable query_loading: boolean = false;
    @observable query_err: any = null;

    // search
    @observable search: string = "";
    @observable search_result: SearchResult = null;
    @observable searching: boolean = false;

    routerStore: RouterStore;

    constructor(routerStore: RouterStore) {
        this.routerStore = routerStore;
        registerHandler(WSMsgType.Tx, this.addLiveFeedTx);
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
        if (this.search_result.tx) {
            this.routerStore.push(`/explorer/tx/${search}`);
            return;
        }
        if (this.search_result.milestone) {
            this.routerStore.push(`/explorer/tx/${this.search_result.milestone.hash}`);
            return;
        }
        if (this.search_result.address) {
            this.routerStore.push(`/explorer/addr/${search}`);
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

    searchTx = async (hash: string) => {
        this.updateQueryLoading(true);
        try {
            let res = await fetch(`/api/tx/${hash}`);
            if (res.status === 404) {
                this.updateQueryError(QueryError.NotFound);
                return;
            }
            let tx: Transaction = await res.json();
            this.updateTx(tx);
        } catch (err) {
            this.updateQueryError(err);
        }
    };

    searchAddress = async (hash: string) => {
        this.updateQueryLoading(true);
        try {
            let res = await fetch(`/api/addr/${hash}`);
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
        this.tx = null;
        this.query_err = null;
    };

    @action
    updateAddress = (addr: AddressResult) => {
        addr.txs = addr.txs.sort((a, b) => {
            return a.timestamp < b.timestamp ? 1 : -1;
        });
        this.addr = addr;
        this.query_err = null;
        this.query_loading = false;
    };

    @action
    updateTx = (tx: Transaction) => {
        this.tx = tx;
        this.query_err = null;
        this.query_loading = false;
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
    addLiveFeedTx = (tx: Tx) => {
        // prevent duplicates (should be fast with only size 10)
        if (this.latest_txs.findIndex((t) => t.hash == tx.hash) === -1) {
            if (this.latest_txs.length >= liveFeedSize) {
                this.latest_txs.shift();
            }
            this.latest_txs.push(tx);
        }
    };

    @computed
    get txsLiveFeed() {
        let feed = [];
        for (let i = this.latest_txs.length - 1; i >= 0; i--) {
            let tx = this.latest_txs[i];
            feed.push(
                <tr key={tx.hash}>
                    <td>
                        <Link to={`/explorer/tx/${tx.hash}`}>
                            {tx.hash.substr(0, 35)}
                        </Link>
                    </td>
                    <td>
                        {tx.value}
                    </td>
                </tr>
            );
        }
        return feed;
    }

}

export default ExplorerStore;