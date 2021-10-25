import { action, observable, ObservableMap } from 'mobx';
import {connectWebSocket, registerHandler, unregisterHandler, WSMsgType} from 'WS';
import cytoscape from 'cytoscape';
import fcose from 'cytoscape-fcose';
import { fcoseOptions } from 'styles/graphStyle';
import layoutUtilities from 'cytoscape-layout-utilities';

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

export class TangleStore {
    @observable maxTangleVertices: number = 100;
    @observable messages = new ObservableMap<string, tangleVertex>();
    @observable markerMap = new ObservableMap<string, Array<string>>();
    @observable awMap = new ObservableMap<string, number>();
    @observable selectedMsg: tangleVertex;
    msgOrder: Array<any> = [];
    newVertexCounter = 0;
    cy;
    layout;
    layoutApi;

    constructor() {        
        registerHandler(WSMsgType.Message, this.addMessage);
        registerHandler(WSMsgType.MessageBooked, this.setMessageBranch);
        registerHandler(WSMsgType.MessageConfirmed, this.setMessageConfirmedTime);
        registerHandler(WSMsgType.FutureMarkerUpdated, this.updateFutureMarker);
        registerHandler(WSMsgType.MarkerAWUpdated, this.updateMarkerAW);

        cytoscape.use(fcose);
        cytoscape.use( layoutUtilities );
    }

    unregisterHandlers() {
        unregisterHandler(WSMsgType.Message);
        unregisterHandler(WSMsgType.MessageBooked);
        unregisterHandler(WSMsgType.MessageConfirmed);
        unregisterHandler(WSMsgType.FutureMarkerUpdated);
        unregisterHandler(WSMsgType.MarkerAWUpdated);
    }

    connect() {
        connectWebSocket("localhost:8061/ws",
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

        //this.drawVertex(msg);
    }

    @action
    removeMessage = (msgID: string) => {
        let msg = this.messages.get(msgID);
        if (msg) {
            this.awMap.delete(msgID);
            if (msg.isMarker) {
                this.markerMap.delete(msgID);
            }
            this.messages.delete(msgID);
            //this.removeVertex(msgID);
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

    drawVertex = (msg: tangleVertex) => {
        this.newVertexCounter++;

        let v = this.cy.add({
            group: 'nodes',
            data: { id: msg.ID },
        });

        msg.strongParentIDs.forEach((spID) => {
            let sp = this.messages.get(spID);
            if (sp) {
                let edgeID = msg.ID+spID; 
                this.cy.add({
                    group: 'edges',
                    data: { id: edgeID, source: msg.ID, target: spID}
                });
            }            
        });

        msg.weakParentIDs.forEach((wpID) => {
            let wp = this.messages.get(wpID);
            if (wp) {
                let edgeID = msg.ID+wpID; 
                this.cy.add({
                    group: 'edges',
                    data: { id: edgeID, source: msg.ID, target: wpID}
                });
            }            
        });
        this.layoutApi.placeNewNodes(v);

        if (this.newVertexCounter >= 0) {
            this.cy.layout(fcoseOptions).run();
            this.newVertexCounter = 0;
        }
    }

    removeVertex = (msgID: string) => {
        let uiID = '#'+msgID;
        this.cy.remove(uiID);
        this.cy.layout( fcoseOptions ).run();
    }

    start = () => {
        this.cy = cytoscape({
            container: document.getElementById("tangleVisualizer"), // container to render in
            style: [ // the stylesheet for the graph
                {
                  selector: 'node',
                  style: {
                    'background-color': '#2E8BC0',
                    'shape': 'round-rectangle',
                    'width': 15,
                    'height': 15,
                  }
                },            
                {
                  selector: 'edge',
                  style: {
                    'width': 1,
                    'curve-style': 'bezier',
                    'line-color': '#696969',
                    'control-point-step-size': '10px'
                  }
                }
              ],
            layout: {
                name: 'fcose',
            },
        });
        this.layoutApi = this.cy.layoutUtilities(
            {
              desiredAspectRatio: 1,
              polyominoGridSizeFactor: 1,
              utilityFunction: 0,
              componentSpacing: 80,
            }
        );
    }
}

export default TangleStore;