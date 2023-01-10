import {action, makeObservable, observable} from 'mobx';
import moment, {Moment} from 'moment';
import TangleStore from './TangleStore';
import {tangleVertex} from 'models/tangle';
import UTXOStore from './UTXOStore';
import {utxoVertex} from 'models/utxo';
import ConflictStore from './ConflictStore';
import {conflictVertex} from 'models/conflict';
import {DEFAULT_DASHBOARD_URL} from 'utils/constants';

export class searchResult {
    blocks: Array<tangleVertex>;
    txs: Array<utxoVertex>;
    conflicts: Array<conflictVertex>;
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
    conflictStore: ConflictStore;

    constructor(
        tangleStore: TangleStore,
        utxoStore: UTXOStore,
        conflictStore: ConflictStore
    ) {
        makeObservable(this);

        this.tangleStore = tangleStore;
        this.utxoStore = utxoStore;
        this.conflictStore = conflictStore;
    }

    @action
    updateStartManualPicker = (b: boolean) => {
        this.manualPicker[0] = b;
    };

    @action
    updateEndManualPicker = (b: boolean) => {
        this.manualPicker[1] = b;
    };

    syncWithBlk = () => {
        const blk = this.tangleStore.selectedBlk;
        if (!blk) return;

        this.utxoStore.clearSelected(true);
        this.utxoStore.clearHighlightedTxs();
        this.conflictStore.clearSelected(true);
        this.conflictStore.clearHighlightedConflicts();

        if (blk.isTx) {
            this.utxoStore.selectTx(blk.txID);
            this.utxoStore.centerTx(blk.txID);
        }
        this.conflictStore.highlightConflicts(blk.conflictIDs);
    };

    syncWithTx = () => {
        const tx = this.utxoStore.selectedTx;
        if (!tx) return;

        // clear previous highlight and selected
        this.tangleStore.clearSelected();
        this.tangleStore.clearHighlightedBlks();
        this.conflictStore.clearSelected(true);

        const blk = this.tangleStore.getTangleVertex(tx.blkID);
        if (blk) {
            this.tangleStore.selectBlk(tx.blkID);
            this.tangleStore.centerBlk(tx.blkID);
        }

        const conflict = this.conflictStore.getConflictVertex(tx.conflictID);
        if (conflict) {
            this.conflictStore.selectConflict(tx.conflictID);
            this.conflictStore.centerConflict(tx.conflictID);
        }
    };

    syncWithConflict = () => {
        const conflict = this.conflictStore.selectedConflict;
        if (!conflict) return;

        // iterate blocks to highlight all blocks lies in that conflict
        const blks = this.tangleStore.getBlksFromConflict(
            conflict.ID,
            this.searchMode
        );
        this.tangleStore.clearSelected();
        this.tangleStore.clearHighlightedBlks();
        this.tangleStore.highlightBlks(blks);

        const txs = this.utxoStore.getTxsFromConflict(conflict.ID, this.searchMode);
        this.utxoStore.clearSelected(true);
        this.utxoStore.clearHighlightedTxs();
        this.utxoStore.highlightTxs(txs);
    };

    clearSync = () => {
        this.tangleStore.clearSelected();
        this.tangleStore.clearHighlightedBlks();
        this.utxoStore.clearSelected(true);
        this.utxoStore.clearHighlightedTxs();
        this.conflictStore.clearSelected(true);
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
    updatePreviewSearchResponse = (blk: string) => {
        this.previewResponseSize = blk;
    };

    updateSearchResults = (results: searchResult) => {
        this.searchResult = results;
    };

    @action
    updatePreviewResponseSize = (response: searchResult) => {
        const numOfConflicts = !response.conflicts ? 0 : response.conflicts.length;
        const numOfBlocks = !response.blocks ? 0 : response.blocks.length;
        const numOfTransactions = !response.txs ? 0 : response.txs.length;
        this.updatePreviewSearchResponse(`Found: blocks: ${numOfBlocks};
            transactions: ${numOfTransactions};
            conflicts: ${numOfConflicts};`);
    };

    @action
    searchAndDrawResults = async () => {
        try {
            const res = await fetch(
                `/api/dagsvisualizer/search/${this.searchStartingTime}/${this.searchEndingTime}`
            );
            const result: searchResult = await res.json();
            if (res.status !== 200) {
                this.updateSearchResponse(result.error);
                return;
            } else {
                this.updateSearchResponse('To show the results click "Render"');
                this.updatePreviewResponseSize(result);
            }

            if ((!result.blocks ? 0 : result.blocks.length) === 0) {
                this.updateSearchResponse('no blocks found!');
                return;
            }
            this.updateSearchResults(result);
        } catch (err) {
            console.log(
                'Fail to fetch blocks/txs/conflicts with the given interval',
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

        (this.searchResult.blocks || []).forEach((blk) => {
            this.tangleStore.addFoundBlk(blk);
            this.tangleStore.drawVertex(blk);
        });

        (this.searchResult.txs || []).forEach((tx) => {
            this.utxoStore.addFoundTx(tx);
            this.utxoStore.drawFoundVertex(tx);
        });

        const conflicts = this.searchResult.conflicts || [];
        for (let i = 0; i < conflicts.length; i++) {
            this.conflictStore.addFoundConflict(conflicts[i]);
            await this.conflictStore.drawVertex(conflicts[i]);
            this.conflictStore.graph.cy
                .getElementById(conflicts[i].ID)
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
        this.tangleStore.drawExistedBlks();
        this.utxoStore.drawExistedTxs();
        this.conflictStore.drawExistedConflicts();

        this.drawNewVertices();
        this.updateSearchResponse('');
        this.updatePreviewSearchResponse('');
    };

    drawNewVertices() {
        // resume need redraw all existed vertices
        this.tangleStore.updateDrawStatus(true);
        this.utxoStore.updateDrawStatus(true);
        this.conflictStore.updateDrawStatus(true);
    }

    stopDrawNewVertices() {
        this.tangleStore.updateDrawStatus(false);
        this.utxoStore.updateDrawStatus(false);
        this.conflictStore.updateDrawStatus(false);
    }

    clearSelectedVertices() {
        this.tangleStore.clearSelected();
        this.utxoStore.clearSelected();
        this.conflictStore.clearSelected();
    }

    clearGraphs() {
        this.tangleStore.clearGraph();
        this.conflictStore.clearGraph();
        this.utxoStore.clearGraph();
    }

    clearFoundVertices() {
        this.tangleStore.clearFoundBlks();
        this.utxoStore.clearFoundTxs();
        this.conflictStore.clearFoundConflicts();
    }
}

export default GlobalStore;
