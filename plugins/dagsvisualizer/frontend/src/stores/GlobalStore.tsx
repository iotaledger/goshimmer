import { action, makeObservable, observable } from 'mobx';
import moment, { Moment } from 'moment';
import TangleStore from './TangleStore';
import { tangleVertex } from 'models/tangle';
import UTXOStore from './UTXOStore';
import { utxoVertex } from 'models/utxo';
import BranchStore from './BranchStore';
import { branchVertex } from 'models/branch';
import { DEFAULT_DASHBOARD_URL } from 'utils/constants';

export class searchResult {
    messages: Array<tangleVertex>;
    txs: Array<utxoVertex>;
    branches: Array<branchVertex>;
    error: string;
}

export class GlobalStore {
    @observable searchStartingTime = moment().unix();
    @observable searchEndingTime = moment().unix();
    @observable explorerAddress = DEFAULT_DASHBOARD_URL;
    @observable searchResponse = '';
    @observable previewResponseSize = '';
    @observable manualPicker = [false, false];
    searchResult: searchResult = undefined;
    searchMode = false;

    tangleStore: TangleStore;
    utxoStore: UTXOStore;
    branchStore: BranchStore;

    constructor(
        tangleStore: TangleStore,
        utxoStore: UTXOStore,
        branchStore: BranchStore
    ) {
        makeObservable(this);

        this.tangleStore = tangleStore;
        this.utxoStore = utxoStore;
        this.branchStore = branchStore;
    }

    @action
    updateStartManualPicker = (b: boolean) => {
        this.manualPicker[0] = b;
    };

    @action
    updateEndManualPicker = (b: boolean) => {
        this.manualPicker[1] = b;
    };

    syncWithMsg = () => {
        const msg = this.tangleStore.selectedMsg;
        if (!msg) return;

        this.utxoStore.clearSelected(true);
        this.utxoStore.clearHighlightedTxs();
        this.branchStore.clearSelected(true);
        this.branchStore.clearHighlightedBranches();

        if (msg.isTx) {
            this.utxoStore.selectTx(msg.txID);
            this.utxoStore.centerTx(msg.txID);
        }
        this.branchStore.highlightBranches(msg.branchIDs);
    };

    syncWithTx = () => {
        const tx = this.utxoStore.selectedTx;
        if (!tx) return;

        // clear previous highlight and selected
        this.tangleStore.clearSelected();
        this.tangleStore.clearHighlightedMsgs();
        this.branchStore.clearSelected(true);

        const msg = this.tangleStore.getTangleVertex(tx.msgID);
        if (msg) {
            this.tangleStore.selectMsg(tx.msgID);
            this.tangleStore.centerMsg(tx.msgID);
        }

        const branch = this.branchStore.getBranchVertex(tx.branchID);
        if (branch) {
            this.branchStore.selectBranch(tx.branchID);
            this.branchStore.centerBranch(tx.branchID);
        }
    };

    syncWithBranch = () => {
        const branch = this.branchStore.selectedBranch;
        if (!branch) return;

        // iterate messages to highlight all messages lies in that branch
        const msgs = this.tangleStore.getMsgsFromBranch(
            branch.ID,
            this.searchMode
        );
        this.tangleStore.clearSelected();
        this.tangleStore.clearHighlightedMsgs();
        this.tangleStore.highlightMsgs(msgs);

        const txs = this.utxoStore.getTxsFromBranch(branch.ID, this.searchMode);
        this.utxoStore.clearSelected(true);
        this.utxoStore.clearHighlightedTxs();
        this.utxoStore.highlightTxs(txs);
    };

    clearSync = () => {
        this.tangleStore.clearSelected();
        this.tangleStore.clearHighlightedMsgs();
        this.utxoStore.clearSelected(true);
        this.utxoStore.clearHighlightedTxs();
        this.branchStore.clearSelected(true);
    };

    get SearchStartingTime() {
        return moment(this.searchStartingTime);
    }

    get SearchEndingTime() {
        return moment(this.searchStartingTime);
    }

    @action
    updateExplorerAddress = (addr: string) => {
        this.explorerAddress = addr;
    };

    @action
    updateSearchStartingTime = (dateTime: Moment) => {
        this.searchStartingTime = dateTime.unix();
    };

    @action
    updateSearchEndingTime = (dateTime: Moment) => {
        this.searchEndingTime = dateTime.unix();
    };

    @action
    updateSearchResponse = (e: string) => {
        this.searchResponse = e;
    };

    @action
    updatePreviewSearchResponse = (msg: string) => {
        this.previewResponseSize = msg;
    };

    updateSearchResults = (results: searchResult) => {
        this.searchResult = results;
    };

    @action
    updatePreviewResponseSize = (response: searchResult) => {
        const numOfBranches = response.branches.length;
        const numOfMessages = response.messages.length;
        const numOfTransactions = response.txs.length;
        this.updatePreviewSearchResponse(`Found: messages: ${numOfMessages};
            transactions: ${numOfTransactions};
            branches: ${numOfBranches};`);
    };

    @action
    searchAndDrawResults = async () => {
        try {
            const res = await fetch(
                `/api/dagsvisualizer/search/${this.searchStartingTime}/${
                    this.searchEndingTime
                }`
            );
            const result: searchResult = await res.json();
            if (res.status !== 200) {
                this.updateSearchResponse(result.error);
                return;
            } else {
                this.updateSearchResponse('To show the results click "Render"');
                this.updatePreviewResponseSize(result);
            }

            if (result.messages.length === 0) {
                this.updateSearchResponse('no messages found!');
                return;
            }
            this.updateSearchResults(result);
        } catch (err) {
            console.log(
                'Fail to fetch messages/txs/branches with the given interval',
                err
            );
        }
        return;
    };

    @action
    renderSearchResults = async () => {
        if (!this.searchResult) {
            return;
        }
        this.searchMode = true;
        this.stopDrawNewVertices();
        this.clearGraphs();

        (this.searchResult.messages || []).forEach((msg) => {
            this.tangleStore.addFoundMsg(msg);
            this.tangleStore.drawVertex(msg);
        });

        (this.searchResult.txs || []).forEach((tx) => {
            this.utxoStore.addFoundTx(tx);
            this.utxoStore.drawFoundVertex(tx);
        });

        const branches = this.searchResult.branches || [];
        for (let i = 0; i < branches.length; i++) {
            this.branchStore.addFoundBranch(branches[i]);
            await this.branchStore.drawVertex(branches[i]);
            this.branchStore.graph.cy
                .getElementById(branches[i].ID)
                .addClass('search');
        }

        this.searchResult = undefined;
        this.updateSearchResponse('');

        return;
    };

    @action
    clearSearchAndResume = () => {
        this.searchMode = false;
        this.clearFoundVertices();
        this.clearGraphs();
        this.clearSelectedVertices();

        // re-draw all existed latest vertices.
        this.tangleStore.drawExistedMsgs();
        this.utxoStore.drawExistedTxs();
        this.branchStore.drawExistedBranches();

        this.drawNewVertices();
        this.updateSearchResponse('');
        this.updatePreviewSearchResponse('');
    };

    drawNewVertices() {
        // resume need redraw all existed vertices
        this.tangleStore.updateDrawStatus(true);
        this.utxoStore.updateDrawStatus(true);
        this.branchStore.updateDrawStatus(true);
    }

    stopDrawNewVertices() {
        this.tangleStore.updateDrawStatus(false);
        this.utxoStore.updateDrawStatus(false);
        this.branchStore.updateDrawStatus(false);
    }

    clearSelectedVertices() {
        this.tangleStore.clearSelected();
        this.utxoStore.clearSelected();
        this.branchStore.clearSelected();
    }

    clearGraphs() {
        this.tangleStore.clearGraph();
        this.branchStore.clearGraph();
        this.utxoStore.clearGraph();
    }

    clearFoundVertices() {
        this.tangleStore.clearFoundMsgs();
        this.utxoStore.clearFoundTxs();
        this.branchStore.clearFoundBranches();
    }
}

export default GlobalStore;
