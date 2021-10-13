import { action, observable, ObservableMap } from 'mobx';
import {connectWebSocket, registerHandler, unregisterHandler, WSMsgType} from 'WS';
import cytoscape from 'cytoscape';

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
    @observable maxTangleVertices: number = 500;
    @observable messages = new ObservableMap<string, tangleVertex>();
    @observable markerMap = new ObservableMap<string, Array<string>>();
    @observable awMap = new ObservableMap<string, number>();
    msgOrder: Array<any> = [];

    constructor() {
        this.connect()
        
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

    start = () => {
        var cy = cytoscape({
            container: document.getElementById("tangleVisualizer"), // container to render in
            elements: [ // list of graph elements to start with
                { // node a
                  data: { id: 'a' }
                },
                { // node b
                  data: { id: 'b' }
                },
                { // edge ab
                  data: { id: 'ab', source: 'a', target: 'b' }
                }
              ],
            style: [ // the stylesheet for the graph
                {
                  selector: 'node',
                  style: {
                    'background-color': '#666',
                    'label': 'data(id)',
                    'width': 5,
                    'height': 5,
                  }
                },            
                {
                  selector: 'edge',
                  style: {
                    'width': 3,
                    'line-color': '#ccc',
                    'target-arrow-color': '#ccc',
                    'target-arrow-shape': 'triangle',
                    'curve-style': 'bezier'
                  }
                }
              ],            
              layout: {
                name: 'grid',
                rows: 1
              }
        });        
    }
}

export default TangleStore;