import { action, makeObservable, observable, ObservableMap } from 'mobx';
import { registerHandler, unregisterHandler, WSMsgType } from 'utils/WS';
import { MAX_VERTICES } from 'utils/constants';
import {
    tangleBooked,
    tangleConfirmed,
    tangleTxGoFChanged,
    tangleFutureMarkerUpdated,
    tangleVertex
} from 'models/tangle';
import {
    drawMessage,
    initTangleDAG,
    reloadAfterShortPause,
    selectMessage,
    unselectMessage,
    updateGraph,
    updateNodeDataAndColor,
    vivagraphLib
} from 'graph/vivagraph';

export class TangleStore {
    @observable maxTangleVertices = MAX_VERTICES;
    @observable messages = new ObservableMap<string, tangleVertex>();
    @observable foundMsgs = new ObservableMap<string, tangleVertex>();
    // might still need markerMap for advanced features
    @observable markerMap = new ObservableMap<string, Array<string>>();
    @observable selectedMsg: tangleVertex = null;
    @observable paused = false;
    @observable search = '';
    msgOrder: Array<string> = [];
    lastMsgAddedBeforePause = '';
    selected_origin_color = '';
    highlightedMsgs = new Map<string, string>();
    draw = true;
    vertexChanges = 0;
    graph;

    constructor() {
        makeObservable(this);

        registerHandler(WSMsgType.Message, this.addMessage);
        registerHandler(WSMsgType.MessageBooked, this.setMessageBranch);
        registerHandler(
            WSMsgType.MessageConfirmed,
            this.setMessageConfirmedTime
        );
        registerHandler(WSMsgType.MessageTxGoFChanged, this.updateMessageTxGoF);
        registerHandler(WSMsgType.FutureMarkerUpdated, this.updateFutureMarker);
    }

    unregisterHandlers() {
        unregisterHandler(WSMsgType.Message);
        unregisterHandler(WSMsgType.MessageBooked);
        unregisterHandler(WSMsgType.MessageConfirmed);
        unregisterHandler(WSMsgType.MessageTxGoFChanged);
        unregisterHandler(WSMsgType.FutureMarkerUpdated);
    }

    @action
    addMessage = (msg: tangleVertex) => {
        this.checkLimit();

        msg.isTip = true;
        msg.futureMarkers = [];
        this.msgOrder.push(msg.ID);
        this.messages.set(msg.ID, msg);

        if (this.draw && !this.paused) {
            this.drawVertex(msg);
        }
    };

    checkLimit = () => {
        if (this.msgOrder.length >= this.maxTangleVertices) {
            const removed = this.msgOrder.shift();
            this.removeMessage(removed);
        }
    };

    @action
    addFoundMsg = (msg: tangleVertex) => {
        this.foundMsgs.set(msg.ID, msg);
    };

    @action
    clearFoundMsgs = () => {
        this.foundMsgs.clear();
    };

    @action
    removeMessage = (msgID: string) => {
        const msg = this.messages.get(msgID);
        if (msg) {
            if (msg.isMarker) {
                this.markerMap.delete(msgID);
            }
            this.removeVertex(msgID);
            this.messages.delete(msgID);
        }
    };

    @action
    setMessageBranch = (branch: tangleBooked) => {
        const msg = this.messages.get(branch.ID);
        if (!msg) {
            return;
        }

        msg.branchIDs = branch.branchIDs;
        msg.isMarker = branch.isMarker;

        this.messages.set(msg.ID, msg);
        if (this.draw) {
            this.updateIfNotPaused(msg);
        }
    };

    @action
    updateMessageTxGoF = (txGoF: tangleTxGoFChanged) => {
        const msg = this.messages.get(txGoF.ID);
        if (!msg) {
            return;
        }

        if (txGoF.isConfirmed) {
            msg.isTxConfirmed = true;
        } else {
            msg.isTxConfirmed = false;
        }

        this.messages.set(msg.ID, msg);
        if (this.draw) {
            this.updateIfNotPaused(msg);
        }
    };

    @action
    setMessageConfirmedTime = (info: tangleConfirmed) => {
        const msg = this.messages.get(info.ID);
        if (!msg) {
            return;
        }

        msg.gof = info.gof;
        msg.isConfirmed = true;
        msg.confirmedTime = info.confirmedTime;
        this.messages.set(msg.ID, msg);
        if (this.draw) {
            this.updateIfNotPaused(msg);
        }
    };

    @action
    updateFutureMarker = (fm: tangleFutureMarkerUpdated) => {
        const msg = this.messages.get(fm.ID);
        if (msg) {
            msg.futureMarkers.push(fm.futureMarkerID);
            this.messages.set(fm.ID, msg);
        }

        // update marker map
        const pastconeList = this.markerMap.get(fm.futureMarkerID);
        if (!pastconeList) {
            this.markerMap.set(fm.futureMarkerID, [fm.ID]);
        } else {
            pastconeList.push(fm.ID);
            this.markerMap.set(fm.futureMarkerID, pastconeList);
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
        this.lastMsgAddedBeforePause = this.msgOrder[this.msgOrder.length - 1];
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

        this.selectMsg(this.search);
        this.centerMsg(this.search);
    };

    drawExistedMsgs = () => {
        this.messages.forEach((msg) => {
            this.drawVertex(msg);
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

    centerMsg = (msgID: string) => {
        this.graph.centerVertex(msgID);
    };

    drawVertex = (msg: tangleVertex) => {
        drawMessage(msg, this.graph, this.messages);
    };

    removeVertex = (msgID: string) => {
        // svg renderer does not stop elements from being removed from the view while being paused
        // after resume() we update graph state with svgRendererOnResume()
        if (this.paused) {
            return;
        } else {
            this.graph.removeVertex(msgID);
        }
        // release svg graphic from rendering
        this.graph.releaseNode(msgID);
    };

    @action
    tangleOnClick = (event: any) => {
        // message is currently selected
        if (event.target.tagName === 'rect') {
            this.selectMsg(event.target.parentNode.node.id);
        } else {
            if (this.selectedMsg !== null) {
                this.clearSelected();
            }
        }
    };

    @action
    updateSelected = (vert: tangleVertex) => {
        if (!vert) return;

        this.selectedMsg = vert;
    };

    selectMsg = (msgID: string) => {
        // clear pre-selected node first
        this.clearSelected();
        this.clearHighlightedMsgs();

        this.selected_origin_color = this.graph.getNodeColor(msgID);
        const node = selectMessage(msgID, this.graph);

        this.updateSelected(node.data);
    };

    @action
    clearSelected = () => {
        if (!this.selectedMsg) {
            return;
        }
        this.clearHighlightedMsg(this.selectedMsg.ID);
        this.selectedMsg = null;
    };

    getTangleVertex = (msgID: string) => {
        return this.messages.get(msgID) || this.foundMsgs.get(msgID);
    };

    highlightMsgs = (msgIDs: string[]) => {
        this.clearHighlightedMsgs();

        // update highlighted msgs and its original color
        msgIDs.forEach((id) => {
            const original_color = this.graph.getNodeColor(id);
            selectMessage(id, this.graph);
            this.highlightedMsgs.set(id, original_color);
        });
    };

    clearHighlightedMsgs = () => {
        if (this.highlightedMsgs.size === 0) {
            return;
        }
        this.highlightedMsgs.forEach((color: string, id) => {
            this.clearHighlightedMsg(id);
        });
        this.highlightedMsgs.clear();
    };

    clearHighlightedMsg = (msgID: string) => {
        let color = '';
        if (this.selectedMsg && msgID === this.selectedMsg.ID) {
            color = this.selected_origin_color;
        } else {
            color = this.highlightedMsgs.get(msgID);
        }

        unselectMessage(msgID, color, this.graph);
    };

    getMsgsFromBranch = (branchID: string, searchedMode: boolean) => {
        const msgs = [];

        if (searchedMode) {
            this.foundMsgs.forEach((msg: tangleVertex) => {
                if (msg.branchIDs.includes(branchID)) {
                    msgs.push(msg.ID);
                }
            });
            return msgs;
        }

        this.messages.forEach((msg: tangleVertex) => {
            if (msg.branchIDs.includes(branchID)) {
                msgs.push(msg.ID);
            }
        });

        return msgs;
    };

    start = () => {
        this.graph = new vivagraphLib(initTangleDAG);

        this.registerTangleEvents();
    };

    stop = () => {
        this.unregisterHandlers();
        this.graph.stop();
        this.selectedMsg = null;
    };

    registerTangleEvents = () => {
        const tangleWindowEl = document.querySelector('#tangleVisualizer');
        tangleWindowEl.addEventListener('click', this.tangleOnClick);
    };

    // For svg renderer, pausing is not going to stop elements from being added or remover from svg frame
    // when pause we are skipping addVertex function, and trigget this function on resume to reload messages
    svgRendererOnResume = () => {
        // if pause was long enough for newest added message to be removed then clear all graph at once
        if (!this.messages.get(this.lastMsgAddedBeforePause)) {
            this.clearGraph();
            this.drawExistedMsgs();
            return;
        }
        reloadAfterShortPause(this.graph, this.messages);

        const idx = this.msgOrder.indexOf(this.lastMsgAddedBeforePause);
        updateGraph(this.graph, this.msgOrder.slice(idx), this.messages);
    };

    updateIfNotPaused = (msg: tangleVertex) => {
        if (!this.paused) {
            updateNodeDataAndColor(msg.ID, msg, this.graph);
        }
    };

    trimTangleToVerticesLimit() {
        if (this.msgOrder.length >= this.maxTangleVertices) {
            const removeStartIndex =
                this.msgOrder.length - this.maxTangleVertices;
            const removed = this.msgOrder.slice(0, removeStartIndex);
            this.msgOrder = this.msgOrder.slice(removeStartIndex);
            this.removeMessages(removed);
        }
    }

    removeMessages(removed: string[]) {
        removed.forEach((msgID: string) => {
            this.removeMessage(msgID);
        });
    }
}

export default TangleStore;
