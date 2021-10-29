import { action, makeObservable, observable, ObservableMap } from 'mobx';
import {connectWebSocket, registerHandler, unregisterHandler, WSMsgType} from 'WS';
import {default as Viva} from 'vivagraphjs';

export class tangleVertex {
    ID:              string;   
	strongParentIDs: Array<string>;
	weakParentIDs:   Array<string>;
    branchID:        string;
	isMarker:        boolean;
	confirmedTime:   number;
    futureMarkers:   Array<string>;
}

export class tangleBooked {
    ID:       string;
    isMarker: boolean;
	branchID: string;
}

export class tangleFinalized {
    ID:            string;
    confirmedTime: number;
}

export class tangleFutureMarkerUpdated {
    ID:             string;
    futureMarkerID: string;
}

export class tangleMarkerAWUpdated {
    ID:             string;
    approvalWeight: number;
}

const vertexSize = 20;

export class TangleStore {
    @observable maxTangleVertices: number = 100;
    @observable messages = new ObservableMap<string, tangleVertex>();
    @observable markerMap = new ObservableMap<string, Array<string>>();
    @observable awMap = new ObservableMap<string, number>();
    @observable selectedMsg: tangleVertex = null;
    @observable selected_approvers_count = 0;
    @observable selected_approvees_count = 0;
    msgOrder: Array<any> = [];
    selected_via_click: boolean = false;
    selected_origin_color: number = 0;
    vertexChanges = 0;
    graph;
    graphics;
    renderer;

    constructor() {        
        makeObservable(this);
        
        registerHandler(WSMsgType.Message, this.addMessage);
        registerHandler(WSMsgType.MessageBooked, this.setMessageBranch);
        registerHandler(WSMsgType.MessageConfirmed, this.setMessageConfirmedTime);
        registerHandler(WSMsgType.FutureMarkerUpdated, this.updateFutureMarker);
        registerHandler(WSMsgType.MarkerAWUpdated, this.updateMarkerAW);
    }

    unregisterHandlers() {
        unregisterHandler(WSMsgType.Message);
        unregisterHandler(WSMsgType.MessageBooked);
        unregisterHandler(WSMsgType.MessageConfirmed);
        unregisterHandler(WSMsgType.FutureMarkerUpdated);
        unregisterHandler(WSMsgType.MarkerAWUpdated);
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
        this.awMap.set(msg.ID, 0);
        msg.futureMarkers = [];
        this.messages.set(msg.ID, msg);

        this.drawVertex(msg);
    }

    @action
    removeMessage = (msgID: string) => {
        let msg = this.messages.get(msgID);
        if (msg) {
            this.awMap.delete(msgID);
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
        console.log(branch.branchID);
        msg.branchID = branch.branchID;
        msg.isMarker = branch.isMarker;

        this.messages.set(msg.ID, msg);
    }

    @action
    setMessageConfirmedTime = (info: tangleFinalized) => {
        let msg = this.messages.get(info.ID);
        if (!msg) {
            return;
        }

        msg.confirmedTime = info.confirmedTime;
        this.messages.set(msg.ID, msg);
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
    updateMarkerAW = (updatedAW: tangleMarkerAWUpdated) => {
        // update AW of the marker
        this.awMap.set(updatedAW.ID, updatedAW.approvalWeight);

        // iterate the past cone of marker to update AW
        let pastcone = this.markerMap.get(updatedAW.ID);
        if (pastcone) {
            pastcone.forEach((msgID) => {
                let msg = this.messages.get(msgID);
                if (msg) {
                    let aw = 0;
                    msg.futureMarkers.forEach((fm) => {
                        let fmAW = this.awMap.get(fm);
                        if (fmAW) {
                            aw += fmAW;
                        }
                    })
                    this.awMap.set(msgID, aw);
                }
            });
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
        }
        this.graph.removeNode(approveeId);
    }


    drawVertex = (msg: tangleVertex) => {
        let node;
        let existing = this.graph.getNode(msg.ID);
        if (existing) {
            node = existing
        } else {
            node = this.graph.addNode(msg.ID, msg);
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
    updateSelected = (vert: tangleVertex, viaClick?: boolean) => {
        if (!vert) return;

        this.selectedMsg = vert;
        this.selected_via_click = !!viaClick;

        // mutate links
        let node = this.graph.getNode(vert.ID);
        let nodeUI = this.graphics.getNodeUI(vert.ID);
        this.selected_origin_color = nodeUI.color
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
    }

    resetLinks = () => {
        this.graph.forEachLink((link) => {
            const linkUI = this.graphics.getLinkUI(link.id);
            linkUI.color = parseColor("#586e75");
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
        nodeUI.color = this.selected_origin_color;
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

        this.selectedMsg = null;
        this.selected_via_click = false;
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
            return Viva.Graph.View.webglSquare(vertexSize, "#6c71c4");
        })
        graphics.link(() => Viva.Graph.View.webglLine("#586e75"));
        let ele = document.getElementById('tangleVisualizer');
        this.renderer = Viva.Graph.View.renderer(this.graph, {
            container: ele, graphics, layout,
        });

        let events = Viva.Graph.webglInputEvents(graphics, this.graph);

        events.click((node) => {
            this.clearSelected(true);
            this.updateSelected(node.data, true);
        });

        this.graphics = graphics;
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