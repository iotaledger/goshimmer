import { action, makeObservable, observable, ObservableMap } from 'mobx';
import {
    connectWebSocket,
    registerHandler,
    unregisterHandler,
    WSMsgType
} from 'utils/WS';
import { MAX_VERTICES, DEFAULT_DASHBOARD_URL } from 'utils/constants';
import {
    tangleVertex,
    tangleBooked,
    tangleConfirmed,
    tangleFutureMarkerUpdated
} from 'models/tangle';
import {
    drawMessage,
    initTangleDAG,
    selectMessage,
    unselectMessage,
    vivagraphLib,
    updateGraph,
    updateNodeDataAndColor
} from 'graph/vivagraph';

export class TangleStore {
    @observable maxTangleVertices = MAX_VERTICES;
    @observable messages = new ObservableMap<string, tangleVertex>();
    // might still need markerMap for advanced features
    @observable markerMap = new ObservableMap<string, Array<string>>();
    @observable selectedMsg: tangleVertex = null;
    @observable selected_approvers_count = 0;
    @observable selected_approvees_count = 0;
    @observable paused = false;
    @observable search = '';
    @observable explorerAddress = DEFAULT_DASHBOARD_URL;
    msgOrder: Array<string> = [];
    lastMsgAddedBeforePause = '';
    selected_origin_color = '';
    highligtedMsgs = new Map<string, string>();
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
        registerHandler(WSMsgType.FutureMarkerUpdated, this.updateFutureMarker);
    }

    unregisterHandlers() {
        unregisterHandler(WSMsgType.Message);
        unregisterHandler(WSMsgType.MessageBooked);
        unregisterHandler(WSMsgType.MessageConfirmed);
        unregisterHandler(WSMsgType.FutureMarkerUpdated);
    }

    connect() {
        connectWebSocket(
            '/ws',
            () => {
                console.log('connection opened');
            },
            this.reconnect,
            () => {
                console.log('connection error');
            }
        );
    }

    reconnect() {
        setTimeout(() => {
            this.connect();
        }, 1000);
    }

    @action
    addMessage = (msg: tangleVertex) => {
        if (this.msgOrder.length >= this.maxTangleVertices) {
            const removed = this.msgOrder.shift();
            this.removeMessage(removed);
        }
        msg.isTip = true;

        this.msgOrder.push(msg.ID);
        msg.futureMarkers = [];
        this.messages.set(msg.ID, msg);

        if (this.draw) {
            this.drawVertex(msg);
        }
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

        msg.branchID = branch.branchID;
        msg.isMarker = branch.isMarker;

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
            this.svgRendererOnResume();
            this.paused = false;
            return;
        }
        this.lastMsgAddedBeforePause = this.msgOrder[this.msgOrder.length - 1];
        this.graph.pause();
        this.paused = true;
    };

    @action
    updateVerticesLimit = (num: number) => {
        this.maxTangleVertices = num;
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

    updateExplorerAddress = (addr: string) => {
        this.explorerAddress = addr;
    };

    drawExistedMsgs = () => {
        this.messages.forEach(msg => {
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
    };

    @action
    tangleOnClick = (event: any) => {
        // message is currently selected
        if (event.target.tagName === 'rect') {
            this.selectMsg(event.target.node.id);
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
        this.selected_approvers_count = 0;
        this.selected_approvees_count = 0;
        this.clearHighlightedMsg(this.selectedMsg.ID);
        this.selectedMsg = null;
    };

    getTangleVertex = (msgID: string) => {
        return this.messages.get(msgID);
    };

    highlightMsgs = (msgIDs: string[]) => {
        this.clearHighlightedMsgs();

        // update highlighted msgs and its original color
        msgIDs.forEach(id => {
            const original_color = this.graph.getNodeColor(id);
            selectMessage(id, this.graph);
            this.highligtedMsgs.set(id, original_color);
        });
    };

    clearHighlightedMsgs = () => {
        if (this.highligtedMsgs.size === 0) {
            return;
        }
        this.highligtedMsgs.forEach((color: string, id) => {
            this.clearHighlightedMsg(id);
        });
        this.highligtedMsgs.clear();
    };

    clearHighlightedMsg = (msgID: string) => {
        let color = '';
        if (this.selectedMsg && msgID === this.selectedMsg.ID) {
            color = this.selected_origin_color;
        } else {
            color = this.highligtedMsgs.get(msgID);
        }

        unselectMessage(msgID, color, this.graph);
    };

    getMsgsFromBranch = (branchID: string) => {
        const msgs = [];
        this.messages.forEach((msg: tangleVertex) => {
            if (msg.branchID === branchID) {
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
        this.graph.dispose();
        this.selectedMsg = null;
    };

    registerTangleEvents = () => {
        const tangleWindowEl = document.querySelector('#tangleVisualizer');
        tangleWindowEl.addEventListener('click', this.tangleOnClick);
    };

    // clear old elements and refresh renderer
    svgRendererOnResume = () => {
        // if pause was long enough for newest added message to be removed then clear all graph at once
        if (!this.messages.get(this.lastMsgAddedBeforePause)) {
            this.clearGraph();
            this.drawExistedMsgs();

            return;
        }

        // pause was short - clear only the needed part
        updateGraph(this.graph, this.messages);
    };

    updateIfNotPaused = (msg: tangleVertex) => {
        if (!this.paused) {
            updateNodeDataAndColor(msg.ID, msg, this.graph);
        }
    };
}

export default TangleStore;
