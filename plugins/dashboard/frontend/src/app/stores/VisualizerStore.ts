import {action, computed, observable, ObservableMap} from 'mobx';
import {registerHandler, WSMsgType} from "app/misc/WS";
import {RouterStore} from "mobx-react-router";
import {default as Viva} from 'vivagraphjs';

export class Vertex {
    id: string;
    trunk_id: string;
    branch_id: string;
    is_solid: boolean;
    is_tip: boolean;
}

export class TipInfo {
    id: string;
    is_tip: boolean;
}

export class VisualizerStore {
    @observable vertices = new ObservableMap<string, Vertex>();
    @observable verticesLimit = 1500;
    verticesIncomingOrder = [];
    collect: boolean = false;
    routerStore: RouterStore;

    // the currently selected vertex via hover
    @observable selected: Vertex;

    // viva graph objs
    graph;

    constructor(routerStore: RouterStore) {
        this.routerStore = routerStore;
        registerHandler(WSMsgType.Vertex, this.addVertex);
        registerHandler(WSMsgType.TipInfo, this.addTipInfo);
    }

    @action
    addVertex = (vert: Vertex) => {
        if (!this.collect) return;

        let existing = this.vertices.get(vert.id);
        if (existing) {
            // can only go from unsolid to solid
            if (!existing.is_solid && vert.is_solid) {
                existing.is_solid = true;
            }
            // update trunk and branch ids since we might be dealing
            // with a vertex obj only created from a tip info
            existing.trunk_id = vert.trunk_id;
            existing.branch_id = vert.branch_id;
            vert = existing
        } else {
            // new vertex
            this.verticesIncomingOrder.push(vert.id);
            while (this.verticesIncomingOrder.length > this.verticesLimit) {
                let deleteId = this.verticesIncomingOrder.shift();
                this.vertices.delete(deleteId);
                this.graph.removeNode(deleteId);
            }
        }

        this.vertices.set(vert.id, vert);
        this.drawVertex(vert);
    };

    @action
    addTipInfo = (tipInfo: TipInfo) => {
        if (!this.collect) return;
        let vert = this.vertices.get(tipInfo.id);
        if (!vert) {
            // create a new empty one for now
            vert = new Vertex();
            vert.id = tipInfo.id;
        }
        if (vert.is_tip) {
            vert.is_tip = tipInfo.is_tip;
        }
        this.vertices.set(vert.id, vert);
        this.drawVertex(vert);
    };

    drawVertex = (vert: Vertex) => {
        this.graph.addNode(vert.id, vert);
        if (vert.trunk_id) {
            this.graph.addLink(vert.id, vert.trunk_id);
        }
        if (vert.branch_id) {
            this.graph.addLink(vert.id, vert.branch_id);
        }
    }

    start = () => {
        this.collect = true;
        this.graph = Viva.Graph.graph();

        let graphics: any = Viva.Graph.View.webglGraphics();

        const layout = Viva.Graph.Layout.forceDirected(this.graph, {
            springLength: 10,
            springCoeff: 0.0001,
            gravity: -4,
            dragCoeff: 0.02,
            timeStep: 22,
            theta: 0.8,
        });

        graphics.node((node) => {
            if (!node.data) return Viva.Graph.View.webglSquare(5, "#dbdbdb");
            if (node.data.is_tip) {
                return Viva.Graph.View.webglSquare(20, "#db6073");
            }
            if (node.data.is_solid) {
                return Viva.Graph.View.webglSquare(20, "#5dbbc9");
            }
            return Viva.Graph.View.webglSquare(20, "#565656");
        })
        graphics.link(() => Viva.Graph.View.webglLine("#424242"));
        let ele = document.getElementById('visualizer');
        let renderer = Viva.Graph.View.renderer(this.graph, {
            container: ele, graphics, layout,
        });

        let events = Viva.Graph.webglInputEvents(graphics, this.graph);

        events.mouseEnter((node) => {
            this.updateSelected(node.data);
        }).mouseLeave((node) => {
            this.clearSelected();
        }).click((node) => {
            this.updateSelected(node.data);
        });
        renderer.run();
    }

    stop = () => {
        this.collect = false;
        this.graph = null;
        this.vertices.clear();
    }

    @action
    updateSelected = (vert: Vertex) => {
        this.selected = vert;
    }

    @action
    clearSelected = () => {
        this.selected = null;
    }

    @computed
    get solidCount() {
        let count = 0;
        this.vertices.forEach((vert: Vertex) => {
            if (!vert.is_solid) return;
            count++
        })
        return count
    }

}

export default VisualizerStore;