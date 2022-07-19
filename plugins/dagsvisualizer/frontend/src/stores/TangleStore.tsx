import { action, makeObservable, observable, ObservableMap } from 'mobx';
import { registerHandler, unregisterHandler, WSBlkType } from 'utils/WS';
import { MAX_VERTICES } from 'utils/constants';
import {
    tangleBooked,
    tangleConfirmed,
    tangleTxConfirmationStateChanged,
    tangleVertex
} from 'models/tangle';
import {
    drawBlock,
    initTangleDAG,
    reloadAfterShortPause,
    selectBlock,
    unselectBlock,
    updateGraph,
    updateNodeDataAndColor,
    vivagraphLib
} from 'graph/vivagraph';

export class TangleStore {
    @observable maxTangleVertices = MAX_VERTICES;
    @observable blocks = new ObservableMap<string, tangleVertex>();
    @observable foundBlks = new ObservableMap<string, tangleVertex>();
    // might still need markerMap for advanced features
    @observable markerMap = new ObservableMap<string, Array<string>>();
    @observable selectedBlk: tangleVertex = null;
    @observable paused = false;
    @observable search = '';
    blkOrder: Array<string> = [];
    lastBlkAddedBeforePause = '';
    selected_origin_color = '';
    highlightedBlks = new Map<string, string>();
    draw = true;
    vertexChanges = 0;
    graph;

    constructor() {
        makeObservable(this);

        registerHandler(WSBlkType.Block, this.addBlock);
        registerHandler(WSBlkType.BlockBooked, this.setBlockConflict);
        registerHandler(
            WSBlkType.BlockConfirmed,
            this.setBlockConfirmedTime
        );
        registerHandler(WSBlkType.BlockTxConfirmationStateChanged, this.updateBlockTxConfirmationState);
    }

    unregisterHandlers() {
        unregisterHandler(WSBlkType.Block);
        unregisterHandler(WSBlkType.BlockBooked);
        unregisterHandler(WSBlkType.BlockConfirmed);
        unregisterHandler(WSBlkType.BlockTxConfirmationStateChanged);
    }

    @action
    addBlock = (blk: tangleVertex) => {
        this.checkLimit();

        blk.isTip = true;
        this.blkOrder.push(blk.ID);
        this.blocks.set(blk.ID, blk);

        if (this.draw && !this.paused) {
            this.drawVertex(blk);
        }
    };

    checkLimit = () => {
        if (this.blkOrder.length >= this.maxTangleVertices) {
            const removed = this.blkOrder.shift();
            this.removeBlock(removed);
        }
    };

    @action
    addFoundBlk = (blk: tangleVertex) => {
        this.foundBlks.set(blk.ID, blk);
    };

    @action
    clearFoundBlks = () => {
        this.foundBlks.clear();
    };

    @action
    removeBlock = (blkID: string) => {
        const blk = this.blocks.get(blkID);
        if (blk) {
            if (blk.isMarker) {
                this.markerMap.delete(blkID);
            }
            this.removeVertex(blkID);
            this.blocks.delete(blkID);
        }
    };

    @action
    setBlockConflict = (conflict: tangleBooked) => {
        const blk = this.blocks.get(conflict.ID);
        if (!blk) {
            return;
        }

        blk.conflictIDs = conflict.conflictIDs;
        blk.isMarker = conflict.isMarker;

        this.blocks.set(blk.ID, blk);
        if (this.draw) {
            this.updateIfNotPaused(blk);
        }
    };

    @action
    updateBlockTxConfirmationState = (txConfirmationState: tangleTxConfirmationStateChanged) => {
        const blk = this.blocks.get(txConfirmationState.ID);
        if (!blk) {
            return;
        }

        if (txConfirmationState.isConfirmed) {
            blk.isTxConfirmed = true;
        } else {
            blk.isTxConfirmed = false;
        }

        this.blocks.set(blk.ID, blk);
        if (this.draw) {
            this.updateIfNotPaused(blk);
        }
    };

    @action
    setBlockConfirmedTime = (info: tangleConfirmed) => {
        const blk = this.blocks.get(info.ID);
        if (!blk) {
            return;
        }

        blk.confirmationState = info.confirmationState;
        blk.isConfirmed = true;
        blk.confirmationStateTime = info.confirmationStateTime;
        this.blocks.set(blk.ID, blk);
        if (this.draw) {
            this.updateIfNotPaused(blk);
        }
    };

    @action
    pauseResume = () => {
        if (this.paused) {
            this.graph.resume();
            this.paused = false;
            this.svgRendererOnResume();
            return;
        }
        this.lastBlkAddedBeforePause = this.blkOrder[this.blkOrder.length - 1];
        this.graph.pause();
        this.paused = true;
    };

    @action
    updateVerticesLimit = (num: number) => {
        this.maxTangleVertices = num;
        this.trimTangleToVerticesLimit();
    };

    @action
    updateSearch = (search: string) => {
        this.search = search.trim();
    };

    @action
    searchAndSelect = () => {
        if (!this.search) return;

        this.selectBlk(this.search);
        this.centerBlk(this.search);
    };

    drawExistedBlks = () => {
        this.blocks.forEach((blk) => {
            this.drawVertex(blk);
        });
    };

    updateDrawStatus = (draw: boolean) => {
        this.draw = draw;
    };

    clearGraph = () => {
        this.graph.clearGraph();
    };

    centerEntireGraph = () => {
        this.graph.centerGraph();
    };

    centerBlk = (blkID: string) => {
        this.graph.centerVertex(blkID);
    };

    drawVertex = (blk: tangleVertex) => {
        drawBlock(blk, this.graph, this.blocks);
    };

    removeVertex = (blkID: string) => {
        // svg renderer does not stop elements from being removed from the view while being paused
        // after resume() we update graph state with svgRendererOnResume()
        if (this.paused) {
            return;
        } else {
            this.graph.removeVertex(blkID);
        }
        // release svg graphic from rendering
        this.graph.releaseNode(blkID);
    };

    @action
    tangleOnClick = (event: any) => {
        // block is currently selected
        if (event.target.tagName === 'rect') {
            this.selectBlk(event.target.parentNode.node.id);
        } else {
            if (this.selectedBlk !== null) {
                this.clearSelected();
            }
        }
    };

    @action
    updateSelected = (vert: tangleVertex) => {
        if (!vert) return;

        this.selectedBlk = vert;
    };

    selectBlk = (blkID: string) => {
        // clear pre-selected node first
        this.clearSelected();
        this.clearHighlightedBlks();

        this.selected_origin_color = this.graph.getNodeColor(blkID);
        const node = selectBlock(blkID, this.graph);

        this.updateSelected(node.data);
    };

    @action
    clearSelected = () => {
        if (!this.selectedBlk) {
            return;
        }
        this.clearHighlightedBlk(this.selectedBlk.ID);
        this.selectedBlk = null;
    };

    getTangleVertex = (blkID: string) => {
        return this.blocks.get(blkID) || this.foundBlks.get(blkID);
    };

    highlightBlks = (blkIDs: string[]) => {
        this.clearHighlightedBlks();

        // update highlighted blks and its original color
        blkIDs.forEach((id) => {
            const original_color = this.graph.getNodeColor(id);
            selectBlock(id, this.graph);
            this.highlightedBlks.set(id, original_color);
        });
    };

    clearHighlightedBlks = () => {
        if (this.highlightedBlks.size === 0) {
            return;
        }
        this.highlightedBlks.forEach((color: string, id) => {
            this.clearHighlightedBlk(id);
        });
        this.highlightedBlks.clear();
    };

    clearHighlightedBlk = (blkID: string) => {
        let color = '';
        if (this.selectedBlk && blkID === this.selectedBlk.ID) {
            color = this.selected_origin_color;
        } else {
            color = this.highlightedBlks.get(blkID);
        }

        unselectBlock(blkID, color, this.graph);
    };

    getBlksFromConflict = (conflictID: string, searchedMode: boolean) => {
        const blks = [];

        if (searchedMode) {
            this.foundBlks.forEach((blk: tangleVertex) => {
                if (blk.conflictIDs.includes(conflictID)) {
                    blks.push(blk.ID);
                }
            });
            return blks;
        }

        this.blocks.forEach((blk: tangleVertex) => {
            if (blk.conflictIDs.includes(conflictID)) {
                blks.push(blk.ID);
            }
        });

        return blks;
    };

    start = () => {
        this.graph = new vivagraphLib(initTangleDAG);

        this.registerTangleEvents();
    };

    stop = () => {
        this.unregisterHandlers();
        this.graph.stop();
        this.selectedBlk = null;
    };

    registerTangleEvents = () => {
        const tangleWindowEl = document.querySelector('#tangleVisualizer');
        tangleWindowEl.addEventListener('click', this.tangleOnClick);
    };

    // For svg renderer, pausing is not going to stop elements from being added or remover from svg frame
    // when pause we are skipping addVertex function, and trigget this function on resume to reload blocks
    svgRendererOnResume = () => {
        // if pause was long enough for newest added block to be removed then clear all graph at once
        if (!this.blocks.get(this.lastBlkAddedBeforePause)) {
            this.clearGraph();
            this.drawExistedBlks();
            return;
        }
        reloadAfterShortPause(this.graph, this.blocks);

        const idx = this.blkOrder.indexOf(this.lastBlkAddedBeforePause);
        updateGraph(this.graph, this.blkOrder.slice(idx), this.blocks);
    };

    updateIfNotPaused = (blk: tangleVertex) => {
        if (!this.paused) {
            updateNodeDataAndColor(blk.ID, blk, this.graph);
        }
    };

    trimTangleToVerticesLimit() {
        if (this.blkOrder.length >= this.maxTangleVertices) {
            const removeStartIndex =
                this.blkOrder.length - this.maxTangleVertices;
            const removed = this.blkOrder.slice(0, removeStartIndex);
            this.blkOrder = this.blkOrder.slice(removeStartIndex);
            this.removeBlocks(removed);
        }
    }

    removeBlocks(removed: string[]) {
        removed.forEach((blkID: string) => {
            this.removeBlock(blkID);
        });
    }
}

export default TangleStore;
