import {action, makeObservable, observable, ObservableMap} from 'mobx';
import {connectWebSocket, registerHandler, unregisterHandler, WSMsgType} from 'WS';
import {default as Viva} from 'vivagraphjs';
import {COLOR} from "../styles/tangleStyles";

export class tangleVertex {
    ID:              string;   
	strongParentIDs: Array<string>;
	weakParentIDs:   Array<string>;
    likedParentIDs:  Array<string>;
    branchID:        string;
	isMarker:        boolean;
    isTx:            boolean;
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

const vertexSize = 20;

export class TangleStore {
    @observable maxTangleVertices: number = 100;
    @observable messages = new ObservableMap<string, tangleVertex>();
    // might still need markerMap for advanced features
    @observable markerMap = new ObservableMap<string, Array<string>>();
    @observable selectedMsg: tangleVertex = null;
    @observable selected_approvers_count = 0;
    @observable selected_approvees_count = 0;
    @observable paused: boolean = false;
    @observable search: string = "";
    @observable explorerAddress = "localhost:8081";
    msgOrder: Array<string> = [];
    lastMsgAddedBeforePause: string = "";
    selected_via_click: boolean = false;
    selected_origin_color: string = "";
    draw: boolean = true;
    vertexChanges = 0;
    graph;
    graphics;
    renderer;
    layout;

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
        // TODO: improve the updated information
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
    searchAndHighlight = () => {
        this.clearSelected(true);
        if (!this.search) return;

        let msgNode = this.graph.getNode(this.search);
        if (!msgNode) return;

        this.updateSelected(msgNode.data, false);
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
        let graph = document.getElementById('tangleVisualizer');
        let centerY = graph.offsetHeight / 2;
        let centerX = graph.offsetWidth / 2;

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
            this.updateNodeColor(msg);
        }

        if (msg.strongParentIDs) {
            msg.strongParentIDs.forEach((value) => {
                // if value is valid AND (links is empty OR there is no between parent and children)
                if ( value && ((!node.links || !node.links.some(link => link.fromId === value)))){
                    // draw the link only when the parent exists
                    let existing = this.graph.getNode(value);
                    if (existing) {
                        this.graph.addLink(value, msg.ID);
                    }
                }
            })
        }
        if (msg.weakParentIDs){
            msg.weakParentIDs.forEach((value) => {
                // if value is valid AND (links is empty OR there is no between parent and children)
                if ( value && ((!node.links || !node.links.some(link => link.fromId === value)))){
                    // draw the link only when the parent exists
                    let existing = this.graph.getNode(value);
                    if (existing) {
                        this.graph.addLink(value, msg.ID);
                    }
                }
            })
        }
    }

    // TODO: take tangleVertex instead
    // only update color when finalized
    updateNodeColor = (msg: tangleVertex) => {
        let nodeUI = this.graphics.getNodeUI(msg.ID);
        let color = "";
        if (!nodeUI || !msg || msg.gof === "GoF(None)") {
            color = COLOR.NODE_UNKNOWN;
        }
        if (msg.isTx) {
            color = COLOR.TRANSACTION_PENDING;
        }

        setUIColor(nodeUI, color);
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
    updateSelected = (vert: tangleVertex, viaClick?: boolean) => {
        if (!vert) return;

        this.selectedMsg = vert;
        this.selected_via_click = !!viaClick;

        // mutate links
        let node = this.graph.getNode(vert.ID);
        let nodeUI = this.graphics.getNodeUI(vert.ID);
        this.selected_origin_color = getUIColor(nodeUI)
        setUIColor(nodeUI, COLOR.NODE_SELECTED)
        setUINodeSize(nodeUI, vertexSize * 1.5);

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
                setUIColor(linkUI, COLOR.LINK_FUTURE_CONE)
            },
            seenForward
        );
        dfsIterator(this.graph, node, node => {
                this.selected_approvees_count++;
            }, false, link => {
                const linkUI = this.graphics.getLinkUI(link.id);
                setUIColor(linkUI, COLOR.LINK_PAST_CONE)
            },
            seenBackwards
        );
    }

    resetLinks = () => {
        this.graph.forEachLink((link) => {
            const linkUI = this.graphics.getLinkUI(link.id);
            setUIColor(linkUI, COLOR.LINK_STRONG)
        });
    }

    @action
    clearSelected = (force_clear?: boolean) => {
        if (!this.selectedMsg || (this.selected_via_click && !force_clear)) {
            return;
        }

        this.selected_approvers_count = 0;
        this.selected_approvees_count = 0;

        // clear link highlight
        let node = this.graph.getNode(this.selectedMsg.ID);
        if (!node) {
            // clear links
            this.resetLinks();
            return;
        }

        let nodeUI = this.graphics.getNodeUI(this.selectedMsg.ID);
        setUIColor(nodeUI, this.selected_origin_color)
        setUINodeSize(nodeUI, vertexSize);

        const seenForward = [];
        const seenBackwards = [];
        dfsIterator(this.graph, node, node => {
            }, true,
            link => {
                const linkUI = this.graphics.getLinkUI(link.id);
                setUIColor(linkUI, COLOR.LINK_STRONG)
            },
            seenBackwards
        );
        dfsIterator(this.graph, node, node => {
            }, false,
            link => {
                const linkUI = this.graphics.getLinkUI(link.id);
                setUIColor(linkUI, COLOR.LINK_STRONG)
            },
            seenForward
        );

        this.selectedMsg = null;
        this.selected_via_click = false;
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
            this.updateNodeColor(msgData);
        }
    }

    start = () => {
        this.graph = Viva.Graph.graph();

        let graphics: any = Viva.Graph.View.svgGraphics();

        const layout = Viva.Graph.Layout.forceDirected(this.graph, {
            springLength: 10,
            springCoeff: 0.0001,
            stableThreshold: 0.15,
            gravity: -2,
            dragCoeff: 0.02,
            timeStep: 20,
            theta: 0.8,
        });

        graphics.node((node) => {
            let ui = svgNodeBuilder(COLOR.NODE_UNKNOWN, vertexSize, vertexSize);
            ui.on("click", () => {
                this.clearSelected(true)
                this.updateSelected(node.data, true)
            });
            ui.data = node.data;

            return ui
        }).placeNode(this.svgUpdateNodePos)

        graphics.link(() => {
            return svgLinkBuilder(COLOR.LINK_STRONG, 5, ":");
        }).placeLink(function (linkUI, fromPos, toPos) {
            // linkUI - is the object returned from link() callback above.
            let data = 'M' + fromPos.x.toFixed(2) + ',' + fromPos.y.toFixed(2) +
                'L' + toPos.x.toFixed(2) + ',' + toPos.y.toFixed(2);

            // 'Path data' (http://www.w3.org/TR/SVG/paths.html#DAttribute )
            // is a common way of rendering paths in SVG:
            linkUI.attr("d", data);
        })
        let ele = document.getElementById('tangleVisualizer');

        this.renderer = Viva.Graph.View.renderer(this.graph, {
            container: ele,
            graphics: graphics,
            layout: layout,
        });

        this.layout = layout;
        this.graphics = graphics;
        this.renderer.run();

        // maximize the svg window
        let svgEl = document.querySelector("#tangleVisualizer>svg")
        svgEl.setAttribute("width", "100%")
        svgEl.setAttribute("height", "100%")
    }

    stop = () => {
        this.unregisterHandlers();
        this.renderer.dispose();
        this.graph = null;
        this.selectedMsg = null;
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

let svgNodeBuilder = function (color: string, width: number, height: number) {
    return Viva.Graph.svg("rect")
        .attr("width", width)
        .attr("height", height)
        .attr("fill", color)
}

let svgLinkBuilder = function (color: string, width: number, type: string) {
    return Viva.Graph.svg("path")
        .attr("stroke", color)
        .attr("width", width)
        .attr('stroke-dasharray', '5, 5');
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


function setUIColor(ui: any, color: any) {
    ui.attr("fill", color);
}

function getUIColor(ui: any): string {
    return ui.getAttribute("fill")
}

function setUINodeSize(ui: any, size: number) {
    ui.attr('width', size)
    ui.attr('height', size)
}
