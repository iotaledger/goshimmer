import {action, observable, ObservableMap} from 'mobx';
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

const vertexSize = 20;

export class VisualizerStore {
    @observable vertices = new ObservableMap<string, Vertex>();
    @observable verticesLimit = 1500;
    @observable solid_count = 0;
    @observable tips_count = 0;
    verticesIncomingOrder = [];
    collect: boolean = false;
    routerStore: RouterStore;

    // the currently selected vertex via hover
    @observable selected: Vertex;
    @observable selected_approvers_count = 0;
    @observable selected_approvees_count = 0;
    selected_via_click: boolean = false;
    selected_origin_color: number = 0;

    // search
    @observable search: string = "";

    // viva graph objs
    graph;
    graphics;
    renderer;
    @observable paused: boolean = false;

    constructor(routerStore: RouterStore) {
        this.routerStore = routerStore;
        registerHandler(WSMsgType.Vertex, this.addVertex);
        registerHandler(WSMsgType.TipInfo, this.addTipInfo);
    }

    @action
    updateSearch = (search: string) => {
        this.search = search.trim();
    }

    @action
    searchAndHighlight = () => {
        this.clearSelected();
        if (!this.search) return;
        let iter: IterableIterator<string> = this.vertices.keys();
        let found = null;
        for (const key of iter) {
            if (key.indexOf(this.search) >= 0) {
                found = key;
                break;
            }
        }
        if (!found) return;
        this.updateSelected(this.vertices.get(found), false);
    }

    @action
    pauseResume = () => {
        if (this.paused) {
            this.renderer.resume();
            this.paused = false;
            return;
        }
        this.renderer.pause();
        this.paused = true;
    }

    @action
    updateVerticesLimit = (num: number) => {
        this.verticesLimit = num;
    }

    @action
    addVertex = (vert: Vertex) => {
        if (!this.collect) return;

        let existing = this.vertices.get(vert.id);
        if (existing) {
            // can only go from unsolid to solid
            if (!existing.is_solid && vert.is_solid) {
                existing.is_solid = true;
                this.solid_count++;
            }
            // update trunk and branch ids since we might be dealing
            // with a vertex obj only created from a tip info
            existing.trunk_id = vert.trunk_id;
            existing.branch_id = vert.branch_id;
            vert = existing
        } else {
            if (vert.is_solid) {
                this.solid_count++;
            }
            this.verticesIncomingOrder.push(vert.id);
            this.checkLimit();
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
            this.verticesIncomingOrder.push(vert.id);
            this.checkLimit();
        }
        this.tips_count += tipInfo.is_tip ? 1 : vert.is_tip ? -1 : 0;
        vert.is_tip = tipInfo.is_tip;
        this.vertices.set(vert.id, vert);
        this.drawVertex(vert);
    };

    @action
    checkLimit = () => {
        while (this.verticesIncomingOrder.length > this.verticesLimit) {
            let deleteId = this.verticesIncomingOrder.shift();
            let vert = this.vertices.get(deleteId);
            // make sure we remove any markings if the vertex gets deleted
            if (this.selected && deleteId === this.selected.id) {
                this.clearSelected();
            }
            this.vertices.delete(deleteId);
            this.graph.removeNode(deleteId);
            if (!vert) {
                continue;
            }
            if (vert.is_solid) {
                this.solid_count--;
            }
            if (vert.is_tip) {
                this.tips_count--;
            }
            this.deleteApproveeLink(vert.trunk_id);
            this.deleteApproveeLink(vert.branch_id);
        }
    }

    @action
    deleteApproveeLink = (approveeId: string) => {
        if (!approveeId) {
            return;
        }
        let approvee = this.vertices.get(approveeId);
        if (approvee) {
            if (this.selected && approveeId === this.selected.id) {
                this.clearSelected();
            }
            if (approvee.is_solid) {
                this.solid_count--;
            }
            if (approvee.is_tip) {
                this.tips_count--;
            }
            this.vertices.delete(approveeId);
        }
        this.graph.removeNode(approveeId);
    }

    drawVertex = (vert: Vertex) => {
        let node;
        let existing = this.graph.getNode(vert.id);
        if (existing) {
            // update coloring
            let nodeUI = this.graphics.getNodeUI(vert.id);
            nodeUI.color = parseColor(this.colorForVertexState(vert));
            node = existing
        } else {
            node = this.graph.addNode(vert.id, vert);
        }
        if (vert.trunk_id && (!node.links || !node.links.some(link => link.fromId === vert.trunk_id))) {
            this.graph.addLink(vert.trunk_id, vert.id);
        }
        if (vert.trunk_id === vert.branch_id) {
            return;
        }
        if (vert.branch_id && (!node.links || !node.links.some(link => link.fromId === vert.branch_id))) {
            this.graph.addLink(vert.branch_id, vert.id);
        }
    }

    colorForVertexState = (vert: Vertex) => {
        if (!vert || (!vert.trunk_id && !vert.branch_id)) return "#b58900";
        if (vert.is_tip) {
            return "#cb4b16";
        }
        if (vert.is_solid) {
            return "#6c71c4";
        }
        return "#2aa198";
    }

    start = () => {
        this.collect = true;
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
            if (!node.data) {
                return Viva.Graph.View.webglSquare(10, this.colorForVertexState(node.data));
            }
            return Viva.Graph.View.webglSquare(vertexSize, this.colorForVertexState(node.data));
        })
        graphics.link(() => Viva.Graph.View.webglLine("#586e75"));
        let ele = document.getElementById('visualizer');
        this.renderer = Viva.Graph.View.renderer(this.graph, {
            container: ele, graphics, layout,
        });

        let events = Viva.Graph.webglInputEvents(graphics, this.graph);

        events.mouseEnter((node) => {
            this.clearSelected();
            this.updateSelected(node.data);
        }).mouseLeave((node) => {
            this.clearSelected();
        });
        this.graphics = graphics;
        this.renderer.run();
    }

    stop = () => {
        this.collect = false;
        this.renderer.dispose();
        this.graph = null;
        this.paused = false;
        this.selected = null;
        this.solid_count = 0;
        this.tips_count = 0;
        this.vertices.clear();
    }

    @action
    updateSelected = (vert: Vertex, viaClick?: boolean) => {
        if (!vert) return;

        this.selected = vert;
        this.selected_via_click = !!viaClick;

        // mutate links
        let node = this.graph.getNode(vert.id);
        let nodeUI = this.graphics.getNodeUI(vert.id);
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
        this.graph.forEachLink(function (link) {
            const linkUI = this.graphics.getLinkUI(link.id);
            linkUI.color = parseColor("#586e75");
        });
    }

    @action
    clearSelected = () => {
        this.selected_approvers_count = 0;
        this.selected_approvees_count = 0;
        if (this.selected_via_click || !this.selected) {
            return;
        }

        // clear link highlight
        let node = this.graph.getNode(this.selected.id);
        if (!node) {
            // clear links
            this.resetLinks();
            return;
        }

        let nodeUI = this.graphics.getNodeUI(this.selected.id);
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

        this.selected = null;
    }

}

export default VisualizerStore;

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