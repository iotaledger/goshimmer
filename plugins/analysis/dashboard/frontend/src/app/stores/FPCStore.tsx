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
        return "#BD0000"
    }
    return "#00BD00"
}

function getEta(conflict) {
    let total = conflict.nodes.length;

    console.log("Total:", total)
    
    if (total == 0) return;
    
    let likes = 0;
    conflict.nodes.forEach((node: Node, id: string, obj: Map<string, Node>) => {
        if (node.opinion == Opinion.Like) likes++;
    })
    
    console.log("Likes:", likes/total)
    
    return likes/total;
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
            conflictSquares.push(
                <Col xs={1} key={id} style={{
                    height: 50,
                    width: 50,
                    border: "1px solid #FFFFFF",
                    background: RGB_Log_Blend(getEta(conflict),"#BD0000", "#00BD00"),
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

const RGB_Log_Blend=(p,c0,c1)=>{
    var i=parseInt,r=Math.round,P=1-p,[a,b,c,d]=c0.split(","),[e,f,g,h]=c1.split(","),x=d||h;
    //j=x?","+(!d?h:!h?d:r((parseFloat(d)*P+parseFloat(h)*p)*1000)/1000+")"):")";
	return"rgb"+(x?"a(":"(")+r((P*i(a[3]=="a"?a.slice(5):a.slice(4))**2+p*i(e[3]=="a"?e.slice(5):e.slice(4))**2)**0.5)+","+r((P*i(b)**2+p*i(f)**2)**0.5)+","+r((P*i(c)**2+p*i(g)**2)**0.5)+d;
}
