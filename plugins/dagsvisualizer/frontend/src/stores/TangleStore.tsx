import { action, makeObservable, observable, ObservableMap } from 'mobx';
import {
    connectWebSocket,
    registerHandler,
    unregisterHandler,
    WSMsgType
} from 'utils/WS';
import { MAX_VERTICES, DEFAULT_DASHBOARD_URL } from 'utils/constants';
import { default as Viva } from 'vivagraphjs';
import { COLOR, LINE_TYPE, LINE_WIDTH, VERTEX } from 'styles/tangleStyles';

export class tangleVertex {
    ID: string;
    strongParentIDs: Array<string>;
    weakParentIDs: Array<string>;
    likedParentIDs: Array<string>;
    branchID: string;
    isMarker: boolean;
    isTx: boolean;
    txID: string;
    isTip: boolean;
    isConfirmed: boolean;
    gof: string;
    confirmedTime: number;
    futureMarkers: Array<string>;
}

export class tangleBooked {
    ID: string;
    isMarker: boolean;
    branchID: string;
}

export class tangleConfirmed {
    ID: string;
    gof: string;
    confirmedTime: number;
}

export class tangleFutureMarkerUpdated {
    ID: string;
    futureMarkerID: string;
}

export enum parentRefType {
    StrongRef,
    WeakRef,
    LikedRef
}

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
    graphics;
    layout;
    renderer;

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
            this.renderer.resume();
            this.svgRendererOnResume();
            this.paused = false;
            return;
        }
        this.lastMsgAddedBeforePause = this.msgOrder[this.msgOrder.length - 1];
        this.renderer.pause();
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
        this.graph.clear();
    };

    centerEntireGraph = () => {
        const rect = this.layout.getGraphRect();
        const centerY = (rect.y1 + rect.y2) / 2;
        const centerX = (rect.x1 + rect.x2) / 2;

        this.renderer.moveTo(centerX, centerY);
    };

    centerMsg = (msgID: string) => {
        const pos = this.layout.getNodePosition(msgID);
        this.renderer.moveTo(pos.x, pos.y);
    };

    drawVertex = (msg: tangleVertex) => {
        let node;
        // For svg renderer, pausing is not gonna stop elements from being added or remover from svg frame
        // when pause we are skipping this function, on resume we manually trigger reloading messages
        if (this.paused) {
            return;
        }
        const existing = this.graph.getNode(msg.ID);
        if (existing) {
            node = existing;
        } else {
            node = this.graph.addNode(msg.ID, msg);
            this.updateNodeColorOnConfirmation(msg);
        }

        const drawVertexParentReference = (
            parentType: parentRefType,
            parentIDs: Array<string>
        ) => {
            if (parentIDs) {
                parentIDs.forEach(value => {
                    // remove tip status
                    const parent = this.messages.get(value);
                    if (parent) {
                        parent.isTip = false;
                        this.updateNodeColorOnConfirmation(parent);
                    }

                    // if value is valid AND (links is empty OR there is no between parent and children)
                    if (
                        value &&
                        (!node.links ||
                            !node.links.some(link => link.fromId === value))
                    ) {
                        // draw the link only when the parent exists
                        const existing = this.graph.getNode(value);
                        if (existing) {
                            const link = this.graph.addLink(value, msg.ID);
                            this.updateParentRefUI(link.id, parentType);
                        }
                    }
                });
            }
        };
        drawVertexParentReference(parentRefType.StrongRef, msg.strongParentIDs);
        drawVertexParentReference(parentRefType.WeakRef, msg.weakParentIDs);
        drawVertexParentReference(parentRefType.LikedRef, msg.likedParentIDs);
    };

    // only update color when finalized
    updateNodeColorOnConfirmation = (msg: tangleVertex) => {
        const nodeUI = this.graphics.getNodeUI(msg.ID);
        if (!nodeUI) return;
        if (msg.isTip) return;

        let color = '';
        color = msg.isTx ? COLOR.TRANSACTION_PENDING : COLOR.MESSAGE_PENDING;
        if (msg.isConfirmed) {
            color = msg.isTx
                ? COLOR.TRANSACTION_CONFIRMED
                : COLOR.MESSAGE_CONFIRMED;
        }

        setUINodeColor(nodeUI, color);
    };

    updateParentRefUI = (linkID: string, parentType?: parentRefType) => {
        // update link line type and color based on reference type
        const linkUI = this.graphics.getLinkUI(linkID);
        if (!linkUI) {
            return;
        }
        // if type not provided look for refType data if not found use strong ref style
        if (parentType === undefined) {
            parentType = linkUI.refType || parentRefType.StrongRef;
        }

        switch (parentType) {
        case parentRefType.StrongRef: {
            setUILink(
                linkUI,
                COLOR.LINK_STRONG,
                LINE_WIDTH.STRONG,
                LINE_TYPE.STRONG
            );
            linkUI.refType = parentRefType.StrongRef;
            break;
        }
        case parentRefType.WeakRef: {
            setUILink(
                linkUI,
                COLOR.LINK_WEAK,
                LINE_WIDTH.WEAK,
                LINE_TYPE.WEAK
            );
            linkUI.refType = parentRefType.WeakRef;
            break;
        }
        case parentRefType.LikedRef: {
            setUILink(
                linkUI,
                COLOR.LINK_LIKED,
                LINE_WIDTH.LIKED,
                LINE_TYPE.LIKED
            );
            linkUI.refType = parentRefType.LikedRef;
            break;
        }
        }
    };

    removeVertex = (msgID: string) => {
        // svg renderer does not stop elements from being removed from the view while being paused
        // after resume() we update graph state with svgRendererOnResume()
        if (this.paused) {
            return;
        } else {
            this.graph.removeNode(msgID);
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

        const vertex = this.graph.getNode(msgID);
        if (!vertex) return;

        this.updateSelected(vertex.data);
        this.selected_origin_color = this.highlightMsg(vertex.data.ID);
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
            const original_color = this.highlightMsg(id);
            this.highligtedMsgs.set(id, original_color);
        });
    };

    highlightMsg = (msgID: string) => {
        // mutate links
        const node = this.graph.getNode(msgID);
        const nodeUI = this.graphics.getNodeUI(msgID);
        if (!nodeUI) {
            // Message not rendered, so it will not be highlighted
            return;
        }
        const original_color = getUINodeColor(nodeUI);

        setUINodeColor(nodeUI, COLOR.NODE_SELECTED);
        setUINodeSize(nodeUI, VERTEX.SIZE_SELECTED);
        setRectBorder(
            nodeUI,
            VERTEX.SELECTED_BORDER_WIDTH,
            COLOR.NODE_BORDER_SELECTED
        );

        const pos = this.layout.getNodePosition(node.id);
        this.svgUpdateNodePos(nodeUI, pos);

        const seenForward = [];
        const seenBackwards = [];
        dfsIterator(
            this.graph,
            node,
            () => {
                this.selected_approvers_count++;
            },
            true,
            link => {
                const linkUI = this.graphics.getLinkUI(link.id);
                setUILinkColor(linkUI, COLOR.LINK_FUTURE_CONE);
            },
            seenForward
        );
        dfsIterator(
            this.graph,
            node,
            () => {
                this.selected_approvees_count++;
            },
            false,
            link => {
                const linkUI = this.graphics.getLinkUI(link.id);
                setUILinkColor(linkUI, COLOR.LINK_PAST_CONE);
            },
            seenBackwards
        );
        return original_color;
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
        // clear link highlight
        const node = this.graph.getNode(msgID);
        if (!node) {
            // clear links
            this.resetLinks();
            return;
        }

        let color = '';
        if (this.selectedMsg && msgID === this.selectedMsg.ID) {
            color = this.selected_origin_color;
        } else {
            color = this.highligtedMsgs.get(msgID);
        }

        const nodeUI = this.graphics.getNodeUI(msgID);
        setUINodeColor(nodeUI, color);
        setUINodeSize(nodeUI, VERTEX.SIZE_DEFAULT);
        resetRectBorder(nodeUI);

        const seenForward = [];
        const seenBackwards = [];
        dfsIterator(
            this.graph,
            node,
            () => {
                return false;
            },
            true,
            link => {
                this.updateParentRefUI(link.id);
            },
            seenBackwards
        );
        dfsIterator(
            this.graph,
            node,
            () => {
                return false;
            },
            false,
            link => {
                this.updateParentRefUI(link.id);
            },
            seenForward
        );
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

    resetLinks = () => {
        this.graph.forEachLink(link => {
            this.updateParentRefUI(link.id);
        });
    };

    svgUpdateNodePos(nodeUI, pos) {
        const size = nodeUI.getAttribute('width');
        nodeUI.attr('x', pos.x - size / 2).attr('y', pos.y - size / 2);
    }

    updateNodeDataAndColor(nodeID: string, msgData: tangleVertex) {
        const node = this.graph.getNode(nodeID);
        // replace existing node data
        if (node && msgData) {
            this.graph.addNode(nodeID, msgData);
            this.updateNodeColorOnConfirmation(msgData);
        }
    }

    start = () => {
        this.graph = Viva.Graph.graph();

        this.setupLayout();
        this.setupSvgGraphics();
        this.setupRenderer();

        this.renderer.run();

        maximizeSvgWindow();
        this.registerTangleEvents();
    };

    stop = () => {
        this.unregisterHandlers();
        this.renderer.dispose();
        this.graph = null;
        this.selectedMsg = null;
    };

    setupLayout = () => {
        this.layout = Viva.Graph.Layout.forceDirected(this.graph, {
            springLength: 10,
            springCoeff: 0.0001,
            stableThreshold: 0.15,
            gravity: -2,
            dragCoeff: 0.02,
            timeStep: 20,
            theta: 0.8
        });
    };

    setupSvgGraphics = () => {
        const graphics: any = Viva.Graph.View.svgGraphics();

        graphics
            .node(() => {
                return svgNodeBuilder();
            })
            .placeNode(this.svgUpdateNodePos);

        graphics
            .link(() => {
                return svgLinkBuilder(
                    COLOR.LINK_STRONG,
                    LINE_WIDTH.STRONG,
                    LINE_TYPE.STRONG
                );
            })
            .placeLink(function(linkUI, fromPos, toPos) {
                // linkUI - is the object returned from link() callback above.
                const data =
                    'M' +
                    fromPos.x.toFixed(2) +
                    ',' +
                    fromPos.y.toFixed(2) +
                    'L' +
                    toPos.x.toFixed(2) +
                    ',' +
                    toPos.y.toFixed(2);

                // 'Path data' (http://www.w3.org/TR/SVG/paths.html#DAttribute )
                // is a common way of rendering paths in SVG:
                linkUI.attr('d', data);
            });

        this.graphics = graphics;
    };

    setupRenderer = () => {
        const ele = document.getElementById('tangleVisualizer');

        this.renderer = Viva.Graph.View.renderer(this.graph, {
            container: ele,
            graphics: this.graphics,
            layout: this.layout
        });
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
        this.graph.forEachNode(node => {
            const msg = this.messages.get(node.id);
            if (!msg) {
                this.graph.removeNode(node.id);
            } else {
                this.updateNodeDataAndColor(msg.ID, msg);
            }
        });

        for (const msgId in this.messages) {
            const exist = this.graphics.getNodeUI(msgId);
            if (!exist) {
                this.drawVertex(this.messages.get(msgId));
            }
        }
    };

    updateIfNotPaused = (msg: tangleVertex) => {
        if (!this.paused) {
            this.updateNodeDataAndColor(msg.ID, msg);
        }
    };
}

const svgNodeBuilder = function(): any {
    const ui = Viva.Graph.svg('rect');
    setUINodeColor(ui, COLOR.TIP);
    setUINodeSize(ui, VERTEX.SIZE_DEFAULT);
    setCorners(ui, VERTEX.ROUNDED_CORNER);

    return ui;
};

const svgLinkBuilder = function(color: string, width: number, type: string) {
    return Viva.Graph.svg('path')
        .attr('stroke', color)
        .attr('stroke-width', width)
        .attr('stroke-dasharray', type);
};

export default TangleStore;

// copied over and refactored from https://github.com/glumb/IOTAtangle
function dfsIterator(
    graph,
    node,
    cb,
    up,
    cbLinks: any = false,
    seenNodes = []
) {
    seenNodes.push(node);
    let pointer = 0;

    while (seenNodes.length > pointer) {
        const node = seenNodes[pointer++];

        if (cb(node)) return true;

        for (const link of node.links) {
            if (cbLinks) cbLinks(link);

            if (
                !up &&
                link.toId === node.id &&
                !seenNodes.includes(graph.getNode(link.fromId))
            ) {
                seenNodes.push(graph.getNode(link.fromId));
                continue;
            }

            if (
                up &&
                link.fromId === node.id &&
                !seenNodes.includes(graph.getNode(link.toId))
            ) {
                seenNodes.push(graph.getNode(link.toId));
            }
        }
    }
}

function maximizeSvgWindow() {
    const svgEl = document.querySelector('#tangleVisualizer>svg');
    svgEl.setAttribute('width', '100%');
    svgEl.setAttribute('height', '100%');
}

function setUINodeColor(ui: any, color: any) {
    ui.attr('fill', color);
}

function setUILinkColor(ui: any, color: any) {
    ui.attr('stroke', color);
}

function getUINodeColor(ui: any): string {
    return ui.getAttribute('fill');
}

function setUINodeSize(ui: any, size: number) {
    ui.attr('width', size);
    ui.attr('height', size);
}

function setUILink(ui: any, color: string, width: number, type: string) {
    ui.attr('stroke-width', width);
    ui.attr('stroke-dasharray', type);
    ui.attr('stroke', color);
}

function setCorners(ui: any, rx: number) {
    ui.attr('rx', rx);
}

function setRectBorder(ui: any, borderWidth: number, borderColor) {
    ui.attr('stroke-width', borderWidth);
    ui.attr('stroke', borderColor);
}

function resetRectBorder(ui: any) {
    ui.removeAttribute('stroke-width');
    ui.removeAttribute('stroke');
}
