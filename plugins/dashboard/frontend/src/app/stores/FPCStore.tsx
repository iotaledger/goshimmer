import {RouterStore} from "mobx-react-router";
import {action, observable, ObservableMap} from "mobx";

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

export class FPCStore {
    routerStore: RouterStore;

    @observable n: number = 0;

    @observable nodes = new ObservableMap<number, Node>();

    constructor(routerStore: RouterStore) {
        this.routerStore = routerStore;

        setInterval(this.addNewNode, 100);
        setInterval(this.updateNodeValue, 400);
    }

    @action
    addNewNode = () => {
        const id = Math.floor(Math.random() * 1000);
        let node = new Node();
        node.id = id;
        node.opinion = Math.floor(Math.random() * 100);
        this.nodes.set(id, node);
    }

    @action
    updateNodeValue = () => {
        let iter: IterableIterator<number> = this.nodes.keys();
        for (const key of iter) {
            let node = this.nodes.get(key);
            node.opinion = Math.floor(Math.random() * 100);
            this.nodes.set(key, node);
        }
    }


}

export default FPCStore;