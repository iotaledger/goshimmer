import { action, makeObservable, observable, ObservableMap } from 'mobx';
import {connectWebSocket, registerHandler, unregisterHandler, WSMsgType} from '../WS';
import {default as Viva} from 'vivagraphjs';

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
    msgOrder: Array<any> = [];
    selected_origin_color: number = 0;
    highligtedMsgs = new Map<string, number>();
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
            this.graph.addNode(msg.ID, msg);
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
            this.graph.addNode(info.ID, msg);
            this.updateNodeColor(msg);
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
    deleteApproveeLink = (approveeId: string) => {
        if (!approveeId) {
            return;
        }
        let approvee = this.messages.get(approveeId);
        if (approvee) {
            if (this.selectedMsg && approveeId === this.selectedMsg.ID) {
                this.clearSelected();
            }
            this.messages.delete(approveeId);
            if (approvee.isMarker) {
                this.markerMap.delete(approveeId);
            }
        }
        this.graph.removeNode(approveeId);
    }

    @action
    pauseResume = () => {
        if (this.paused) {
            this.renderer.resume();
            this.paused = false;
            return;
        }
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

    updateNodeColor = (msg: tangleVertex) => {
        let nodeUI = this.graphics.getNodeUI(msg.ID);
        let color = "";
        if (!nodeUI || !msg || msg.gof === "GoF(None)") {
            color = "#b58900";
        }
        if (msg.isTx) {
            color = "#fad02c";
        }
        color = "#6c71c4";

        nodeUI.color = parseColor(color);
    }

    removeVertex = (msgID: string) => {
        let vert = this.messages.get(msgID);
        if (vert) {
            this.messages.delete(msgID);
            this.graph.removeNode(msgID);
            
            vert.strongParentIDs.forEach((value) => {
                this.deleteApproveeLink(value)
            })
            vert.weakParentIDs.forEach((value) => {
                this.deleteApproveeLink(value)
            })
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

        let msg = this.messages.get(msgID);
        if (!msg)  return;

        this.updateSelected(msg);
        this.selected_origin_color = this.highlightMsg(msg.ID);

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
        let original_color = nodeUI.color
        nodeUI.color = parseColor("#859900");
        nodeUI.size = vertexSize * 1.5;

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
                linkUI.color = parseColor("#d33682");
            },
            seenForward
        );
        dfsIterator(this.graph, node, node => {
                this.selected_approvees_count++;
            }, false, link => {
                const linkUI = this.graphics.getLinkUI(link.id);
                linkUI.color = parseColor("#b58900");
            },
            seenBackwards
        );
        
        return original_color
    }

    clearHighlightedMsgs = () => {
        this.highligtedMsgs.forEach((color, id) => {
          this.clearHighlightedMsg(id, color);
        })
    }

    clearHighlightedMsg = (msgID: string, originalColor: number) => {
        // clear link highlight
        let node = this.graph.getNode(msgID);
        if (!node) {
            // clear links
            this.resetLinks();
            return;
        }

        let nodeUI = this.graphics.getNodeUI(msgID);
        nodeUI.color = originalColor;
        nodeUI.size = vertexSize;

        const seenForward = [];
        const seenBackwards = [];
        dfsIterator(this.graph, node, node => {
            }, true,
            link => {
                const linkUI = this.graphics.getLinkUI(link.id);
                linkUI.color = parseColor("#586e75");
            },
            seenBackwards
        );
        dfsIterator(this.graph, node, node => {
            }, false,
            link => {
                const linkUI = this.graphics.getLinkUI(link.id);
                linkUI.color = parseColor("#586e75");
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
            linkUI.color = parseColor("#586e75");
        });
    }

    start = () => {
        this.graph = Viva.Graph.graph();

        let graphics: any = Viva.Graph.View.webglGraphics();

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
            return Viva.Graph.View.webglSquare(vertexSize, "#b9b7bd");
        })
        graphics.link(() => Viva.Graph.View.webglLine("#586e75"));
        let ele = document.getElementById('tangleVisualizer');
        this.renderer = Viva.Graph.View.renderer(this.graph, {
            container: ele, graphics, layout,
        });

        let events = Viva.Graph.webglInputEvents(graphics, this.graph);

        events.click((node) => {
            this.selectMsg(node.data.ID);
        });

        this.graphics = graphics;
        this.layout = layout;
        this.renderer.run();
    }

    stop = () => {
        this.unregisterHandlers();
        this.renderer.dispose();
        this.graph = null;
        this.selectedMsg = null;
    }
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

function parseColor(color): any {
    let parsedColor = 0x009ee8ff;

    if (typeof color === 'number') {
        return color;
    }

    if (typeof color === 'string' && color) {
        if (color.length === 4) {
            // #rgb, duplicate each letter except first #.
            color = color.replace(/([^#])/g, '$1$1');
        }
        if (color.length === 9) {
            // #rrggbbaa
            parsedColor = parseInt(color.substr(1), 16);
        } else if (color.length === 7) {
            // or #rrggbb.
            parsedColor = (parseInt(color.substr(1), 16) << 8) | 0xff;
        } else {
            throw 'Color expected in hex format with preceding "#". E.g. #00ff00. Got value: ' + color;
        }
    }

    return parsedColor;
}