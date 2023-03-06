import { action, observable, ObservableMap } from 'mobx';
import { registerHandler, WSMsgType } from "app/misc/WS";
import { RouterStore } from "mobx-react-router";
import { default as Viva } from 'vivagraphjs';
import {Block} from './ExplorerStore';

export class Vertex {
    id: string;
    parentIDsByType: Map<String, Array<string>>;
    is_tip: boolean;
    is_finalized: boolean;
    is_tx: boolean;
}

export class TipInfo {
    id: string;
    is_tip: boolean;
}

class history {
    vertices: Array<Vertex>;
}

const COLOR = {
    BlockPending: "#b9b7bd",
    BlockConfirmed: "#6c71c4",
    TransactionPending: "#393e46",
    TransactionConfirmed: "#fad02c",
    Tip: "#cb4b16",
    Unknown: "#b58900",
    Line: "#586e75",
    SelectedPastConeLine: "#e105f5",
    SelectedFutureConeLine: "#51e05d",
    Selected: "#859900"
}

const vertexSize = 20;

export class VisualizerStore {
    @observable vertices = new ObservableMap<string, Vertex>();
    @observable verticesLimit = 1500;
    @observable finalized_count = 0;
    @observable tips_count = 0;
    verticesIncomingOrder = [];
    draw: boolean = false;
    routerStore: RouterStore;

    // the currently selected vertex via hover
    @observable selected: Vertex;
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
        this.fetchHistory();
    }

    fetchHistory = async () => {
        try {
            let res = await fetch(`/api/visualizer/history`);
            let history: history = await res.json();
            history.vertices.forEach(v => {
                this.addVertex(v);
            });
        } catch (err) {
            console.log("Fail to fetch history in visualizer", err);
        }
        return
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
        let existing = this.vertices.get(vert.id);
        if (existing) {
            if (!existing.is_finalized && vert.is_finalized) {
                this.finalized_count++;
            }
        } else {
            if (vert.is_finalized) {
                this.finalized_count++;
            }
            this.verticesIncomingOrder.push(vert.id);
            this.checkLimit();
        }
        this.vertices.set(vert.id, vert);

        if (this.draw) {
            this.drawVertex(vert);
        }
    };

    @action
    addTipInfo = async (tipInfo: TipInfo) => {
        let v = this.vertices.get(tipInfo.id);
        if (!v) {
            v = new Vertex();
            v.id = tipInfo.id;

            // first seen as tip, get parents info
            let res = await fetch(`/api/block/${tipInfo.id}`);
            if (res.status === 200) {
                let blk: Block = await res.json();
                v.parentIDsByType = blk.parentsByType;
                v.is_finalized = blk.acceptance;
            }
            this.verticesIncomingOrder.push(v.id);
        }

        this.tips_count += tipInfo.is_tip ? 1 : v.is_tip ? -1 : 0;
        v.is_tip = tipInfo.is_tip;
        this.vertices.set(tipInfo.id, v);

        if (this.draw) {
            this.drawVertex(v);
        }
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
            if (this.draw) {
                this.graph.removeNode(deleteId);
            }
            if (!vert) {
                continue;
            }
            if (vert.is_finalized) {
                this.finalized_count--;
            }
            if (vert.is_tip) {
                this.tips_count--;
            }
            this.vertices.delete(deleteId);
        }
    }

    drawVertex = (vert: Vertex) => {
        let node = this.graph.getNode(vert.id);
        if (node) {
            // update coloring
            let nodeUI = this.graphics.getNodeUI(vert.id);
            nodeUI.color = parseColor(this.colorForVertexState(vert));
        } else {
            node = this.graph.addNode(vert.id, vert);
        }

        if (vert.parentIDsByType) {
            Object.keys(vert.parentIDsByType).map((parentType) => {
                vert.parentIDsByType[parentType].forEach((value) => {
                    // if value is valid AND (links is empty OR there is no between parent and children)
                    if (value && ((!node.links || !node.links.some(link => link.fromId === value)))) {
                        // draw the link only when the parent exists
                        let parent = this.graph.getNode(value);
                        if (parent) {
                            this.graph.addLink(value, vert.id);
                        } else {
                            console.log("link not added, parent doesn't exist", value);
                        }
                    }
                })
            })
        }
    }

    colorForVertexState = (vert: Vertex) => {
        if (!vert) {
            return COLOR.Unknown;
        }

        // finalized
        if (vert.is_finalized) {
            if (vert.is_tx) {
                return COLOR.TransactionConfirmed;
            }
            return COLOR.BlockConfirmed;
        }

        if (vert.is_tip) {
            return COLOR.Tip;
        }

        // pending
        if (vert.is_tx) {
            return COLOR.TransactionPending
        }
        return COLOR.BlockPending;
    }

    start = () => {
        this.draw = true;
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
        graphics.link(() => Viva.Graph.View.webglLine(COLOR.Line));
        let ele = document.getElementById('visualizer');
        this.renderer = Viva.Graph.View.renderer(this.graph, {
            container: ele, graphics, layout,
        });

        let events = Viva.Graph.webglInputEvents(graphics, this.graph);

        events.mouseEnter((node) => {
            this.clearSelected(true);
            this.updateSelected(node.data);
        }).mouseLeave((node) => {
            this.clearSelected(false);
        });

        events.click((node) => {
            this.clearSelected(true);
            this.updateSelected(node.data, true);
        });

        this.graphics = graphics;
        this.renderer.run();

        // draw vertices by order
        this.verticesIncomingOrder.forEach((id) => {
            let v = this.vertices.get(id);
            if (v) {
                this.drawVertex(v);
            }
        })
    }

    stop = () => {
        this.draw = false;
        this.renderer.dispose();
        this.graph = null;
        this.paused = false;
        this.selected = null;
    }

    @action
    updateSelected = (vert: Vertex, viaClick?: boolean) => {
        if (!vert) return;

        this.selected = vert;
        this.selected_via_click = !!viaClick;

        // mutate links
        let nodeUI = this.graphics.getNodeUI(vert.id);
        this.selected_origin_color = nodeUI.color
        nodeUI.color = parseColor(COLOR.Selected);
        nodeUI.size = vertexSize * 1.5;

        let node = this.graph.getNode(vert.id);
        const seenForward = [];
        const seenBackwards = [];
        dfsIterator(this.graph, node, node => {
        }, true,
            link => {
                const linkUI = this.graphics.getLinkUI(link.id);
                linkUI.color = parseColor(COLOR.SelectedFutureConeLine);
            },
            seenBackwards
        );
        dfsIterator(this.graph, node, node => {
        }, false,
            link => {
                const linkUI = this.graphics.getLinkUI(link.id);
                linkUI.color = parseColor(COLOR.SelectedPastConeLine);
            },
            seenForward
        );
    }

    resetLinks = () => {
        this.graph.forEachLink(function (link) {
            const linkUI = this.graphics.getLinkUI(link.id);
            linkUI.color = parseColor(COLOR.Line);
        });
    }

    @action
    clearSelected = (force_clear?: boolean) => {
        if (!this.selected || (this.selected_via_click && !force_clear)) {
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
                linkUI.color = parseColor(COLOR.Line);
            },
            seenBackwards
        );
        dfsIterator(this.graph, node, node => {
        }, false,
            link => {
                const linkUI = this.graphics.getLinkUI(link.id);
                linkUI.color = parseColor(COLOR.Line);
            },
            seenForward
        );

        this.selected = null;
        this.selected_via_click = false;
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

        if (!node.links) {
            return
        }

        for (const link of node.links) {
            // parents
            if (!up && link.toId === node.id && !seenNodes.includes(graph.getNode(link.fromId))) {
                if (cbLinks) cbLinks(link);
                seenNodes.push(graph.getNode(link.fromId));
                continue;
            }

            // children
            if (up && link.fromId === node.id && !seenNodes.includes(graph.getNode(link.toId))) {
                if (cbLinks) cbLinks(link);
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
