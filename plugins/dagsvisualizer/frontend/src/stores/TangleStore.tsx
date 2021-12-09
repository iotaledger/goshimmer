import {action, makeObservable, observable, ObservableMap} from 'mobx';
import {connectWebSocket, registerHandler, unregisterHandler, WSMsgType} from '../WS';
import {default as Viva} from 'vivagraphjs';
import {COLOR, LINE_TYPE, LINE_WIDTH, VERTEX} from "../styles/tangleStyles";

export class tangleVertex {
    ID:              string;   
	strongParentIDs: Array<string>;
	weakParentIDs:   Array<string>;
    likedParentIDs:  Array<string>;
    branchID:        string;
	isMarker:        boolean;
    isTx:            boolean;
    txID:            string;
    isConfirmed:     boolean;
    gof:             string;
	confirmedTime:   number;
    futureMarkers:   Array<string>;
}

export class tangleBooked {
    ID:       string;
    isMarker: boolean;
	branchID: string;
}

export class tangleConfirmed {
    ID:            string;
    gof:            string;
    confirmedTime: number;
}

export class tangleFutureMarkerUpdated {
    ID:             string;
    futureMarkerID: string;
}

export enum parentRefType {
    StrongRef,
    WeakRef,
    LikedRef,
}

export class TangleStore {
    @observable maxTangleVertices: number = 500;
    @observable messages = new ObservableMap<string, tangleVertex>();
    // might still need markerMap for advanced features
    @observable markerMap = new ObservableMap<string, Array<string>>();
    @observable selectedMsg: tangleVertex = null;
    @observable selected_approvers_count = 0;
    @observable selected_approvees_count = 0;
    @observable paused: boolean = false;
    @observable search: string = "";
    @observable explorerAddress = "http://localhost:8081";
    msgOrder: Array<string> = [];
    lastMsgAddedBeforePause: string = "";
    selected_origin_color: string = "";
    highligtedMsgs = new Map<string, string>();
    draw: boolean = true;
    vertexChanges = 0;
    graph;
    graphics;
    layout;
    renderer;

    constructor() {        
        makeObservable(this);
        
        registerHandler(WSMsgType.Message, this.addMessage);
        registerHandler(WSMsgType.MessageBooked, this.setMessageBranch);
        registerHandler(WSMsgType.MessageConfirmed, this.setMessageConfirmedTime);
        registerHandler(WSMsgType.FutureMarkerUpdated, this.updateFutureMarker);
    }

    unregisterHandlers() {
        unregisterHandler(WSMsgType.Message);
        unregisterHandler(WSMsgType.MessageBooked);
        unregisterHandler(WSMsgType.MessageConfirmed);
        unregisterHandler(WSMsgType.FutureMarkerUpdated);
    }

    connect() {
        connectWebSocket("/ws",
        () => {console.log("connection opened")},
        this.reconnect,
        () => {console.log("connection error")});
    }

    reconnect() {
        setTimeout(() => {
            this.connect();
        }, 1000);
    }

    @action
    addMessage = (msg: tangleVertex) => {
        if (this.msgOrder.length >= this.maxTangleVertices) {
            let removed = this.msgOrder.shift();
            this.removeMessage(removed);
        }

        this.msgOrder.push(msg.ID);
        msg.futureMarkers = [];
        this.messages.set(msg.ID, msg);

        if (this.draw) {
            this.drawVertex(msg);
        }
    }

    @action
    removeMessage = (msgID: string) => {
        let msg = this.messages.get(msgID);
        if (msg) {
            if (msg.isMarker) {
                this.markerMap.delete(msgID);
            }
            this.removeVertex(msgID);
            this.messages.delete(msgID);
        }
    }

    @action
    setMessageBranch = (branch: tangleBooked) => {
        let msg = this.messages.get(branch.ID);
        if (!msg) {
            return;
        }

        msg.branchID = branch.branchID;
        msg.isMarker = branch.isMarker;

        this.messages.set(msg.ID, msg);
        if (this.draw) {
            this.updateIfNotPaused(msg)
        }
    }

    @action
    setMessageConfirmedTime = (info: tangleConfirmed) => {
        let msg = this.messages.get(info.ID);
        if (!msg) {
            return;
        }

        msg.gof = info.gof;
        msg.isConfirmed = true;
        msg.confirmedTime = info.confirmedTime;
        this.messages.set(msg.ID, msg);
        if (this.draw) {
            this.updateIfNotPaused(msg)
        }
    }

    @action
    updateFutureMarker = (fm: tangleFutureMarkerUpdated) => {
        let msg = this.messages.get(fm.ID);
        if (msg) {
            msg.futureMarkers.push(fm.futureMarkerID);
            this.messages.set(fm.ID, msg);
        }

        // update marker map
        let pastconeList = this.markerMap.get(fm.futureMarkerID);
        if (!pastconeList) {
            this.markerMap.set(fm.futureMarkerID, [fm.ID]);
        } else {
            pastconeList.push(fm.ID);
            this.markerMap.set(fm.futureMarkerID, pastconeList);
        }
    }

    @action
    pauseResume = () => {
        if (this.paused) {
            this.renderer.resume();
            this.svgRendererOnResume()
            this.paused = false;
            return;
        }
        this.lastMsgAddedBeforePause = this.msgOrder[this.msgOrder.length - 1]
        this.renderer.pause();
        this.paused = true;
    }

    @action
    updateVerticesLimit = (num: number) => {
        this.maxTangleVertices = num;
    }

    @action
    updateSearch = (search: string) => {
        this.search = search.trim();
    }

    @action
    searchAndSelect = () => {
        if (!this.search) return;

        this.selectMsg(this.search);
    }

    updateExplorerAddress = (addr: string) => {
        this.explorerAddress = addr;
    }

    drawExistedMsgs = () => {
        this.messages.forEach((msg) => {
            this.drawVertex(msg);
        })
    }

    updateDrawStatus = (draw: boolean) => {
        this.draw = draw;
    }

    clearGraph = () => {
        this.graph.clear();
    }

    centerEntireGraph = () => {
        let rect = this.layout.getGraphRect();
        let centerY = (rect.y1 + rect.y2) / 2;
        let centerX = (rect.x1 + rect.x2) / 2;

        this.renderer.moveTo(centerX, centerY);
      }

    drawVertex = (msg: tangleVertex) => {
        let node;
        // For svg renderer, pausing is not gonna stop elements from being added or remover from svg frame
        // when pause we are skipping this function, on resume we manually trigger reloading messages
        if (this.paused) {
            return
        }
        let existing = this.graph.getNode(msg.ID);
        if (existing) {
            node = existing
        } else {
            node = this.graph.addNode(msg.ID, msg);
            this.updateNodeColorOnConfirmation(msg);
        }

        let drawVertexParentReference = (parentType: parentRefType, parentIDs: Array<string>) => {
            if (parentIDs) {
                parentIDs.forEach((value) => {
                    // if value is valid AND (links is empty OR there is no between parent and children)
                    if (value && ((!node.links || !node.links.some(link => link.fromId === value)))) {
                        // draw the link only when the parent exists
                        let existing = this.graph.getNode(value);
                        if (existing) {
                            let link = this.graph.addLink(value, msg.ID);
                            this.updateParentRefUI(link.id, parentType)
                        }
                    }
                })
            }
        }
        drawVertexParentReference(parentRefType.StrongRef, msg.strongParentIDs)
        drawVertexParentReference(parentRefType.WeakRef, msg.weakParentIDs)
        drawVertexParentReference(parentRefType.LikedRef, msg.likedParentIDs)
    }

    // only update color when finalized
    updateNodeColorOnConfirmation = (msg: tangleVertex) => {
        let nodeUI = this.graphics.getNodeUI(msg.ID);
        let color = "";
        if (!msg.isConfirmed) {
            return
        }

        if (msg.isTx) {
            color = COLOR.TRANSACTION_CONFIRMED;
        } else {
            color = COLOR.MESSAGE_CONFIRMED;
        }

        if (!nodeUI || !msg || msg.gof === "GoF(None)") {
            color = COLOR.NODE_UNKNOWN;
        }

        setUINodeColor(nodeUI, color)
    }

    updateParentRefUI = (linkID: string, parentType?: parentRefType) => {
        // update link line type and color based on reference type
        let linkUI = this.graphics.getLinkUI(linkID)
        if (!linkUI) {
            return
        }
        // if type not provided look for refType data if not found use strong ref style
        if (parentType === null) {
            parentType = linkUI.refType || parentRefType.StrongRef
        }

        switch (parentType) {
            case parentRefType.StrongRef: {
                setUILink(linkUI, COLOR.LINK_STRONG, LINE_WIDTH.STRONG, LINE_TYPE.STRONG)
                linkUI.refType = parentRefType.StrongRef
                break;
            }
            case parentRefType.WeakRef: {
                setUILink(linkUI, COLOR.LINK_WEAK, LINE_WIDTH.WEAK, LINE_TYPE.WEAK)
                linkUI.refType = parentRefType.WeakRef
                break;
            }
            case parentRefType.LikedRef: {
                setUILink(linkUI, COLOR.LINK_LIKED, LINE_WIDTH.LIKED, LINE_TYPE.LIKED)
                linkUI.refType = parentRefType.LikedRef
                break;
            }
        }
    }

    removeVertex = (msgID: string) => {
        // svg renderer does not stop elements from being removed from the view while being paused
        // after resume() we update graph state with svgRendererOnResume()
        if (this.paused) {
            return
        } else {
            this.graph.removeNode(msgID);
        }
    }

    @action
    tangleOnClick = (event: any) => {
        // message is currently selected
        if (event.target.tagName === "rect") {
            this.clearSelected()
            this.updateSelected(event.target.node.data)
        } else {
            if (this.selectedMsg !== null) {
                this.clearSelected()
            }
        }
    }

    @action
    updateSelected = (vert: tangleVertex) => {
        if (!vert) return;

        this.selectedMsg = vert;
    }

    selectMsg = (msgID: string) => {
        // clear pre-selected node first
        this.clearSelected();
        let vertex = this.graph.getNode(msgID)
        if (!vertex) return;

        this.updateSelected(vertex.data);
        this.selected_origin_color = this.highlightMsg(vertex.data.ID);

        // center the selected node.
        var pos = this.layout.getNodePosition(msgID);
        this.renderer.moveTo(pos.x, pos.y);
    }

    @action
    clearSelected = () => {
        if (!this.selectedMsg) {
            return;
        }

        this.selected_approvers_count = 0;
        this.selected_approvees_count = 0;
        this.clearHighlightedMsg(this.selectedMsg.ID, this.selected_origin_color);
        this.selectedMsg = null;
    }

    getTangleVertex = (msgID: string) => {
        return this.messages.get(msgID);
    }

    highlightMsgs = (msgIDs: string[]) => {
        this.highligtedMsgs.forEach((color, id) => {
            this.clearHighlightedMsg(id, color);
        })

        // update highlighted msgs and its original color
        msgIDs.forEach((id) => {
            let original_color = this.highlightMsg(id);
            this.highligtedMsgs.set(id, original_color);
        })
    }

    highlightMsg = (msgID: string) => {
        // mutate links
        let node = this.graph.getNode(msgID);
        let nodeUI = this.graphics.getNodeUI(msgID);
        if (!nodeUI) {
            // Message not rendered, so it will not be highlighted
            return
        }
        let original_color = getUINodeColor(nodeUI);

        setUINodeColor(nodeUI, COLOR.NODE_SELECTED)
        setUINodeSize(nodeUI, VERTEX.SIZE_SELECTED);
        setRectBorder(nodeUI, VERTEX.SELECTED_BORDER_WIDTH, COLOR.NODE_BORDER_SELECTED)

        let pos = this.layout.getNodePosition(node.id);
        this.svgUpdateNodePos(nodeUI, pos);

        const seenForward = [];
        const seenBackwards = [];
        dfsIterator(this.graph,
            node,
            node => {
                this.selected_approvers_count++;
            },
            true,
            link => {
                const linkUI = this.graphics.getLinkUI(link.id);
                setUILinkColor(linkUI, COLOR.LINK_FUTURE_CONE)
            },
            seenForward
        );
        dfsIterator(this.graph, node, node => {
                this.selected_approvees_count++;
            }, false, link => {
                const linkUI = this.graphics.getLinkUI(link.id);
                setUILinkColor(linkUI, COLOR.LINK_PAST_CONE)
            },
            seenBackwards
        );
        return original_color
    }

    clearHighlightedMsgs = () => {
        this.highligtedMsgs.forEach((color: string, id) => {
          this.clearHighlightedMsg(id, color);
        })
    }

    clearHighlightedMsg = (msgID: string, originalColor: string) => {
        // clear link highlight
        let node = this.graph.getNode(msgID);
        if (!node) {
            // clear links
            this.resetLinks();
            return;
        }

        let nodeUI = this.graphics.getNodeUI(msgID);
        setUINodeColor(nodeUI, this.selected_origin_color)
        setUINodeSize(nodeUI, VERTEX.SIZE_DEFAULT);
        resetRectBorder(nodeUI)

        const seenForward = [];
        const seenBackwards = [];
        dfsIterator(this.graph, node, node => {
            }, true,
            link => {
                const linkUI = this.graphics.getLinkUI(link.id);
                this.updateParentRefUI(linkUI)
            },
            seenBackwards
        );
        dfsIterator(this.graph, node, node => {
            }, false,
            link => {
                const linkUI = this.graphics.getLinkUI(link.id);
                this.updateParentRefUI(linkUI)
            },
            seenForward
        );
    }

    getMsgsFromBranch = (branchID: string) => {
        let msgs = [];
        this.messages.forEach((msg: tangleVertex) => {
            if (msg.branchID === branchID) {
                msgs.push(msg.ID);
            }
        })

        return msgs;
    }

    resetLinks = () => {
        this.graph.forEachLink((link) => {
            const linkUI = this.graphics.getLinkUI(link.id);
            this.updateParentRefUI(linkUI)
        });
    }

    svgUpdateNodePos(nodeUI, pos) {
        let size = nodeUI.getAttribute('width')
        nodeUI.attr('x', pos.x - size / 2).attr('y', pos.y - size / 2);
    }

    updateNodeDataAndColor(nodeID: string, msgData: tangleVertex) {
        let node = this.graph.getNode(nodeID)
        // replace existing node data
        if (node && msgData) {
            this.graph.addNode(nodeID, msgData)
            this.updateNodeColorOnConfirmation(msgData);
        }
    }

    start = () => {
        this.graph = Viva.Graph.graph();

        this.setupLayout()
        this.setupSvgGraphics()
        this.setupRenderer()

        this.renderer.run();

        maximizeSvgWindow()
        this.registerTangleEvents()
    }

    stop = () => {
        this.unregisterHandlers();
        this.renderer.dispose();
        this.graph = null;
        this.selectedMsg = null;
    }

    setupLayout = () => {
        this.layout = Viva.Graph.Layout.forceDirected(this.graph, {
            springLength: 10,
            springCoeff: 0.0001,
            stableThreshold: 0.15,
            gravity: -2,
            dragCoeff: 0.02,
            timeStep: 20,
            theta: 0.8,
        });
    }

    setupSvgGraphics = () => {
        let graphics: any = Viva.Graph.View.svgGraphics();

        graphics.node((node) => {
            return svgNodeBuilder(node.data);
        }).placeNode(this.svgUpdateNodePos)

        graphics.link(() => {
            return svgLinkBuilder(COLOR.LINK_STRONG, LINE_WIDTH.STRONG, LINE_TYPE.STRONG);
        }).placeLink(function (linkUI, fromPos, toPos) {
            // linkUI - is the object returned from link() callback above.
            let data = 'M' + fromPos.x.toFixed(2) + ',' + fromPos.y.toFixed(2) +
                'L' + toPos.x.toFixed(2) + ',' + toPos.y.toFixed(2);

            // 'Path data' (http://www.w3.org/TR/SVG/paths.html#DAttribute )
            // is a common way of rendering paths in SVG:
            linkUI.attr("d", data);
        })

        this.graphics = graphics;
    }

    setupRenderer = () => {
        let ele = document.getElementById('tangleVisualizer');

        this.renderer = Viva.Graph.View.renderer(this.graph, {
            container: ele,
            graphics: this.graphics,
            layout: this.layout,
        });
    }

    registerTangleEvents = () => {
        let tangleWindowEl = document.querySelector("#tangleVisualizer")
        tangleWindowEl.addEventListener("click", this.tangleOnClick)
    }

    // clear old elements and refresh renderer
    svgRendererOnResume = () => {
        // if pause was long enough for newest added message to be removed then clear all graph at once
        if (!this.messages.get(this.lastMsgAddedBeforePause)) {
            this.clearGraph()
            this.drawExistedMsgs()

            return
        }
        // pause was short - clear only the needed part
        this.graph.forEachNode((node) => {
            let msg = this.messages.get(node.id)
            if (!msg) {
                this.graph.removeNode(node.id)
            } else {
                this.updateNodeDataAndColor(msg.ID, msg)
            }
        })

        for (let msgId in this.messages) {
            let exist = this.graph.getNode(msgId)
            if (!exist) {
                this.drawVertex(this.messages.get(msgId))
            }
        }
    }

    updateIfNotPaused = (msg: tangleVertex) => {
        if (!this.paused) {
            this.updateNodeDataAndColor(msg.ID, msg)
        }
    }
}

let svgNodeBuilder = function (node: tangleVertex): any {
    let color = ""
    if (node.isTx) {
        color = COLOR.TRANSACTION_PENDING
    } else {
        color = COLOR.MESSAGE_PENDING
    }

    let ui = Viva.Graph.svg("rect")
    setUINodeColor(ui, color)
    setUINodeSize(ui, VERTEX.SIZE_DEFAULT)
    setCorners(ui, VERTEX.ROUNDED_CORNER)

    return ui
}

let svgLinkBuilder = function (color: string, width: number, type: string) {
    return Viva.Graph.svg("path")
        .attr("stroke", color)
        .attr("stroke-width", width)
        .attr('stroke-dasharray', type);
}

export default TangleStore;

// copied over and refactored from https://github.com/glumb/IOTAtangle
function dfsIterator(graph, node, cb, up, cbLinks: any = false, seenNodes = []) {
    seenNodes.push(node);
    let pointer = 0;

    while (seenNodes.length > pointer) {
        const node = seenNodes[pointer++];

        if (cb(node)) return true;

        for (const link of node.links) {
            if (cbLinks) cbLinks(link);

            if (!up && link.toId === node.id && !seenNodes.includes(graph.getNode(link.fromId))) {
                seenNodes.push(graph.getNode(link.fromId));
                continue;
            }

            if (up && link.fromId === node.id && !seenNodes.includes(graph.getNode(link.toId))) {
                seenNodes.push(graph.getNode(link.toId));
            }
        }
    }
}

function maximizeSvgWindow() {
    let svgEl = document.querySelector("#tangleVisualizer>svg")
    svgEl.setAttribute("width", "100%")
    svgEl.setAttribute("height", "100%")
}

function setUINodeColor(ui: any, color: any) {
    ui.attr("fill", color);
}

function setUILinkColor(ui: any, color: any) {
    ui.attr("stroke", color);
}

function getUINodeColor(ui: any): string {
    return ui.getAttribute("fill")
}

function setUINodeSize(ui: any, size: number) {
    ui.attr('width', size)
    ui.attr('height', size)
}

function setUILink(ui: any, color: string, width: number, type: string) {
    ui.attr('stroke-width', width)
    ui.attr('stroke-dasharray', type)
    ui.attr('stroke', color)
}

function setCorners(ui: any, rx: number) {
    ui.attr('rx', rx)
}

function setRectBorder(ui: any, borderWidth: number, borderColor) {
    ui.attr('stroke-width', borderWidth)
    ui.attr('stroke', borderColor)
}

function resetRectBorder(ui: any) {
    ui.removeAttribute('stroke-width')
    ui.removeAttribute('stroke')
}
