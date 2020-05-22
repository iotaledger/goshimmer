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
                //console.log(voteContext.nodeid, voteContext.rounds, voteContext.opinions, voteContext.like);

                this.addNewNode(key, voteContext.nodeid, voteContext.opinions[voteContext.opinions.length-1])
                this.updateNodeValue(key, voteContext.nodeid, voteContext.opinions[voteContext.opinions.length-1])
            }
        }
    }

    @action
    addNewNode = (conflictID, nodeID, opinion) => {
        let node = new Node();
        node.id = nodeID;
        node.opinion = opinion;

        if (!this.conflicts.has(conflictID)) {
            let cd = new ConflictDetail();
            cd.nodes = new ObservableMap<string, Node>();
            this.conflicts.set(conflictID, cd);
        }

        let c = this.conflicts.get(conflictID)
        
        c.nodes.set(nodeID, node);
        this.conflicts.set(conflictID, c);
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
        c.nodes.forEach((node: Node, id: string, obj: Map<string, Node>) => {
            nodeSquares.push(
                <Col xs={1} key={id} style={{
                    height: 50,
                    width: 50,
                    border: "1px solid #FFFFFF",
                    textAlign: "center",
                    verticalAlign: "middle",
                    background: SetColor(node.opinion),
                }}>
                    {id.substr(0, 5)}
                </Col>
            )
        })
        return nodeSquares;
    }

    @computed
    get conflictGrid(){
        let conflictSquares = [];
        this.conflicts.forEach((conflict: ConflictDetail, id: string, obj: Map<string, ConflictDetail>) => {
            
            let likes = 0;
            let total = conflict.nodes.size;

            conflict.nodes.forEach((node: Node, id: string, obj: Map<string, Node>) => {
                if (node.opinion == Opinion.Like) likes++;
            })
 

            conflictSquares.push(
                <Col xs={1} key={id} style={{
                    height: 50,
                    width: 50,
                    border: "1px solid #FFFFFF",
                    textAlign: "center",
                    verticalAlign: "middle",
                    background: getColorShade(likes/total),
                }}>
                    {<Link to={`/fpc-example/conflict/${id}`}>
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
    if (eta > 0.1) return "#D40000";
    return "#800000";
}