import {RouterStore} from "mobx-react-router";
import {action, computed, observable, ObservableMap} from "mobx";
import Col from "react-bootstrap/Col";
import * as React from "react";
import {registerHandler, WSMsgType} from "app/misc/WS";
import {Link} from 'react-router-dom';

export class Node {
    id: number;
    opinion: number = 0;
}

export function LightenDarkenColor(col, amt) {
    var num = parseInt(col, 16);
    var r = (num >> 16) + amt;
    var b = ((num >> 8) & 0x00FF) + amt;
    var g = (num & 0x0000FF) + amt;
    var newColor = g | (b << 8) | (r << 16);
    return newColor.toString(16);
}

function SetColor(opinion) {
    if (opinion == Opinion.Dislike) {
        return "#BD0000"
    }
    return "#00BD00"
}

enum Opinion {
    Like = 1,
    Dislike
}

class VoteContext {
    nodeid: string;
    rounds: number;
    opinions: number[];
    like: number;
}
class Conflict {
    nodesview: Map<string, VoteContext>
}
export class FPCMessage {
    nodes: number;
    conflictset: Map<string, Conflict>
}

class ConflictDetail {
    @observable like: number;
    @observable dislike: number;
    @observable nodes: ObservableMap<string, Node>
}

export class FPCStore {
    routerStore: RouterStore;

    @observable msg: FPCMessage = null;

    @observable conflicts = new ObservableMap<string, ConflictDetail>();

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
        for (const key of Object.keys(msg.conflictset)) {
            for (const key2 of Object.keys(msg.conflictset[key].nodesview)){
                let voteContext = msg.conflictset[key].nodesview[key2];
                console.log(voteContext.nodeid, voteContext.rounds, voteContext.opinions, voteContext.like);

                this.addNewNode(key, voteContext.nodeid, voteContext.opinions[voteContext.opinions.length-1])
                this.updateNodeValue(key, voteContext.nodeid, voteContext.opinions[voteContext.opinions.length-1])
            }
            console.log("for loop", this.conflicts.get(key).nodes);
        }
    }

    @action
    addNewNode = (conflictID, nodeID, opinion) => {
        let node = new Node();
        node.id = nodeID;
        node.opinion = opinion;

        console.log("has", this.conflicts.has(conflictID))

        if (!this.conflicts.has(conflictID)) {
            let cd = new ConflictDetail();
            cd.nodes = new ObservableMap<string, Node>();
            this.conflicts.set(conflictID, cd);
        }

        let c = this.conflicts.get(conflictID)

        console.log("c nodes", c.nodes);
        
        c.nodes.set(nodeID, node);
        this.conflicts.set(conflictID, c);

        console.log("conflict nodes", this.conflicts.get(conflictID).nodes);
    }

    @action
    updateNodeValue = (conflictID, nodeID, opinion) => {
        let c = this.conflicts.get(conflictID);
        let node = c.nodes.get(nodeID);
        node.opinion = opinion;
        c.nodes.set(nodeID, node);
        this.conflicts.set(conflictID, c)
    }

    @computed
    get nodeGrid(){
        if (!this.currentConflict) return;
        let nodeSquares = [];
        let c = this.conflicts.get(this.currentConflict);

        console.log("current conflict", this.currentConflict, "current", c); 
        
        c.nodes.forEach((node: Node, id: string, obj: Map<string, Node>) => {
            nodeSquares.push(
                <Col xs={1} key={id} style={{
                    height: 50,
                    width: 50,
                    border: "1px solid #FFFFFF",
                    background: SetColor(node.opinion),
                }}>
                    {/* {node.opinion} */}
                </Col>
            )
        })
        return nodeSquares;
    }

    @computed
    get conflictGrid(){
        let conflictSquares = [];
        this.conflicts.forEach((conflict: ConflictDetail, id: string, obj: Map<string, ConflictDetail>) => {
            //getEta(conflict)
            conflictSquares.push(
                <Col xs={1} key={id} style={{
                    height: 50,
                    width: 50,
                    border: "1px solid #FFFFFF",
                    background: SetColor(conflict.like),
                }}>
                    {<Link to={`/fpc-example/conflict/${id}`}>
                        {id.substr(0, 5)}
                    </Link>}
                </Col>
            )
        })
        return conflictSquares;
    }

}

export default FPCStore;