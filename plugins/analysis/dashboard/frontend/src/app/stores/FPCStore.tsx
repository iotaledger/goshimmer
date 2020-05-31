import {RouterStore} from "mobx-react-router";
import {action, computed, observable, ObservableMap} from "mobx";
import Col from "react-bootstrap/Col";
import * as React from "react";
import {registerHandler, WSMsgType} from "app/misc/WS";

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
    if (opinion == 0) {
        return "#BD0000"
    }
    return "#00BD00"
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

export class FPCStore {
    routerStore: RouterStore;

    @observable msg: FPCMessage = null;

    @observable n: number = 0;

    @observable nodes = new ObservableMap<number, Node>();

    constructor(routerStore: RouterStore) {
        this.routerStore = routerStore;

        setInterval(this.addNewNode, 100);
        setInterval(this.updateNodeValue, 400);

        registerHandler(WSMsgType.FPC, this.addLiveFeed);
    }

    @action
    addLiveFeed = (msg: FPCMessage) => {
        console.log(msg)
    }

    @action
    addNewNode = () => {
        const id = Math.floor(Math.random() * 1000);
        let node = new Node();
        node.id = id;
        node.opinion = Math.floor(Math.random() * 100)%2;
        this.nodes.set(id, node);
    }

    @action
    updateNodeValue = () => {
        let iter: IterableIterator<number> = this.nodes.keys();
        for (const key of iter) {
            let node = this.nodes.get(key);
            node.opinion = Math.floor(Math.random() * 100)%2;
            this.nodes.set(key, node);
        }
    }

    @computed
    get nodeGrid(){
        let nodeSquares = [];
        this.nodes.forEach((node: Node, id: number, obj: Map<number, Node>) => {
            nodeSquares.push(
                <Col xs={1} key={id.toString()} style={{
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


}

export default FPCStore;