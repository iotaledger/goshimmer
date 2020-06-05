import {RouterStore} from "mobx-react-router";
import {action, computed, observable} from "mobx";
import Col from "react-bootstrap/Col";
import * as React from "react";
import {registerHandler, WSMsgType} from "app/misc/WS";
import {Link} from 'react-router-dom';

function SetColor(opinion) {
    if (opinion == Opinion.Dislike) {
        return "#800000"
    }
    return "#00AB08"
}

enum Opinion {
    Like = 1,
    Dislike
}

class VoteContext {
    nodeid: string;
    rounds: number;
    opinions: number[];
    status: number;
}

class Conflict {
    nodesview: Map<string, VoteContext>
}

export class FPCMessage {
    conflictset: Map<string, Conflict>
}

class Conflicts {

}

export class FPCStore {
    routerStore: RouterStore;

    @observable msg: FPCMessage = null;

    @observable conflicts = new Conflicts();

    @observable currentConflict = "";

    constructor(routerStore: RouterStore) {
        this.routerStore = routerStore;
        registerHandler(WSMsgType.FPC, this.addLiveFeed);
    }

    @action
    updateCurrentConflict = (id: string) => {
        this.currentConflict = id;
    }

    @action
    addLiveFeed = (msg: FPCMessage) => {
        let conflictIDs = Object.keys(msg.conflictset);
        if (!conflictIDs) return;
        console.log("FPC message has conflicts");
        for (const conflictID of conflictIDs) {
            let nodeIDs = Object.keys(msg.conflictset[conflictID].nodesview);
            console.log("nodes for conflict", conflictID, nodeIDs);
            for (const nodeID of nodeIDs) {
                let voteContext = msg.conflictset[conflictID].nodesview[nodeID];
                let latestOpinion = voteContext.opinions[voteContext.opinions.length - 1];
                console.log("updating node opinion");
                this.updateNodeValue(conflictID, nodeID, latestOpinion);
            }
        }
    }

    @action
    updateNodeValue = (conflictID: string, nodeID: string, latestOpinion: number) => {
        let conflicts = Object.assign({}, this.conflicts, {});
        if (!conflicts.hasOwnProperty(conflictID)) {
            conflicts[conflictID] = {[nodeID]: latestOpinion}
            this.conflicts = conflicts;
            console.log(this.conflicts);
            return;
        }

        conflicts[conflictID][nodeID] = latestOpinion;
        this.conflicts = conflicts;
        console.log(this.conflicts);
        return;
    }

    @computed
    get nodeGrid() {
        if (!this.currentConflict) return;
        let nodeSquares = [];
        let currentConflict = this.conflicts[this.currentConflict];
        let nodeIDs = Object.keys(currentConflict);
        nodeIDs.forEach((nodeID: string) => {
            nodeSquares.push(
                <Col xs={1} key={nodeID} style={{
                    height: 50,
                    width: 50,
                    border: "1px solid #FFFFFF",
                    textAlign: "center",
                    verticalAlign: "middle",
                    background: SetColor(currentConflict[nodeID]),
                }}>
                    {nodeID.substr(0, 5)}
                </Col>
            )
        })
        return nodeSquares;
    }

    @computed
    get conflictGrid() {
        let conflictSquares = [];
        let conflictIDs = Object.keys(this.conflicts);
        conflictIDs.forEach((conflictID: string) => {

            let likes = 0;
            let nodeIDs = Object.keys(this.conflicts[conflictID]);
            let total = nodeIDs.length;

            nodeIDs.forEach((nodeID) => {
                if (this.conflicts[conflictID][nodeID] === Opinion.Like) likes++;
            })


            conflictSquares.push(
                <Col xs={1} key={conflictID} style={{
                    height: 50,
                    width: 50,
                    border: "1px solid #FFFFFF",
                    textAlign: "center",
                    verticalAlign: "middle",
                    background: getColorShade(likes / total),
                }}>
                    {<Link to={`/fpc-example/conflict/${conflictID}`} style={{color: "white"}}>
                        {`[${likes}/${total}]`}
                    </Link>}
                </Col>
            )
        })
        return conflictSquares;
    }

}

export default FPCStore;

function getColorShade(eta) {
    if (eta == 1) return "#00AB08";
    if (eta > 0.9) return "#00C301";
    if (eta > 0.8) return "#26D701";
    if (eta > 0.7) return "#4DED30";
    if (eta > 0.6) return "#95F985";
    if (eta > 0.5) return "#B7FFBF";
    if (eta > 0.4) return "#FFD5D5";
    if (eta > 0.3) return "#FF8080";
    if (eta > 0.2) return "#FF2A2A";
    if (eta > 0.0001) return "#D40000";
    return "#800000";
}
