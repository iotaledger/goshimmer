import {action, makeObservable, observable} from 'mobx';
import moment, {Moment} from 'moment';
import TangleStore from './TangleStore';
import {tangleVertex} from 'models/tangle';
import UTXOStore from './UTXOStore';
import {utxoVertex} from 'models/utxo';
import BranchStore from './BranchStore';
import {branchVertex} from 'models/branch';
import {DEFAULT_DASHBOARD_URL} from 'utils/constants';

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
    @observable manualPicker = [false, false];

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
    }
    @action
    updateEndManualPicker = (b: boolean) => {
        this.manualPicker[1] = b;
    }

    syncWithMsg = () => {
        const msg = this.tangleStore.selectedMsg;
        if (!msg) return;

        if (msg.isTx) {
            this.utxoStore.selectTx(msg.txID);
            this.utxoStore.centerTx(msg.txID);
        }
        this.branchStore.selectBranch(msg.branchID);
    };

    syncWithTx = () => {
        const tx = this.utxoStore.selectedTx;
        if (!tx) return;

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
        const msgs = this.tangleStore.getMsgsFromBranch(branch.ID);
        this.tangleStore.clearSelected();
        this.tangleStore.highlightMsgs(msgs);

        const txs = this.utxoStore.getTxsFromBranch(branch.ID);
        this.utxoStore.clearSelected(true);
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
                this.updateSearchResponse('Done!');
            }

            if (result.messages.length === 0) {
                this.updateSearchResponse('no messages found!');
                return;
            }

            this.stopDrawNewVertices();
            this.clearGraphs();

            result.messages.forEach((msg) => {
                this.tangleStore.drawVertex(msg);
            });

            result.txs.forEach((tx) => {
                this.utxoStore.drawVertex(tx);
            });

            result.branches.forEach((branch) => {
                this.branchStore.drawVertex(branch);
            });
        } catch (err) {
            console.log(
                'Fail to fetch messages/txs/branches with the given interval',
                err
            );
        }
        return;
    };

    @action
    clearSearchAndResume = () => {
        this.clearGraphs();
        this.clearSelectedVertices();

        // re-draw all existed latest vertices.
        this.tangleStore.drawExistedMsgs();
        this.utxoStore.drawExistedTxs();
        this.branchStore.drawExistedBranches();

        this.drawNewVertices();
        this.updateSearchResponse('');
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
}

export default GlobalStore;
